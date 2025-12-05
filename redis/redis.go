package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/base"
	"github.com/os-golib/go-cache/internal/metrics"
)

type redisCache[T any] struct {
	*base.Base
	client     *redis.Client
	serializer config.Serializer[T]
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache[T any](cfg config.Config) (*redisCache[T], error) {
	return NewRedisContext[T](context.Background(), cfg)
}

// NewRedisContext creates a new Redis cache with context
func NewRedisContext[T any](ctx context.Context, cfg config.Config) (*redisCache[T], error) {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, base.NewError("redis config", err, "")
	}

	// Apply configuration
	opt.PoolSize = cfg.PoolSize
	opt.MinIdleConns = cfg.MinIdleConn
	opt.MaxRetries = cfg.MaxRetries
	opt.DialTimeout = cfg.DialTimeout
	opt.ReadTimeout = cfg.ReadTimeout
	opt.WriteTimeout = cfg.WriteTimeout
	// opt.MaxConnAge = cfg.MaxConnAge

	client := redis.NewClient(opt)

	// Ensure context has timeout
	if _, ok := ctx.Deadline(); !ok {
		timeout := cfg.ConnTimeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, base.NewError("redis connection", base.ErrConnectionFailed, "")
	}

	return &redisCache[T]{
		Base:       base.NewBase(cfg),
		client:     client,
		serializer: &config.JsonSerializer[T]{},
	}, nil
}

// Unified operation wrapper with metrics
func (r *redisCache[T]) execute(ctx context.Context, op string, itemCount int, fn func() error) error {
	start := time.Now()
	defer func() {
		r.RecordOperation(op, time.Since(start), itemCount)
	}()

	if err := r.CheckContext(ctx); err != nil {
		return err
	}

	if err := fn(); err != nil {
		r.RecordError(op)
		return err
	}
	return nil
}

// Get retrieves a value from Redis
func (r *redisCache[T]) Get(ctx context.Context, key string) (T, error) {
	var val T

	err := r.execute(ctx, "get", 1, func() error {
		if err := r.ValidateKey(key); err != nil {
			return err
		}

		data, err := r.client.Get(ctx, r.FullKey(key)).Bytes()
		if err == redis.Nil {
			r.RecordMiss("get", 1)
			return base.ErrCacheMiss
		}
		if err != nil {
			return base.NewError("redis get", err, key)
		}

		v, err := r.serializer.Decode(data)
		if err != nil {
			return base.NewError("deserialization", err, key)
		}

		val = v
		r.RecordHit("get", 1)
		return nil
	})

	return val, err
}

// Set stores a value in Redis
func (r *redisCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	return r.execute(ctx, "set", 1, func() error {
		if err := r.ValidateKey(key); err != nil {
			return err
		}

		data, err := r.serializer.Encode(value)
		if err != nil {
			return base.NewError("serialization", err, key)
		}

		effectiveTTL := r.TTL(ttl)
		if err := r.client.Set(ctx, r.FullKey(key), data, effectiveTTL).Err(); err != nil {
			return base.NewError("redis set", err, key)
		}
		return nil
	})
}

// Delete removes keys from Redis
func (r *redisCache[T]) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	return r.execute(ctx, "delete", len(keys), func() error {
		fullKeys := make([]string, len(keys))
		for i, k := range keys {
			fullKeys[i] = r.FullKey(k)
		}

		if err := r.client.Del(ctx, fullKeys...).Err(); err != nil {
			return base.NewError("redis delete", err, "")
		}
		return nil
	})
}

// Exists checks if a key exists in Redis
func (r *redisCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	var exists bool

	err := r.execute(ctx, "exists", 1, func() error {
		if err := r.ValidateKey(key); err != nil {
			return err
		}

		n, err := r.client.Exists(ctx, r.FullKey(key)).Result()
		if err != nil {
			return base.NewError("redis exists", err, key)
		}
		exists = n > 0
		return nil
	})

	return exists, err
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
	if err := r.client.Ping(ctx).Err(); err != nil {
		return base.NewError("redis ping", err, "")
	}
	return nil
}

// scanKeys performs a scan operation with the given pattern
func (r *redisCache[T]) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

// Clear removes all keys with the prefix
func (r *redisCache[T]) Clear(ctx context.Context) error {
	return r.execute(ctx, "clear", 1, func() error {
		keys, err := r.scanKeys(ctx, r.Cfg.Prefix+"*")
		if err != nil {
			return base.NewError("redis scan", err, "")
		}

		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return base.NewError("redis delete", err, "")
			}
		}
		return nil
	})
}

// Len returns the number of items with the prefix
func (r *redisCache[T]) Len(ctx context.Context) (int, error) {
	if err := r.CheckContext(ctx); err != nil {
		return 0, err
	}

	keys, err := r.scanKeys(ctx, r.Cfg.Prefix+"*")
	if err != nil {
		r.RecordError("len")
		return 0, base.NewError("redis scan", err, "")
	}
	return len(keys), nil
}

// DeleteByPrefix removes all keys with the given prefix
func (r *redisCache[T]) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	var total int64

	err := r.execute(ctx, "delete_by_prefix", 1, func() error {
		pattern := r.FullKey(prefix) + "*"
		var cursor uint64

		for {
			keys, next, err := r.client.Scan(ctx, cursor, pattern, 1000).Result()
			if err != nil {
				return base.NewError("redis scan", err, prefix)
			}

			if len(keys) > 0 {
				deleted, err := r.client.Del(ctx, keys...).Result()
				if err != nil {
					return base.NewError("redis delete", err, prefix)
				}
				total += deleted
			}

			cursor = next
			if cursor == 0 {
				break
			}
		}
		return nil
	})

	return total, err
}

// Stats returns cache statistics
func (r *redisCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	items, _ := r.Len(ctx)

	snapshot := r.Metrics.Snapshot()
	var hits, misses int64

	for _, stat := range snapshot {
		hits += stat.Hits
		misses += stat.Misses
	}

	return metrics.CacheStats{
		Backend:         "redis",
		Items:           int64(items),
		Hits:            hits,
		Misses:          misses,
		HitRate:         metrics.CalculateHitRate(hits, misses),
		Uptime:          r.Uptime(),
		RefreshTTLOnHit: r.Cfg.RefreshTTLOnHit,
	}
}
