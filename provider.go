package cache

import (
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/redis/go-redis/v9"
)

// CacheType 缓存类型
type CacheType string

const (
	// MemoryCache 内存缓存类型
	MemoryCache CacheType = "memory"
	// RedisCache Redis缓存类型
	RedisCache CacheType = "redis"
	// RedisClusterCache Redis集群缓存类型
	RedisClusterCache CacheType = "redis_cluster"
)

// Config 缓存配置
type Config struct {
	// Type 缓存类型
	Type CacheType `json:"type" yaml:"type"`
	// KeyPrefix 键前缀
	KeyPrefix string `json:"key_prefix" yaml:"key_prefix"`
	// DefaultExpireTime 默认过期时间
	DefaultExpireTime time.Duration `json:"default_expire_time" yaml:"default_expire_time"`
	// Memory 内存缓存配置
	Memory *MemoryConfig `json:"memory,omitempty" yaml:"memory,omitempty"`
	// Redis Redis缓存配置
	Redis *RedisConfig `json:"redis,omitempty" yaml:"redis,omitempty"`
	// RedisCluster Redis集群缓存配置
	RedisCluster *RedisClusterConfig `json:"redis_cluster,omitempty" yaml:"redis_cluster,omitempty"`
}

// MemoryConfig 内存缓存配置
type MemoryConfig struct {
	// NumCounters 跟踪频率的键数量
	NumCounters int64 `json:"num_counters" yaml:"num_counters"`
	// MaxCost 缓存的最大成本
	MaxCost int64 `json:"max_cost" yaml:"max_cost"`
	// BufferItems 每个Get缓冲区的键数量
	BufferItems int64 `json:"buffer_items" yaml:"buffer_items"`
}

// RedisConfig Redis缓存配置
type RedisConfig struct {
	// Addr Redis服务器地址
	Addr string `json:"addr" yaml:"addr"`
	// Password Redis密码
	Password string `json:"password" yaml:"password"`
	// DB Redis数据库索引
	DB int `json:"db" yaml:"db"`
	// PoolSize 连接池大小
	PoolSize int `json:"pool_size" yaml:"pool_size"`
	// MinIdleConns 最小空闲连接数
	MinIdleConns int `json:"min_idle_conns" yaml:"min_idle_conns"`
	// MaxIdleConns 最大空闲连接数
	MaxIdleConns int `json:"max_idle_conns" yaml:"max_idle_conns"`
	// ConnMaxLifetime 连接最大生存时间
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	// DialTimeout 连接超时时间
	DialTimeout time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	// ReadTimeout 读取超时时间
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`
	// WriteTimeout 写入超时时间
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
}

// RedisClusterConfig Redis集群缓存配置
type RedisClusterConfig struct {
	// Addrs Redis集群节点地址列表
	Addrs []string `json:"addrs" yaml:"addrs"`
	// Password Redis密码
	Password string `json:"password" yaml:"password"`
	// PoolSize 连接池大小
	PoolSize int `json:"pool_size" yaml:"pool_size"`
	// MinIdleConns 最小空闲连接数
	MinIdleConns int `json:"min_idle_conns" yaml:"min_idle_conns"`
	// MaxIdleConns 最大空闲连接数
	MaxIdleConns int `json:"max_idle_conns" yaml:"max_idle_conns"`
	// ConnMaxLifetime 连接最大生存时间
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	// DialTimeout 连接超时时间
	DialTimeout time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	// ReadTimeout 读取超时时间
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`
	// WriteTimeout 写入超时时间
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
}

// Provider 缓存提供者接口
type Provider interface {
	// GetCache 获取缓存实例
	GetCache() Cache
	// Close 关闭缓存连接
	Close() error
}

// memoryProvider 内存缓存提供者
type memoryProvider struct {
	cache  Cache
	client *ristretto.Cache
}

// GetCache 获取内存缓存实例
func (p *memoryProvider) GetCache() Cache {
	return p.cache
}

// Close 关闭内存缓存
func (p *memoryProvider) Close() error {
	if p.client != nil {
		p.client.Close()
	}
	return nil
}

// redisProvider Redis缓存提供者
type redisProvider struct {
	cache  Cache
	client *redis.Client
}

// GetCache 获取Redis缓存实例
func (p *redisProvider) GetCache() Cache {
	return p.cache
}

