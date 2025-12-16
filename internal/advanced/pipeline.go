package advanced

import (
	"context"
	"sync"
	"time"

	"github.com/os-golib/go-cache/internal/interfaces"
)

/* ------------------ Pipeline: GET ------------------ */

func (a *advancedCache[T]) GetManyPipeline(
	ctx context.Context,
	keys []string,
) (map[string]T, error) {
	// Fast path: backend supports pipeline
	if pg, ok := a.cache.(interfaces.PipelineGetter[T]); ok {
		return pg.GetManyPipeline(ctx, keys)
	}

	result := make(map[string]T, len(keys))
	var mu sync.Mutex

	tasks := make([]func(context.Context) error, 0, len(keys))
	for _, key := range keys {
		k := key
		tasks = append(tasks, func(ctx context.Context) error {
			val, err := a.Get(ctx, k)
			if err != nil {
				return err
			}

			mu.Lock()
			result[k] = val
			mu.Unlock()
			return nil
		})
	}

	err := a.withMetrics("get_many_pipeline", len(keys), func() error {
		return a.concurrentExecute(ctx, tasks, 10)
	})

	return result, err
}

/* ------------------ Pipeline: SET ------------------ */

func (a *advancedCache[T]) SetManyPipeline(
	ctx context.Context,
	items map[string]T,
	ttl time.Duration,
) error {
	// Fast path: backend supports pipeline
	if ps, ok := a.cache.(interfaces.PipelineSetter[T]); ok {
		return ps.SetManyPipeline(ctx, items, ttl)
	}

	tasks := make([]func(context.Context) error, 0, len(items))
	for key, val := range items {
		k, v := key, val
		tasks = append(tasks, func(ctx context.Context) error {
			return a.Set(ctx, k, v, ttl)
		})
	}

	return a.withMetrics("set_many_pipeline", len(items), func() error {
		return a.concurrentExecute(ctx, tasks, 10)
	})
}

/* ------------------ Concurrent Helper ------------------ */

func (a *advancedCache[T]) concurrentExecute(
	ctx context.Context,
	tasks []func(context.Context) error,
	maxConcurrency int,
) error {
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var firstErr error
	var once sync.Once

	for _, task := range tasks {
		wg.Add(1)
		go func(fn func(context.Context) error) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				once.Do(func() { firstErr = ctx.Err() })
				return
			}
			defer func() { <-sem }()

			if err := fn(ctx); err != nil {
				once.Do(func() { firstErr = err })
			}
		}(task)
	}

	wg.Wait()
	return firstErr
}
