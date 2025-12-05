package advanced

import (
	"context"
	"sync"
	"time"

	"github.com/os-golib/go-cache/internal/interfaces"
)

func (a *advancedCache[T]) GetMany(ctx context.Context, keys []string) (map[string]T, error) {
	if pg, ok := a.base.(interfaces.PipelineGetter[T]); ok {
		return pg.GetManyPipeline(ctx, keys)
	}

	result := make(map[string]T)
	var mu sync.Mutex

	fns := make(map[string]func() error)
	for _, key := range keys {
		k := key
		fns[k] = func() error {
			val, err := a.Get(ctx, k)
			if err == nil {
				mu.Lock()
				result[k] = val
				mu.Unlock()
			}
			return nil
		}
	}

	a.concurrentExecute(ctx, fns, 10)
	return result, nil
}

func (a *advancedCache[T]) SetMany(ctx context.Context, items map[string]T, ttl time.Duration) error {
	if ps, ok := a.base.(interfaces.PipelineSetter[T]); ok {
		return ps.SetManyPipeline(ctx, items, ttl)
	}

	fns := make(map[string]func() error)
	for key, val := range items {
		k, v := key, val
		fns[k] = func() error {
			return a.Set(ctx, k, v, ttl)
		}
	}

	return a.concurrentExecute(ctx, fns, 10)
}

func (a *advancedCache[T]) GetManyPipeline(ctx context.Context, keys []string) (map[string]T, error) {
	return a.GetMany(ctx, keys)
}

func (a *advancedCache[T]) SetManyPipeline(ctx context.Context, items map[string]T, ttl time.Duration) error {
	return a.SetMany(ctx, items, ttl)
}
