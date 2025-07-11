package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheNotFound 缓存未命中
var CacheNotFound = redis.Nil

// redisCache Redis缓存对象
type redisCache struct {
	client            *redis.Client
	KeyPrefix         string
	encoding          Encoding
	DefaultExpireTime time.Duration
	newObject         func() interface{}
}

// NewRedisCache 创建新的缓存，client参数可以传入用于单元测试
func NewRedisCache(client *redis.Client, keyPrefix string, encode Encoding, newObject func() interface{}) Cache {
	return &redisCache{
		client:    client,
		KeyPrefix: keyPrefix,
		encoding:  encode,
		newObject: newObject,
	}
}

// Set 设置单个值
func (c *redisCache) Set(ctx context.Context, key string, val interface{}, expiration time.Duration) error {
	buf, err := Marshal(c.encoding, val)
	if err != nil {
		return fmt.Errorf("编码错误: %v, 键=%s, 值=%+v ", err, key, val)
	}

	cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
	if err != nil {
		return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
	}
	// 如果过期时间为0，使用默认过期时间
	// if expiration == 0 {
	//	expiration = DefaultExpireTime
	// }
	if len(buf) == 0 {
		buf = NotFoundPlaceholderBytes
	}
	err = c.client.Set(ctx, cacheKey, buf, expiration).Err()
	if err != nil {
		return fmt.Errorf("客户端设置错误: %v, 缓存键=%s", err, cacheKey)
	}
	return nil
}

// Get 获取单个值
func (c *redisCache) Get(ctx context.Context, key string, val interface{}) error {
	cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
	if err != nil {
		return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
	}

	dataBytes, err := c.client.Get(ctx, cacheKey).Bytes()
	// 注意：不处理redis值为nil的情况
	// 而是留给上游处理
	if err != nil {
		return err
	}

	// 防止数据为空时Unmarshal报错
	if len(dataBytes) == 0 || bytes.Equal(dataBytes, NotFoundPlaceholderBytes) {
		return ErrPlaceholder
	}
	err = Unmarshal(c.encoding, dataBytes, val)
	if err != nil {
		return fmt.Errorf("解码错误: %v, 键=%s, 缓存键=%s, 类型=%T, json=%s ",
			err, key, cacheKey, val, dataBytes)
	}
	return nil
}

// MultiSet 设置多个值
func (c *redisCache) MultiSet(ctx context.Context, valueMap map[string]interface{}, expiration time.Duration) error {
	if len(valueMap) == 0 {
		return nil
	}
	//if expiration == 0 {
	//	expiration = DefaultExpireTime
	//}

	// 键值对成对出现，容量是map的两倍
	paris := make([]interface{}, 0, 2*len(valueMap))
	for key, value := range valueMap {
		buf, err := Marshal(c.encoding, value)
		if err != nil {
			fmt.Printf("编码错误, %v, 值:%v\n", err, value)
			continue
		}
		cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
		if err != nil {
			fmt.Printf("构建缓存键错误, %v, 键:%v\n", err, key)
			continue
		}
		paris = append(paris, []byte(cacheKey))
		paris = append(paris, buf)
	}
	pipeline := c.client.Pipeline()
	err := pipeline.MSet(ctx, paris...).Err()
	if err != nil {
		return fmt.Errorf("管道批量设置错误: %v", err)
	}
	for i := 0; i < len(paris); i = i + 2 {
		switch paris[i].(type) {
		case []byte:
			pipeline.Expire(ctx, string(paris[i].([]byte)), expiration)
		default:
			fmt.Printf("redis过期不支持的键类型: %T\n", paris[i])
		}
	}
	_, err = pipeline.Exec(ctx)
	if err != nil {
		return fmt.Errorf("管道执行错误: %v", err)
	}
	return nil
}

