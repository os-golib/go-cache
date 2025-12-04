package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/base"
	"github.com/os-golib/go-cache/internal/metrics"
)

// redisCache is a Redis-backed cache implementation
type redisCache[T any] struct {
	base.Base
	Cache      *base.Cache
	client     *redis.Client
	serializer config.Serializer[T]
	startTime  time.Time
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache[T any](cfg config.Config, serializer config.Serializer[T]) (*redisCache[T], error) {
	return NewRedisContext(context.Background(), cfg, serializer)
}

// NewRedisContext creates a new Redis cache instance using caller-provided context NewRedisCache
func NewRedisContext[T any](
	ctx context.Context,
	cfg config.Config,
	serializer config.Serializer[T],
) (*redisCache[T], error) {
	if serializer == nil {
		serializer = &config.JsonSerializer[T]{}
	}

	opt, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, config.NewError("redis config", err, "")
	}

	// Apply custom redis options
	opt.PoolSize = cfg.PoolSize
	opt.MinIdleConns = cfg.MinIdleConn
	opt.MaxRetries = cfg.MaxRetries
	opt.DialTimeout = cfg.DialTimeout
	opt.ReadTimeout = cfg.ReadTimeout
	opt.WriteTimeout = cfg.WriteTimeout

	client := redis.NewClient(opt)

	// If no deadline is set on the incoming context, enforce a default timeout.
	if _, ok := ctx.Deadline(); !ok {
		timeout := cfg.ConnTimeout
		if timeout <= 0 {
			timeout = 5 * time.Second // sane fallback
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, config.NewError("redis connection", config.ErrConnectionFailed, "")
	}

	return &redisCache[T]{
		Base:       base.Base{Cfg: cfg},
		Cache:      base.NewCache(),
		client:     client,
		serializer: serializer,
		startTime:  time.Now(),
	}, nil
}

// Get retrieves a value from Redis
func (r *redisCache[T]) Get(ctx context.Context, key string) (T, error) {
	start := time.Now()
	defer func() { r.Cache.RecordOperation("get", time.Since(start), 1) }()

	var zero T
	if err := r.CheckContext(ctx); err != nil {
		return zero, err
	}
	if err := r.ValidateKey(key); err != nil {
		return zero, err
	}
	fullKey := r.FullKey(key)

	data, err := r.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		r.Cache.RecordMiss("get", 1)
		return zero, config.ErrCacheMiss
	}
	if err != nil {
		r.Cache.RecordError("get")
		return zero, config.NewError("redis get", err, key)
	}

	val, err := r.serializer.Deserialize(data)
	if err != nil {
		r.Cache.RecordError("get")
		return zero, config.NewError("deserialization", err, key)
	}

	r.Cache.RecordHit("get", 1)
	return val, nil
}

// Set stores a value in Redis
func (r *redisCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	start := time.Now()
	defer func() { r.Cache.RecordOperation("set", time.Since(start), 1) }()

	if err := r.CheckContext(ctx); err != nil {
		return err
	}
	if err := r.ValidateKey(key); err != nil {
		return err
	}
	fullKey := r.FullKey(key)
	effectiveTTL := r.TTL(ttl)

	data, err := r.serializer.Serialize(value)
	if err != nil {
		return config.NewError("serialization", err, key)
	}

	if effectiveTTL > 0 {
		err = r.client.Set(ctx, fullKey, data, effectiveTTL).Err()
	} else {
		err = r.client.Set(ctx, fullKey, data, 0).Err()
	}

	if err != nil {
		r.Cache.RecordError("set")
		return config.NewError("redis set", err, key)
	}
	return nil
}

// Delete removes keys from Redis
func (r *redisCache[T]) Delete(ctx context.Context, keys ...string) error {
	start := time.Now()
	defer func() { r.Cache.RecordOperation("delete", time.Since(start), len(keys)) }()

	if len(keys) == 0 {
		return nil
	}
	if err := r.CheckContext(ctx); err != nil {
		return err
	}

	fullKeys := make([]string, len(keys))
	for in, k := range keys {
		fullKeys[in] = r.FullKey(k)
	}

	err := r.client.Del(ctx, fullKeys...).Err()
	if err != nil {
		r.Cache.RecordError("delete")
		return config.NewError("redis delete", err, "")
	}
	return nil
}

