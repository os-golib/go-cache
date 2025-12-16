# Go-Cache: A Comprehensive Caching Library for Go

Go-Cache is a high-performance, extensible caching library for Go applications with support for multiple backends, advanced features, and seamless integrations.

## Features

- 🚀 **Multi-backend Support**: Memory and Redis backends
- ⚡ **Advanced Operations**: GetOrSet, GetOrSetLocked, pipeline operations
- 🔒 **Distributed Locking**: Safe concurrent cache population
- 📊 **Metrics & Monitoring**: Built-in metrics collection
- 🔌 **Integrations**: GORM, FastHTTP middleware, and more
- 🛠️ **Builder Pattern**: Clean configuration management
- 🎯 **Cache Strategies**: Cache-aside, write-through, time-based invalidation
- 📈 **High Performance**: Pipeline operations, concurrent execution

## Installation

```bash
go get github.com/os-golib/go-cache
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "time"
    
    "github.com/os-golib/go-cache"
)

func main() {
    ctx := context.Background()
    
    // Create memory cache
    memCache, err := cache.NewMemory[string]()
    if err != nil {
        panic(err)
    }
    defer memCache.Close()
    
    // Basic operations
    err = memCache.Set(ctx, "key", "value", 5*time.Minute)
    if err != nil {
        panic(err)
    }
    
    val, err := memCache.Get(ctx, "key")
    if err != nil {
        panic(err)
    }
    println("Retrieved:", val)
}
```

### Advanced Cache with Redis

```go
func exampleRedisCache() {
    cfg, err := cache.NewBuilder().
        WithRedis("redis://localhost:6379/0").
        WithTTL(10 * time.Minute).
        Build()
    if err != nil {
        panic(err)
    }
    
    ac, err := cache.NewAdvanced[User](cfg)
    if err != nil {
        panic(err)
    }
    defer ac.Close()
    
    // GetOrSet with compute function
    user, err := ac.GetOrSet(ctx, "user:1", 30*time.Minute, func() (User, error) {
        // Expensive computation or DB query
        return User{ID: 1, Name: "John"}, nil
    })
}
```

## Backends

### Memory Cache
In-memory cache with LRU eviction and configurable limits:

```go
cfg := cache.NewBuilder().
    WithMemory().
    WithMaxEntries(10000).
    WithTTL(5*time.Minute).
    MustBuild()

cache, _ := cache.New[string](cfg)
```

### Redis Cache
Redis backend with connection pooling and pipeline support:

```go
cfg := cache.NewBuilder().
    WithRedis("redis://localhost:6379/0").
    WithPoolSize(20).
    WithMinIdleConn(5).
    MustBuild()

cache, _ := cache.NewAdvanced[string](cfg)
```

## Advanced Features

### GetOrSet with Locking
Prevent cache stampede with distributed locking:

```go
val, err := cache.GetOrSetLocked(ctx, "expensive:key", 30*time.Second, func() (string, error) {
    // Only one goroutine computes this at a time
    return computeExpensiveValue(), nil
})
```

### Pipeline Operations
Batch operations for better performance:

```go
// Batch set
items := map[string]string{
    "user:1": "Alice",
    "user:2": "Bob",
}
err := cache.SetManyPipeline(ctx, items, 10*time.Minute)

// Batch get
results, err := cache.GetManyPipeline(ctx, []string{"user:1", "user:2"})
```

### Key Builder
Consistent key generation:

```go
keyBuilder := cache.NewKeyBuilder("myapp")
userKey := keyBuilder.Add("users").Add("1").Build() // "myapp:users:1"
```

## Integrations

### GORM Integration
Cache database queries automatically:

```go
db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
cache, _ := cache.NewAdvanced[User](cfg)

gormCache := integration.NewGORMCache[User](cache, db)

// Automatically caches DB queries
user, err := gormCache.GetByID(ctx, 1, 10*time.Second)
```

### FastHTTP Middleware
HTTP response caching:

```go
cache, _ := cache.NewAdvanced[[]byte](cfg)
mw := integration.NewHTTPCache[[]byte](cache, 30*time.Second)

handler := func(ctx *fasthttp.RequestCtx) {
    ctx.SetBodyString("Hello, cached world!")
}

cachedHandler := mw.Handler(handler)
fasthttp.ListenAndServe(":8080", cachedHandler)
```

## Configuration

### Programmatic Configuration

```go
cfg := cache.NewBuilder().
    WithRedis("redis://localhost:6379/0").
    WithTTL(10 * time.Minute).
    WithPoolSize(20).
    WithMinIdleConn(5).
    WithMaxRetries(3).
    WithPrefix("myapp:").
    MustBuild()
```

### YAML Configuration

```yaml
# redis.yaml
type: redis
redis_url: "redis://localhost:6379/0"
ttl: 10m
pool_size: 20
min_idle: 5
prefix: "myapp:"
```

Load from file:
```go
cfg, err := cache.NewBuilder().
    WithLoadFromFile("redis.yaml").
    Build()
```

## Metrics

Built-in metrics collection:

```go
stats := cache.Stats(ctx)
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate)
fmt.Printf("Total items: %d\n", stats.Items)

// Detailed operation metrics
metrics := cache.Metrics().Snapshot()
for op, stats := range metrics {
    fmt.Printf("%s: Hits=%d, Misses=%d, Avg=%v\n", 
        op, stats.Hits, stats.Misses, stats.AvgDuration)
}
```

## Error Handling

Consistent error types and classification:

```go
val, err := cache.Get(ctx, "key")
if err != nil {
    if errors.Is(err, base.ErrCacheMiss) {
        // Cache miss - normal case
    } else if base.IsConnectionError(err) {
        // Connection issue - may be retryable
    } else if base.IsContextError(err) {
        // Operation timed out
    }
}
```

## Best Practices

1. **Key Design**: Use consistent key patterns with `KeyBuilder`
2. **TTL Strategy**: Set appropriate TTLs based on data volatility
3. **Pipeline Operations**: Use batch operations for better performance
4. **Error Handling**: Always handle cache misses gracefully
5. **Monitoring**: Track hit rates and operation metrics
6. **Connection Pooling**: Configure appropriate pool sizes for Redis
7. **Locking**: Use `GetOrSetLocked` for expensive computations

## Performance Tips

- Use pipeline operations for bulk reads/writes
- Configure appropriate connection pool sizes
- Enable compression for large values
- Use memory cache for high-frequency, low-latency data
- Monitor and adjust eviction policies based on usage patterns

## License

MIT License

## Contributing

Contributions are welcome! Please see the contributing guidelines for details.

## Support

- GitHub Issues: [github.com/os-golib/go-cache/issues](https://github.com/os-golib/go-cache/issues)
- Documentation: [pkg.go.dev/github.com/os-golib/go-cache](https://pkg.go.dev/github.com/os-golib/go-cache)

---

**Go-Cache** is actively maintained and used in production environments. For more examples and advanced usage, check the `examples/` directory in the repository.