// MultiGet 获取多个值
func (c *redisCache) MultiGet(ctx context.Context, keys []string, value interface{}) error {
	if len(keys) == 0 {
		return nil
	}
	cacheKeys := make([]string, len(keys))
	for index, key := range keys {
		cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
		if err != nil {
			return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
		}
		cacheKeys[index] = cacheKey
	}
	values, err := c.client.MGet(ctx, cacheKeys...).Result()
	if err != nil {
		return fmt.Errorf("客户端批量获取错误: %v, 键=%+v", err, cacheKeys)
	}

	// 通过反射注入到map中
	valueMap := reflect.ValueOf(value)
	for i, v := range values {
		if v == nil {
			continue
		}
		dataBytes := []byte(v.(string))
		if len(dataBytes) == 0 || bytes.Equal(dataBytes, NotFoundPlaceholderBytes) {
			continue
		}
		object := c.newObject()
		err = Unmarshal(c.encoding, dataBytes, object)
		if err != nil {
			fmt.Printf("反序列化数据错误: %+v, 缓存键=%s 值类型=%T\n", err, cacheKeys[i], value)
			continue
		}
		valueMap.SetMapIndex(reflect.ValueOf(cacheKeys[i]), reflect.ValueOf(object))
	}
	return nil
}

// Del 删除多个值
func (c *redisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	cacheKeys := make([]string, len(keys))
	for index, key := range keys {
		cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
		if err != nil {
			continue
		}
		cacheKeys[index] = cacheKey
	}
	err := c.client.Del(ctx, cacheKeys...).Err()
	if err != nil {
		return fmt.Errorf("客户端删除错误: %v, 键=%+v", err, cacheKeys)
	}
	return nil
}

// SetCacheWithNotFound 为未找到的情况设置值
func (c *redisCache) SetCacheWithNotFound(ctx context.Context, key string) error {
	cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
	if err != nil {
		return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
	}

	return c.client.Set(ctx, cacheKey, NotFoundPlaceholder, DefaultNotFoundExpireTime).Err()
}

// BuildCacheKey 使用前缀构造缓存键
func BuildCacheKey(keyPrefix string, key string) (string, error) {
	if key == "" {
		return "", errors.New("[缓存] 键不能为空")
	}

	cacheKey := key
	if keyPrefix != "" {
		cacheKey = strings.Join([]string{keyPrefix, key}, ":")
	}

	return cacheKey, nil
}

// -------------------------------------------------------------------------------------------

// redisClusterCache Redis集群缓存对象
type redisClusterCache struct {
	client            *redis.ClusterClient
	KeyPrefix         string
	encoding          Encoding
	DefaultExpireTime time.Duration
	newObject         func() interface{}
}

// NewRedisClusterCache 创建新的集群缓存
func NewRedisClusterCache(client *redis.ClusterClient, keyPrefix string, encode Encoding, newObject func() interface{}) Cache {
	return &redisClusterCache{
		client:    client,
		KeyPrefix: keyPrefix,
		encoding:  encode,
		newObject: newObject,
	}
}

// Set 设置单个值
func (c *redisClusterCache) Set(ctx context.Context, key string, val interface{}, expiration time.Duration) error {
	buf, err := Marshal(c.encoding, val)
	if err != nil {
		return fmt.Errorf("编码错误: %v, 键=%s, 值=%+v ", err, key, val)
	}

	cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
	if err != nil {
		return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
	}
	//if expiration == 0 {
	//	expiration = DefaultExpireTime
	//}
	if len(buf) == 0 {
		buf = NotFoundPlaceholderBytes
	}
	err = c.client.Set(ctx, cacheKey, buf, expiration).Err()
	if err != nil {
		return fmt.Errorf("客户端设置错误: %v, 缓存键=%s", err, cacheKey)
	}
	return nil
}

