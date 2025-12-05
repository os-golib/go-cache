package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/os-golib/go-cache/internal/base"
)

// pipelineResult encapsulates pipeline execution results
type pipelineResult struct {
	hits   int64
	misses int64
	err    error
}

// GetManyPipeline performs pipelined GET operations
func (r *redisCache[T]) GetManyPipeline(ctx context.Context, keys []string) (map[string]T, error) {
	if len(keys) == 0 {
		return map[string]T{}, nil
	}

	var result map[string]T
	var pipeRes pipelineResult

	err := r.execute(ctx, "get_many_pipeline", len(keys), func() error {
		result, pipeRes = r.executePipelineGet(ctx, keys)

		r.RecordHit("get_many_pipeline", pipeRes.hits)
		r.RecordMiss("get_many_pipeline", pipeRes.misses)

		if pipeRes.err != nil && pipeRes.err != redis.Nil {
			return base.NewError("redis pipeline get", pipeRes.err, "")
		}
		return nil
	})

	return result, err
}

// executePipelineGet performs the actual pipeline GET operations
func (r *redisCache[T]) executePipelineGet(ctx context.Context, keys []string) (map[string]T, pipelineResult) {
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = r.FullKey(k)
	}

	pipe := r.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(fullKeys))

	for i, fk := range fullKeys {
		cmds[i] = pipe.Get(ctx, fk)
	}

	_, execErr := pipe.Exec(ctx)

	result := make(map[string]T, len(keys))
	var res pipelineResult

	for i, cmd := range cmds {
		origKey := keys[i]
		data, err := cmd.Bytes()

		if err == nil {
			if v, derr := r.serializer.Decode(data); derr == nil {
				result[origKey] = v
				res.hits++
			} else {
				res.misses++
			}
		} else if err == redis.Nil {
			res.misses++
		} else {
			if res.err == nil {
				res.err = err
			}
			res.misses++
		}
	}

	if execErr != nil && execErr != redis.Nil {
		res.err = execErr
	}

	return result, res
}

// SetManyPipeline performs pipelined SET operations
func (r *redisCache[T]) SetManyPipeline(ctx context.Context, items map[string]T, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	return r.execute(ctx, "set_many_pipeline", len(items), func() error {
		effectiveTTL := r.TTL(ttl)
		pipe := r.client.Pipeline()

		for k, v := range items {
			data, err := r.serializer.Encode(v)
			if err != nil {
				return base.NewError("serialization", err, k)
			}
			pipe.Set(ctx, r.FullKey(k), data, effectiveTTL)
		}

		if _, err := pipe.Exec(ctx); err != nil {
			return base.NewError("redis pipeline set", err, "")
		}
		return nil
	})
}

// GetMany is an alias for GetManyPipeline
func (r *redisCache[T]) GetMany(ctx context.Context, keys []string) (map[string]T, error) {
	return r.GetManyPipeline(ctx, keys)
}

// SetMany is an alias for SetManyPipeline
func (r *redisCache[T]) SetMany(ctx context.Context, items map[string]T, ttl time.Duration) error {
	return r.SetManyPipeline(ctx, items, ttl)
}