// Close 关闭Redis连接
func (p *redisProvider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// redisClusterProvider Redis集群缓存提供者
type redisClusterProvider struct {
	cache  Cache
	client *redis.ClusterClient
}

// GetCache 获取Redis集群缓存实例
func (p *redisClusterProvider) GetCache() Cache {
	return p.cache
}

// Close 关闭Redis集群连接
func (p *redisClusterProvider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// NewProvider 创建缓存提供者
func NewProvider(config *Config, encoding Encoding, newObject func() interface{}) (Provider, error) {
	if config == nil {
		return nil, fmt.Errorf("缓存配置不能为空")
	}

	switch config.Type {
	case MemoryCache:
		return newMemoryProvider(config, encoding, newObject)
	case RedisCache:
		return newRedisProvider(config, encoding, newObject)
	case RedisClusterCache:
		return newRedisClusterProvider(config, encoding, newObject)
	default:
		return nil, fmt.Errorf("不支持的缓存类型: %s", config.Type)
	}
}

// newMemoryProvider 创建内存缓存提供者
func newMemoryProvider(config *Config, encoding Encoding, newObject func() interface{}) (Provider, error) {
	if config.Memory == nil {
		config.Memory = defaultMemoryConfig()
	}

	// 创建内存缓存客户端
	client := InitMemory(
		WithNumCounters(config.Memory.NumCounters),
		WithMaxCost(config.Memory.MaxCost),
		WithBufferItems(config.Memory.BufferItems),
	)

	// 创建内存缓存实例
	cache := &memoryCache{
		client:            client,
		KeyPrefix:         config.KeyPrefix,
		encoding:          encoding,
		DefaultExpireTime: config.DefaultExpireTime,
		newObject:         newObject,
	}

	return &memoryProvider{
		cache:  cache,
		client: client,
	}, nil
}

// newRedisProvider 创建Redis缓存提供者
func newRedisProvider(config *Config, encoding Encoding, newObject func() interface{}) (Provider, error) {
	if config.Redis == nil {
		return nil, fmt.Errorf("Redis配置不能为空")
	}

	// 设置默认值
	redisConfig := config.Redis
	if redisConfig.PoolSize == 0 {
		redisConfig.PoolSize = 10
	}
	if redisConfig.MinIdleConns == 0 {
		redisConfig.MinIdleConns = 2
	}
	if redisConfig.ConnMaxLifetime == 0 {
		redisConfig.ConnMaxLifetime = time.Hour
	}
	if redisConfig.DialTimeout == 0 {
		redisConfig.DialTimeout = 5 * time.Second
	}
	if redisConfig.ReadTimeout == 0 {
		redisConfig.ReadTimeout = 3 * time.Second
	}
	if redisConfig.WriteTimeout == 0 {
		redisConfig.WriteTimeout = 3 * time.Second
	}

	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr:            redisConfig.Addr,
		Password:        redisConfig.Password,
		DB:              redisConfig.DB,
		PoolSize:        redisConfig.PoolSize,
		MinIdleConns:    redisConfig.MinIdleConns,
		MaxIdleConns:    redisConfig.MaxIdleConns,
		ConnMaxLifetime: redisConfig.ConnMaxLifetime,
		DialTimeout:     redisConfig.DialTimeout,
		ReadTimeout:     redisConfig.ReadTimeout,
		WriteTimeout:    redisConfig.WriteTimeout,
	})

	// 创建Redis缓存实例
	cache := &redisCache{
		client:            client,
		KeyPrefix:         config.KeyPrefix,
		encoding:          encoding,
		DefaultExpireTime: config.DefaultExpireTime,
		newObject:         newObject,
	}

	return &redisProvider{
		cache:  cache,
		client: client,
	}, nil
}

// newRedisClusterProvider 创建Redis集群缓存提供者
func newRedisClusterProvider(config *Config, encoding Encoding, newObject func() interface{}) (Provider, error) {
	if config.RedisCluster == nil {
		return nil, fmt.Errorf("Redis集群配置不能为空")
	}

	if len(config.RedisCluster.Addrs) == 0 {
		return nil, fmt.Errorf("Redis集群地址列表不能为空")
	}

	// 设置默认值
	clusterConfig := config.RedisCluster
	if clusterConfig.PoolSize == 0 {
		clusterConfig.PoolSize = 10
	}
	if clusterConfig.MinIdleConns == 0 {
		clusterConfig.MinIdleConns = 2
	}
	if clusterConfig.ConnMaxLifetime == 0 {
		clusterConfig.ConnMaxLifetime = time.Hour
	}
	if clusterConfig.DialTimeout == 0 {
		clusterConfig.DialTimeout = 5 * time.Second
	}
	if clusterConfig.ReadTimeout == 0 {
		clusterConfig.ReadTimeout = 3 * time.Second
	}
	if clusterConfig.WriteTimeout == 0 {
		clusterConfig.WriteTimeout = 3 * time.Second
	}

	// 创建Redis集群客户端
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:           clusterConfig.Addrs,
		Password:        clusterConfig.Password,
		PoolSize:        clusterConfig.PoolSize,
		MinIdleConns:    clusterConfig.MinIdleConns,
		MaxIdleConns:    clusterConfig.MaxIdleConns,
		ConnMaxLifetime: clusterConfig.ConnMaxLifetime,
		DialTimeout:     clusterConfig.DialTimeout,
		ReadTimeout:     clusterConfig.ReadTimeout,
		WriteTimeout:    clusterConfig.WriteTimeout,
	})

	// 创建Redis集群缓存实例
	cache := &redisClusterCache{
		client:            client,
		KeyPrefix:         config.KeyPrefix,
		encoding:          encoding,
		DefaultExpireTime: config.DefaultExpireTime,
		newObject:         newObject,
	}

	return &redisClusterProvider{
		cache:  cache,
		client: client,
	}, nil
}

