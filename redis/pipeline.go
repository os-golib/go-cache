package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/os-golib/go-cache/config"
)

// GetManyPipeline performs pipelined GET operations for multiple keys
func (r *redisCache[T]) GetManyPipeline(ctx context.Context, keys []string) (map[string]T, error) {
	start := time.Now()
	defer func() {
		r.Cache.RecordOperation("get_many_pipeline", time.Since(start), len(keys))
	}()

	if len(keys) == 0 {
		return map[string]T{}, nil
	}

	fullKeys := make([]string, len(keys))
	for in, k := range keys {
		fullKeys[in] = r.FullKey(k)
	}

	pipe := r.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(fullKeys))
	for in, fk := range fullKeys {
		cmds[in] = pipe.Get(ctx, fk)
	}

	_, execErr := pipe.Exec(ctx)

	result := make(map[string]T, len(keys))
	var hitCount, missCount int64

	for in, cmd := range cmds {
		origKey := keys[in]
		data, err := cmd.Bytes()

		switch {
		case err == nil:
			v, derr := r.serializer.Deserialize(data)
			if derr == nil {
				result[origKey] = v
				hitCount++
				continue
			}
			// Deserialization error → treat as miss
			missCount++

		case err != redis.Nil:
			// Real redis error
			if execErr == nil {
				execErr = err
			}
			missCount++

		default:
			// err == redis.Nil → key not found
			missCount++
		}
	}

	r.Cache.RecordHit("get_many_pipeline", hitCount)
	r.Cache.RecordMiss("get_many_pipeline", missCount)

	if execErr != nil && execErr != redis.Nil {
		r.Cache.RecordError("get_many_pipeline")
		return result, config.NewError("redis pipeline get", execErr, "")
	}

	return result, nil
}

// SetManyPipeline performs pipelined SET operations for multiple items
func (r *redisCache[T]) SetManyPipeline(ctx context.Context, items map[string]T, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		r.Cache.RecordOperation("set_many_pipeline", time.Since(start), len(items))
	}()

	if len(items) == 0 {
		return nil
	}

	effectiveTTL := r.TTL(ttl)
	pipe := r.client.Pipeline()

	for k, v := range items {
		data, err := r.serializer.Serialize(v)
		if err != nil {
			return config.NewError("serialization", err, k)
		}
		fk := r.FullKey(k)
		if effectiveTTL > 0 {
			pipe.Set(ctx, fk, data, effectiveTTL)
		} else {
			pipe.Set(ctx, fk, data, 0)
		}
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		r.Cache.RecordError("set_many_pipeline")
		return config.NewError("redis pipeline set", err, "")
	}
	return nil
}

// GetMany is an alias for GetManyPipeline for interface compatibility
func (r *redisCache[T]) GetMany(ctx context.Context, keys []string) (map[string]T, error) {
	return r.GetManyPipeline(ctx, keys)
}

// SetMany is an alias for SetManyPipeline for interface compatibility
func (r *redisCache[T]) SetMany(ctx context.Context, items map[string]T, ttl time.Duration) error {
	return r.SetManyPipeline(ctx, items, ttl)
}