// Get 获取单个值
func (c *redisClusterCache) Get(ctx context.Context, key string, val interface{}) error {
	cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
	if err != nil {
		return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
	}

	dataBytes, err := c.client.Get(ctx, cacheKey).Bytes()
	// NOTE: don't handle the case where redis value is nil
	// 但留给上游处理
	if err != nil {
		return err
	}

	// 防止数据为空时Unmarshal报错
	if len(dataBytes) == 0 || bytes.Equal(dataBytes, NotFoundPlaceholderBytes) {
		return ErrPlaceholder
	}
	err = Unmarshal(c.encoding, dataBytes, val)
	if err != nil {
		return fmt.Errorf("解码错误: %v, 键=%s, 缓存键=%s, 类型=%T, json=%s ",
			err, key, cacheKey, val, dataBytes)
	}
	return nil
}

// MultiSet 设置多个值
func (c *redisClusterCache) MultiSet(ctx context.Context, valueMap map[string]interface{}, expiration time.Duration) error {
	if len(valueMap) == 0 {
		return nil
	}

	// 键值对成对出现，容量是map的两倍
	paris := make([]interface{}, 0, 2*len(valueMap))
	for key, value := range valueMap {
		buf, err := Marshal(c.encoding, value)
		if err != nil {
			fmt.Printf("编码错误, %v, 值:%v\n", err, value)
			continue
		}
		cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
		if err != nil {
			fmt.Printf("构建缓存键错误, %v, 键:%v\n", err, key)
			continue
		}
		paris = append(paris, []byte(cacheKey))
		paris = append(paris, buf)
	}
	pipeline := c.client.Pipeline()
	err := pipeline.MSet(ctx, paris...).Err()
	if err != nil {
		return fmt.Errorf("管道批量设置错误: %v", err)
	}
	for i := 0; i < len(paris); i = i + 2 {
		switch paris[i].(type) {
		case []byte:
			pipeline.Expire(ctx, string(paris[i].([]byte)), expiration)
		default:
			fmt.Printf("redis过期不支持的键类型: %T\n", paris[i])
		}
	}
	_, err = pipeline.Exec(ctx)
	if err != nil {
		return fmt.Errorf("管道执行错误: %v", err)
	}
	return nil
}

// MultiGet 获取多个值
func (c *redisClusterCache) MultiGet(ctx context.Context, keys []string, value interface{}) error {
	if len(keys) == 0 {
		return nil
	}
	cacheKeys := make([]string, len(keys))
	for index, key := range keys {
		cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
		if err != nil {
			return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
		}
		cacheKeys[index] = cacheKey
	}
	values, err := c.client.MGet(ctx, cacheKeys...).Result()
	if err != nil {
		return fmt.Errorf("客户端批量获取错误: %v, 键=%+v", err, cacheKeys)
	}

	// 通过反射注入到map中
	valueMap := reflect.ValueOf(value)
	for i, v := range values {
		if v == nil {
			continue
		}
		dataBytes := []byte(v.(string))
		if len(dataBytes) == 0 || bytes.Equal(dataBytes, NotFoundPlaceholderBytes) {
			continue
		}
		object := c.newObject()
		err = Unmarshal(c.encoding, dataBytes, object)
		if err != nil {
			fmt.Printf("反序列化数据错误: %+v, 缓存键=%s 类型=%T\n", err, cacheKeys[i], value)
			continue
		}
		valueMap.SetMapIndex(reflect.ValueOf(cacheKeys[i]), reflect.ValueOf(object))
	}
	return nil
}

// Del 删除多个值
func (c *redisClusterCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	cacheKeys := make([]string, len(keys))
	for index, key := range keys {
		cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
		if err != nil {
			continue
		}
		cacheKeys[index] = cacheKey
	}
	err := c.client.Del(ctx, cacheKeys...).Err()
	if err != nil {
		return fmt.Errorf("客户端删除错误: %v, 键=%+v", err, cacheKeys)
	}
	return nil
}

// SetCacheWithNotFound 为未找到的情况设置值
func (c *redisClusterCache) SetCacheWithNotFound(ctx context.Context, key string) error {
	cacheKey, err := BuildCacheKey(c.KeyPrefix, key)
	if err != nil {
		return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
	}

	return c.client.Set(ctx, cacheKey, NotFoundPlaceholder, DefaultNotFoundExpireTime).Err()
}
