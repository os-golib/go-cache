package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/base"
	"github.com/os-golib/go-cache/internal/metrics"
)

/* ------------------ Types ------------------ */

type redisCache[T any] struct {
	base       *base.Base
	client     *redis.Client
	serializer base.Serializer[T]
}

/* ------------------ Constructor ------------------ */

func NewRedisCache[T any](cfg config.Config) (*redisCache[T], error) {
	return NewRedisContext[T](context.Background(), cfg)
}

func NewRedisContext[T any](ctx context.Context, cfg config.Config) (*redisCache[T], error) {
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, base.WrapError(base.OpSet, err, "")
	}

	// Apply config
	opt.PoolSize = cfg.PoolSize
	opt.MinIdleConns = cfg.MinIdleConn
	opt.MaxRetries = cfg.MaxRetries
	opt.DialTimeout = cfg.DialTimeout
	opt.ReadTimeout = cfg.ReadTimeout
	opt.WriteTimeout = cfg.WriteTimeout

	client := redis.NewClient(opt)

	// Ensure timeout
	if _, ok := ctx.Deadline(); !ok {
		timeout := cfg.ConnTimeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, base.WrapError(base.OpPing, base.ErrConnection, "")
	}

	return &redisCache[T]{
		base:       base.NewBase(cfg),
		client:     client,
		serializer: &base.JsonSerializer[T]{},
	}, nil
}

/* ------------------ Cache API ------------------ */

func (r *redisCache[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T

	if err := r.base.ValidateKey(key); err != nil {
		return zero, err
	}
	if err := r.base.CheckContext(ctx); err != nil {
		return zero, err
	}

	data, err := r.client.Get(ctx, r.base.FullKey(key)).Bytes()
	if err == redis.Nil {
		return zero, base.WrapError(base.OpGet, base.ErrCacheMiss, key)
	}
	if err != nil {
		return zero, base.WrapError(base.OpGet, err, key)
	}

	val, err := r.serializer.Decode(data)
	if err != nil {
		return zero, base.WrapError(base.OpGet, base.ErrDeserialize, key)
	}

	return val, nil
}

func (r *redisCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	if err := r.base.ValidateKey(key); err != nil {
		return err
	}
	if err := r.base.CheckContext(ctx); err != nil {
		return err
	}

	data, err := r.serializer.Encode(value)
	if err != nil {
		return base.WrapError(base.OpSet, base.ErrSerialize, key)
	}

	ttl = r.base.ResolveTTL(ttl)
	if err := r.client.Set(ctx, r.base.FullKey(key), data, ttl).Err(); err != nil {
		return base.WrapError(base.OpSet, err, key)
	}
	return nil
}

func (r *redisCache[T]) Delete(ctx context.Context, keys ...string) error {
	if err := r.base.CheckContext(ctx); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}

	full := make([]string, len(keys))
	for i, k := range keys {
		full[i] = r.base.FullKey(k)
	}

	if err := r.client.Del(ctx, full...).Err(); err != nil {
		return base.WrapError(base.OpDelete, err, "")
	}
	return nil
}

func (r *redisCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	if err := r.base.ValidateKey(key); err != nil {
		return false, err
	}

	n, err := r.client.Exists(ctx, r.base.FullKey(key)).Result()
	if err != nil {
		return false, base.WrapError(base.OpExists, err, key)
	}
	return n > 0, nil
}

func (r *redisCache[T]) Clear(ctx context.Context) error {
	pattern := r.base.FullKey("") + "*"
	var cursor uint64

	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 1000).Result()
		if err != nil {
			return base.WrapError(base.OpClear, err, "")
		}
		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return base.WrapError(base.OpClear, err, "")
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (r *redisCache[T]) Len(ctx context.Context) (int, error) {
	if err := r.base.CheckContext(ctx); err != nil {
		return 0, err
	}

	pattern := r.base.FullKey("") + "*"
	var cursor uint64
	var total int

	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 1000).Result()
		if err != nil {
			return 0, base.WrapError(base.OpLen, err, "")
		}
		total += len(keys)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return total, nil
}

func (r *redisCache[T]) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	pattern := r.base.FullKey(prefix) + "*"
	var cursor uint64
	var total int64

	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 1000).Result()
		if err != nil {
			return total, base.WrapError(base.OpDeleteByPrefix, err, prefix)
		}
		if len(keys) > 0 {
			n, err := r.client.Del(ctx, keys...).Result()
			if err != nil {
				return total, base.WrapError(base.OpDeleteByPrefix, err, prefix)
			}
			total += n
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return total, nil
}

func (r *redisCache[T]) Ping(ctx context.Context) error {
	if err := r.base.CheckContext(ctx); err != nil {
		return err
	}
	if err := r.client.Ping(ctx).Err(); err != nil {
		return base.WrapError(base.OpPing, err, "")
	}
	return nil
}

func (r *redisCache[T]) Close() error {
	return r.client.Close()
}

/* ------------------ Stats ------------------ */

func (r *redisCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	items, _ := r.Len(ctx)

	snap := r.base.Metrics().Snapshot()
	var hits, misses int64
	for _, s := range snap {
		hits += s.Hits
		misses += s.Misses
	}

	return metrics.CacheStats{
		Backend: "redis",
		Items:   int64(items),
		Hits:    hits,
		Misses:  misses,
		HitRate: metrics.CalculateHitRate(hits, misses),
		Uptime:  r.base.Uptime(),
	}
}