// defaultMemoryConfig 默认内存缓存配置
func defaultMemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		NumCounters: 1e7,     // 10M
		MaxCost:     1 << 30, // 1GB
		BufferItems: 64,
	}
}

// DefaultConfig 默认缓存配置
func DefaultConfig() *Config {
	return &Config{
		Type:              MemoryCache,
		KeyPrefix:         "",
		DefaultExpireTime: DefaultExpireTime,
		Memory:            defaultMemoryConfig(),
	}
}

// DefaultRedisConfig 默认Redis缓存配置
func DefaultRedisConfig(addr string) *Config {
	return &Config{
		Type:              RedisCache,
		KeyPrefix:         "",
		DefaultExpireTime: DefaultExpireTime,
		Redis: &RedisConfig{
			Addr:            addr,
			Password:        "",
			DB:              0,
			PoolSize:        10,
			MinIdleConns:    2,
			ConnMaxLifetime: time.Hour,
			DialTimeout:     5 * time.Second,
			ReadTimeout:     3 * time.Second,
			WriteTimeout:    3 * time.Second,
		},
	}
}

// DefaultRedisClusterConfig 默认Redis集群缓存配置
func DefaultRedisClusterConfig(addrs []string) *Config {
	return &Config{
		Type:              RedisClusterCache,
		KeyPrefix:         "",
		DefaultExpireTime: DefaultExpireTime,
		RedisCluster: &RedisClusterConfig{
			Addrs:           addrs,
			Password:        "",
			PoolSize:        10,
			MinIdleConns:    2,
			ConnMaxLifetime: time.Hour,
			DialTimeout:     5 * time.Second,
			ReadTimeout:     3 * time.Second,
			WriteTimeout:    3 * time.Second,
		},
	}
}

// SetupGlobalCache 设置全局缓存
func SetupGlobalCache(config *Config, encoding Encoding, newObject func() interface{}) error {
	provider, err := NewProvider(config, encoding, newObject)
	if err != nil {
		return fmt.Errorf("创建缓存提供者失败: %w", err)
	}

	DefaultClient = provider.GetCache()
	return nil
}

// Manager 缓存管理器
type Manager struct {
	providers map[string]Provider
}

// NewManager 创建缓存管理器
func NewManager() *Manager {
	return &Manager{
		providers: make(map[string]Provider),
	}
}

// AddProvider 添加缓存提供者
func (m *Manager) AddProvider(name string, provider Provider) {
	m.providers[name] = provider
}

// GetProvider 获取缓存提供者
func (m *Manager) GetProvider(name string) (Provider, bool) {
	provider, exists := m.providers[name]
	return provider, exists
}

// GetCache 获取缓存实例
func (m *Manager) GetCache(name string) (Cache, bool) {
	provider, exists := m.providers[name]
	if !exists {
		return nil, false
	}
	return provider.GetCache(), true
}

// CloseAll 关闭所有缓存连接
func (m *Manager) CloseAll() error {
	var lastErr error
	for name, provider := range m.providers {
		if err := provider.Close(); err != nil {
			lastErr = fmt.Errorf("关闭缓存提供者 %s 失败: %w", name, err)
		}
	}
	return lastErr
}

// RemoveProvider 移除缓存提供者
func (m *Manager) RemoveProvider(name string) error {
	provider, exists := m.providers[name]
	if !exists {
		return fmt.Errorf("缓存提供者 %s 不存在", name)
	}

	if err := provider.Close(); err != nil {
		return fmt.Errorf("关闭缓存提供者 %s 失败: %w", name, err)
	}

	delete(m.providers, name)
	return nil
}

// ListProviders 列出所有缓存提供者名称
func (m *Manager) ListProviders() []string {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names
}