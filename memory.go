package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto"
)

type options struct {
	numCounters int64
	maxCost     int64
	bufferItems int64
}

func defaultOptions() *options {
	return &options{
		numCounters: 1e7,     // 跟踪频率的键数量 (10M)
		maxCost:     1 << 30, // 缓存的最大成本 (1GB)
		bufferItems: 64,      // 每个Get缓冲区的键数量
	}
}

// Option 设置选项
type Option func(*options)

func (o *options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

// WithNumCounters 设置键的数量
func WithNumCounters(numCounters int64) Option {
	return func(o *options) {
		o.numCounters = numCounters
	}
}

// WithMaxCost 设置缓存的最大成本
func WithMaxCost(maxCost int64) Option {
	return func(o *options) {
		o.maxCost = maxCost
	}
}

// WithBufferItems 设置每个Get缓冲区的键数量
func WithBufferItems(bufferItems int64) Option {
	return func(o *options) {
		o.bufferItems = bufferItems
	}
}

// InitMemory 创建内存缓存
func InitMemory(opts ...Option) *ristretto.Cache {
	o := defaultOptions()
	o.apply(opts...)

	// 参考: https://dgraph.io/blog/post/introducing-ristretto-high-perf-go-cache/
	//		https://www.start.io/blog/we-chose-ristretto-cache-for-go-heres-why/
	config := &ristretto.Config{
		NumCounters: o.numCounters,
		MaxCost:     o.maxCost,
		BufferItems: o.bufferItems,
	}
	store, err := ristretto.NewCache(config)
	if err != nil {
		panic(err)
	}
	return store
}

// ----------------------------------------------------------------------------

// 全局内存缓存客户端
var (
	memoryCli *ristretto.Cache
	once      sync.Once
)

// InitGlobalMemory 初始化全局内存缓存
func InitGlobalMemory(opts ...Option) {
	memoryCli = InitMemory(opts...)
}

// GetGlobalMemoryCli 获取内存缓存客户端
func GetGlobalMemoryCli() *ristretto.Cache {
	if memoryCli == nil {
		once.Do(func() {
			memoryCli = InitMemory() // 默认选项
		})
	}
	return memoryCli
}

// CloseGlobalMemory 关闭内存缓存
func CloseGlobalMemory() error {
	if memoryCli != nil {
		memoryCli.Close()
	}
	return nil
}

// ----------------------------------------------------------------------------

type memoryCache struct {
	client            *ristretto.Cache
	KeyPrefix         string
	encoding          Encoding
	DefaultExpireTime time.Duration
	newObject         func() interface{}
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(keyPrefix string, encode Encoding, newObject func() interface{}) Cache {
	return &memoryCache{
		client:    GetGlobalMemoryCli(),
		KeyPrefix: keyPrefix,
		encoding:  encode,
		newObject: newObject,
	}
}

// Set 设置数据
func (m *memoryCache) Set(_ context.Context, key string, val interface{}, expiration time.Duration) error {
	buf, err := Marshal(m.encoding, val)
	if err != nil {
		return fmt.Errorf("编码错误: %v, 键=%s, 值=%+v ", err, key, val)
	}
	if len(buf) == 0 {
		buf = NotFoundPlaceholderBytes
	}
	cacheKey, err := BuildCacheKey(m.KeyPrefix, key)
	if err != nil {
		return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
	}
	ok := m.client.SetWithTTL(cacheKey, buf, 0, expiration)
	if !ok {
		return errors.New("SetWithTTL失败")
	}
	m.client.Wait()

	return nil
}

// Get 获取数据
func (m *memoryCache) Get(_ context.Context, key string, val interface{}) error {
	cacheKey, err := BuildCacheKey(m.KeyPrefix, key)
	if err != nil {
		return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
	}

	data, ok := m.client.Get(cacheKey)
	if !ok {
		return CacheNotFound // 未找到，转换为redis nil错误
	}

	dataBytes, ok := data.([]byte)
	if !ok {
		return fmt.Errorf("数据类型错误, 键=%s, 类型=%T", key, data)
	}

	if len(dataBytes) == 0 || bytes.Equal(dataBytes, NotFoundPlaceholderBytes) {
		return ErrPlaceholder
	}

	err = Unmarshal(m.encoding, dataBytes, val)
	if err != nil {
		return fmt.Errorf("解码错误: %v, 键=%s, 缓存键=%s, 类型=%T, 数据=%s ",
			err, key, cacheKey, val, dataBytes)
	}
	return nil
}

// Del 删除数据
func (m *memoryCache) Del(_ context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	key := keys[0]
	cacheKey, err := BuildCacheKey(m.KeyPrefix, key)
	if err != nil {
		return fmt.Errorf("构建缓存键错误, 错误=%v, 键=%s", err, key)
	}
	m.client.Del(cacheKey)
	return nil
}

// MultiSet 批量设置数据
func (m *memoryCache) MultiSet(ctx context.Context, valueMap map[string]interface{}, expiration time.Duration) error {
	var err error
	for key, value := range valueMap {
		err = m.Set(ctx, key, value, expiration)
		if err != nil {
			return err
		}
	}
	return nil
}

// MultiGet 批量获取数据
func (m *memoryCache) MultiGet(ctx context.Context, keys []string, value interface{}) error {
	valueMap := reflect.ValueOf(value)
	var err error
	for _, key := range keys {
		object := m.newObject()
		err = m.Get(ctx, key, object)
		if err != nil {
			continue
		}
		valueMap.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(object))
	}

	return nil
}

// SetCacheWithNotFound 设置未找到的缓存
func (m *memoryCache) SetCacheWithNotFound(_ context.Context, key string) error {
	cacheKey, err := BuildCacheKey(m.KeyPrefix, key)
	if err != nil {
		return fmt.Errorf("构建缓存键错误: %v, 键=%s", err, key)
	}

	ok := m.client.SetWithTTL(cacheKey, []byte(NotFoundPlaceholder), 0, DefaultNotFoundExpireTime)
	if !ok {
		return errors.New("SetWithTTL失败")
	}

	return nil
}
