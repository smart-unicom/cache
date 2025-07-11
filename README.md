# Cache 缓存组件库

一个功能强大、易于使用的 Go 缓存组件库，支持内存缓存、Redis 单机缓存和 Redis 集群缓存。

## 🚀 特性

- **多种缓存类型支持**：内存缓存（基于 Ristretto）、Redis 单机、Redis 集群
- **统一接口设计**：通过 `Cache` 接口提供一致的 API
- **配置驱动**：支持通过配置文件或代码进行缓存配置
- **提供者模式**：使用 Provider 模式管理缓存实例的生命周期
- **多实例管理**：支持同时管理多个不同类型的缓存实例
- **全局缓存支持**：提供全局缓存实例，简化使用
- **类型安全**：支持泛型和类型安全的序列化/反序列化
- **高性能**：内存缓存基于高性能的 Ristretto 库
- **连接池管理**：Redis 缓存支持连接池配置和管理
- **错误处理**：完善的错误处理和中文错误信息
- **测试覆盖**：完整的单元测试和性能基准测试

## 📦 安装

```bash
go get github.com/smart-unicom/cache
```

## 🔧 依赖

- `github.com/dgraph-io/ristretto` - 高性能内存缓存
- `github.com/redis/go-redis/v9` - Redis 客户端

## 📖 快速开始

### 使用提供者模式（推荐）

#### 内存缓存

```go
package main

import (
	"context"
	"fmt"
	"time"
	"github.com/smart-unicom/cache"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func newUser() interface{} {
	return &User{}
}

func main() {
	// 使用默认内存缓存配置
	config := cache.DefaultConfig()
	config.KeyPrefix = "myapp:"
	config.DefaultExpireTime = time.Hour

	// 创建缓存提供者
	provider, err := cache.NewProvider(config, &cache.JSONEncoding{}, newUser)
	if err != nil {
		panic(err)
	}
	defer provider.Close()

	// 获取缓存实例
	c := provider.GetCache()

	// 缓存操作
	ctx := context.Background()
	user := &User{ID: 1, Name: "张三", Age: 25}

	// 设置缓存
	err = c.Set(ctx, "user:1", user, time.Minute*10)
	if err != nil {
		fmt.Printf("设置缓存失败: %v\n", err)
		return
	}

	// 获取缓存
	var result User
	err = c.Get(ctx, "user:1", &result)
	if err != nil {
		fmt.Printf("获取缓存失败: %v\n", err)
		return
	}

	fmt.Printf("用户信息: %+v\n", result)
}
```

#### Redis 缓存

```go
package main

import (
	"context"
	"fmt"
	"time"
	"github.com/smart-unicom/cache"
)

func main() {
	// 使用默认 Redis 配置
	config := cache.DefaultRedisConfig("localhost:6379")
	config.KeyPrefix = "myapp:"
	config.Redis.Password = "your-password" // 如果需要密码
	config.Redis.DB = 0

	// 创建缓存提供者
	provider, err := cache.NewProvider(config, &cache.JSONEncoding{}, newUser)
	if err != nil {
		panic(err)
	}
	defer provider.Close()

	// 获取缓存实例
	c := provider.GetCache()

	// 缓存操作
	ctx := context.Background()
	user := &User{ID: 1, Name: "李四", Age: 30}

	err = c.Set(ctx, "user:1", user, time.Hour)
	if err != nil {
		fmt.Printf("设置缓存失败: %v\n", err)
		return
	}

	var result User
	err = c.Get(ctx, "user:1", &result)
	if err != nil {
		fmt.Printf("获取缓存失败: %v\n", err)
		return
	}

	fmt.Printf("用户信息: %+v\n", result)
}
```

#### Redis 集群缓存

```go
package main

import (
	"github.com/smart-unicom/cache"
)

func main() {
	// Redis 集群配置
	addrs := []string{
		"localhost:7000",
		"localhost:7001",
		"localhost:7002",
	}
	config := cache.DefaultRedisClusterConfig(addrs)
	config.KeyPrefix = "cluster:"

	// 创建缓存提供者
	provider, err := cache.NewProvider(config, &cache.JSONEncoding{}, newUser)
	if err != nil {
		panic(err)
	}
	defer provider.Close()

	// 使用缓存...
}
```

### 使用全局缓存

```go
package main

import (
	"context"
	"github.com/smart-unicom/cache"
)

func main() {
	// 设置全局缓存
	config := cache.DefaultConfig()
	err := cache.SetupGlobalCache(config, &cache.JSONEncoding{}, newUser)
	if err != nil {
		panic(err)
	}

	// 直接使用全局函数
	ctx := context.Background()
	user := &User{ID: 1, Name: "王五", Age: 28}

	// 设置缓存
	cache.Set(ctx, "user:1", user, time.Hour)

	// 获取缓存
	var result User
	cache.Get(ctx, "user:1", &result)
}
```

