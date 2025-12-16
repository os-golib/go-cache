package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/os-golib/go-cache/internal/base"
)

/* ------------------ Pipeline Result ------------------ */

// type pipelineResult[T any] struct {
// 	values map[string]T
// 	hits   int64
// 	misses int64
// }

/* ------------------ GET MANY (Pipeline) ------------------ */

func (r *redisCache[T]) GetManyPipeline(
	ctx context.Context,
	keys []string,
) (map[string]T, error) {
	if len(keys) == 0 {
		return map[string]T{}, nil
	}

	if err := r.base.CheckContext(ctx); err != nil {
		return nil, err
	}

	result, err := r.executePipelineGet(ctx, keys)
	if err != nil {
		return nil, base.WrapError(base.OpGet, err, "")
	}

	return result, nil
}

/* ------------------ SET MANY (Pipeline) ------------------ */

func (r *redisCache[T]) SetManyPipeline(
	ctx context.Context,
	items map[string]T,
	ttl time.Duration,
) error {
	if len(items) == 0 {
		return nil
	}

	if err := r.base.CheckContext(ctx); err != nil {
		return err
	}

	ttl = r.base.ResolveTTL(ttl)
	pipe := r.client.Pipeline()

	for k, v := range items {
		if err := r.base.ValidateKey(k); err != nil {
			return err
		}

		data, err := r.serializer.Encode(v)
		if err != nil {
			return base.WrapError(base.OpSet, base.ErrSerialize, k)
		}

		pipe.Set(ctx, r.base.FullKey(k), data, ttl)
	}

	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return base.WrapError(base.OpSet, err, "")
	}

	return nil
}

/* ------------------ Internal: Pipeline GET ------------------ */

func (r *redisCache[T]) executePipelineGet(
	ctx context.Context,
	keys []string,
) (map[string]T, error) {
	pipe := r.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))

	for i, k := range keys {
		if err := r.base.ValidateKey(k); err != nil {
			return nil, err
		}
		cmds[i] = pipe.Get(ctx, r.base.FullKey(k))
	}

	_, execErr := pipe.Exec(ctx)
	if execErr != nil && execErr != redis.Nil {
		return nil, execErr
	}

	result := make(map[string]T, len(keys))

	for i, cmd := range cmds {
		data, err := cmd.Bytes()
		if err == redis.Nil {
			continue
		}
		if err != nil {
			return nil, err
		}

		val, derr := r.serializer.Decode(data)
		if derr != nil {
			return nil, derr
		}
		result[keys[i]] = val
	}

	return result, nil
}
