// Package cache 内存和Redis缓存库
package cache

import (
	"context"
	"errors"
	"time"
)

var (
	// DefaultExpireTime 默认过期时间
	DefaultExpireTime = time.Hour * 24
	// DefaultNotFoundExpireTime 结果为空时的过期时间，1分钟
	// 通常用于数据为空时的缓存时间（缓存穿透）
	DefaultNotFoundExpireTime = time.Minute * 10

	// NotFoundPlaceholder 占位符
	NotFoundPlaceholder      = "*"
	NotFoundPlaceholderBytes = []byte(NotFoundPlaceholder)
	ErrPlaceholder           = errors.New("缓存: 占位符")

	// DefaultClient 生成缓存客户端，keyPrefix通常是业务前缀
	DefaultClient Cache
)

// Cache 缓存驱动接口
type Cache interface {
	Set(ctx context.Context, key string, val interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, val interface{}) error
	MultiSet(ctx context.Context, valMap map[string]interface{}, expiration time.Duration) error
	MultiGet(ctx context.Context, keys []string, valueMap interface{}) error
	Del(ctx context.Context, keys ...string) error
	SetCacheWithNotFound(ctx context.Context, key string) error
}

// Set 设置数据
func Set(ctx context.Context, key string, val interface{}, expiration time.Duration) error {
	return DefaultClient.Set(ctx, key, val, expiration)
}

// Get 获取数据
func Get(ctx context.Context, key string, val interface{}) error {
	return DefaultClient.Get(ctx, key, val)
}

// MultiSet 批量设置数据
func MultiSet(ctx context.Context, valMap map[string]interface{}, expiration time.Duration) error {
	return DefaultClient.MultiSet(ctx, valMap, expiration)
}

// MultiGet 批量获取数据
func MultiGet(ctx context.Context, keys []string, valueMap interface{}) error {
	return DefaultClient.MultiGet(ctx, keys, valueMap)
}

// Del 批量删除数据
func Del(ctx context.Context, keys ...string) error {
	return DefaultClient.Del(ctx, keys...)
}

// SetCacheWithNotFound 设置未找到的缓存
func SetCacheWithNotFound(ctx context.Context, key string) error {
	return DefaultClient.SetCacheWithNotFound(ctx, key)
}