// Exists checks if a key exists in Redis
func (r *redisCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	start := time.Now()
	defer func() { r.Cache.RecordOperation("exists", time.Since(start), 1) }()

	if err := r.CheckContext(ctx); err != nil {
		return false, err
	}
	if err := r.ValidateKey(key); err != nil {
		return false, err
	}
	fullKey := r.FullKey(key)

	n, err := r.client.Exists(ctx, fullKey).Result()
	if err != nil {
		r.Cache.RecordError("exists")
		return false, config.NewError("redis exists", err, key)
	}
	return n > 0, nil
}

// Close closes the Redis connection
func (r *redisCache[T]) Close() error {
	return r.client.Close()
}

// Ping checks if Redis is reachable
func (r *redisCache[T]) Ping(ctx context.Context) error {
	if err := r.CheckContext(ctx); err != nil {
		return err
	}
	err := r.client.Ping(ctx).Err()
	if err != nil {
		return config.NewError("redis ping", err, "")
	}
	return nil
}

// Clear removes all keys with the prefix
func (r *redisCache[T]) Clear(ctx context.Context) error {
	start := time.Now()
	defer func() { r.Cache.RecordOperation("clear", time.Since(start), 1) }()

	if err := r.CheckContext(ctx); err != nil {
		return err
	}

	iter := r.client.Scan(ctx, 0, r.Cfg.Prefix+"*", 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if iter.Err() != nil {
		r.Cache.RecordError("clear")
		return config.NewError("redis scan", iter.Err(), "")
	}
	if len(keys) > 0 {
		if err := r.client.Del(ctx, keys...).Err(); err != nil {
			r.Cache.RecordError("clear")
			return config.NewError("redis delete", err, "")
		}
	}
	return nil
}

// Len returns the number of items with the prefix
func (r *redisCache[T]) Len(ctx context.Context) (int, error) {
	if err := r.CheckContext(ctx); err != nil {
		return 0, err
	}

	iter := r.client.Scan(ctx, 0, r.Cfg.Prefix+"*", 0).Iterator()
	count := 0
	for iter.Next(ctx) {
		count++
	}
	if iter.Err() != nil {
		r.Cache.RecordError("len")
		return 0, config.NewError("redis scan", iter.Err(), "")
	}
	return count, nil
}

// DeleteByPrefix removes all keys with the given prefix
func (r *redisCache[T]) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	start := time.Now()
	defer func() {
		r.Cache.RecordOperation("delete_by_prefix", time.Since(start), 1)
	}()

	pattern := r.FullKey(prefix) + "*"
	var cursor uint64
	var total int64

	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 1000).Result()
		if err != nil {
			r.Cache.RecordError("delete_by_prefix")
			return total, config.NewError("redis scan", err, prefix)
		}
		if len(keys) > 0 {
			deleted, err := r.client.Del(ctx, keys...).Result()
			if err != nil {
				r.Cache.RecordError("delete_by_prefix")
				return total, config.NewError("redis delete", err, prefix)
			}
			total += int64(deleted)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return total, nil
}

// Stats returns cache statistics
func (r *redisCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	// Get number of cached keys
	items, err := r.Len(ctx)
	items64 := int64(0)
	if err == nil && items >= 0 {
		items64 = int64(items)
	}

	// Collect hits & misses from metrics snapshot
	snapshot := r.Cache.Metrics().Snapshot()
	var hits, misses int64
	for _, stat := range snapshot {
		if stat.Hits > 0 {
			hits += stat.Hits
		}
		if stat.Misses > 0 {
			misses += stat.Misses
		}
	}

	return metrics.CacheStats{
		Backend:         "redis",
		Items:           items64,
		Hits:            hits,
		Misses:          misses,
		HitRate:         metrics.CalculateHitRate(hits, misses),
		Uptime:          r.Cache.Uptime(),
		RefreshTTLOnHit: r.Cfg.RefreshTTLOnHit,
	}
}