### 使用缓存管理器

```go
package main

import (
	"github.com/smart-unicom/cache"
)

func main() {
	// 创建缓存管理器
	manager := cache.NewManager()
	defer manager.CloseAll()

	// 添加内存缓存
	memoryConfig := cache.DefaultConfig()
	memoryProvider, _ := cache.NewProvider(memoryConfig, &cache.JSONEncoding{}, newUser)
	manager.AddProvider("memory", memoryProvider)

	// 添加 Redis 缓存
	redisConfig := cache.DefaultRedisConfig("localhost:6379")
	redisProvider, _ := cache.NewProvider(redisConfig, &cache.JSONEncoding{}, newUser)
	manager.AddProvider("redis", redisProvider)

	// 使用不同的缓存
	memoryCache, _ := manager.GetCache("memory")
	redisCache, _ := manager.GetCache("redis")

	// 分别操作不同的缓存...
}
```

## 🔧 配置选项

### 内存缓存配置

```go
config := &cache.Config{
	Type:              cache.MemoryCache,
	KeyPrefix:         "myapp:",
	DefaultExpireTime: time.Hour,
	Memory: &cache.MemoryConfig{
		NumCounters: 1e7,     // 跟踪频率的键数量
		MaxCost:     1 << 30, // 缓存的最大成本 (1GB)
		BufferItems: 64,      // 每个Get缓冲区的键数量
	},
}
```

### Redis 缓存配置

```go
config := &cache.Config{
	Type:              cache.RedisCache,
	KeyPrefix:         "myapp:",
	DefaultExpireTime: time.Hour,
	Redis: &cache.RedisConfig{
		Addr:            "localhost:6379",
		Password:        "your-password",
		DB:              0,
		PoolSize:        10,
		MinIdleConns:    2,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
	},
}
```

### Redis 集群配置

```go
config := &cache.Config{
	Type:              cache.RedisClusterCache,
	KeyPrefix:         "myapp:",
	DefaultExpireTime: time.Hour,
	RedisCluster: &cache.RedisClusterConfig{
		Addrs:           []string{"localhost:7000", "localhost:7001"},
		Password:        "your-password",
		PoolSize:        10,
		MinIdleConns:    2,
		ConnMaxLifetime: time.Hour,
		DialTimeout:     5 * time.Second,
		ReadTimeout:     3 * time.Second,
		WriteTimeout:    3 * time.Second,
	},
}
```

## 📚 API 文档

### Cache 接口

```go
type Cache interface {
	// Set 设置缓存
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	
	// Get 获取缓存
	Get(ctx context.Context, key string, value interface{}) error
	
	// Del 删除缓存
	Del(ctx context.Context, keys ...string) error
	
	// MultiSet 批量设置缓存
	MultiSet(ctx context.Context, valueMap map[string]interface{}, expiration time.Duration) error
	
	// MultiGet 批量获取缓存
	MultiGet(ctx context.Context, keys []string, value interface{}) error
	
	// SetCacheWithNotFound 设置缓存（包含未找到标记）
	SetCacheWithNotFound(ctx context.Context, key string, value interface{}, expiration time.Duration) error
}
```

### Provider 接口

```go
type Provider interface {
	// GetCache 获取缓存实例
	GetCache() Cache
	
	// Close 关闭缓存连接
	Close() error
}
```

### 全局函数

```go
// 设置缓存
func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error

// 获取缓存
func Get(ctx context.Context, key string, value interface{}) error

// 删除缓存
func Del(ctx context.Context, keys ...string) error

// 批量设置缓存
func MultiSet(ctx context.Context, valueMap map[string]interface{}, expiration time.Duration) error

// 批量获取缓存
func MultiGet(ctx context.Context, keys []string, value interface{}) error
```

## 🧪 测试

运行所有测试：

```bash
go test -v ./...
```

运行性能测试：

```bash
go test -bench=. -benchmem
```

跳过需要 Redis 服务器的测试：

```bash
go test -short
```

## 📝 最佳实践

1. **使用提供者模式**：推荐使用 `Provider` 模式管理缓存实例，便于资源管理和配置
2. **合理设置过期时间**：根据业务需求设置合适的缓存过期时间
3. **键命名规范**：使用有意义的键前缀，避免键冲突
4. **错误处理**：始终检查缓存操作的错误返回值
5. **连接池配置**：根据应用负载合理配置 Redis 连接池参数
6. **内存缓存大小**：根据可用内存合理设置内存缓存的最大成本
7. **批量操作**：对于多个键的操作，优先使用批量方法提高性能

## 🤝 贡献

欢迎提交 Issue 和 Pull Request 来改进这个项目。

## 📄 许可证

本项目采用 MIT 许可证。详情请参阅 [LICENSE](LICENSE) 文件。
