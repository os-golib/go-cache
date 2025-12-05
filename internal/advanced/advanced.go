package advanced

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/base"
	"github.com/os-golib/go-cache/internal/interfaces"
	"github.com/os-golib/go-cache/internal/metrics"
)

type advancedCache[T any] struct {
	base    interfaces.Cache[T]
	cfg     config.Config
	metrics *base.Base
}

func NewAdvancedCache[T any](cache interfaces.Cache[T], cfg config.Config) interfaces.AdvancedCache[T] {
	return &advancedCache[T]{
		base:    cache,
		cfg:     cfg,
		metrics: base.NewBase(cfg),
	}
}

// Unified operation wrapper
func (a *advancedCache[T]) withMetrics(op string, itemCount int, fn func() error) error {
	start := time.Now()
	defer func() {
		a.metrics.RecordOperation(op, time.Since(start), itemCount)
	}()

	err := fn()
	if err != nil {
		a.metrics.RecordError(op)
	}
	return err
}

func (a *advancedCache[T]) Get(ctx context.Context, key string) (T, error) {
	var val T
	var getErr error

	err := a.withMetrics("get", 1, func() error {
		var err error
		val, err = a.base.Get(ctx, key)
		getErr = err

		if err == nil {
			a.metrics.RecordHit("get", 1)

			if a.cfg.RefreshTTLOnHit {
				go a.base.Set(context.Background(), key, val, 0)
			}
		} else if errors.Is(err, base.ErrCacheMiss) {
			a.metrics.RecordMiss("get", 1)
		}

		return err
	})

	if err != nil && !errors.Is(err, base.ErrCacheMiss) {
		return val, err
	}

	return val, getErr
}

func (a *advancedCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	return a.withMetrics("set", 1, func() error {
		return a.base.Set(ctx, key, value, ttl)
	})
}

func (a *advancedCache[T]) Delete(ctx context.Context, keys ...string) error {
	return a.withMetrics("delete", len(keys), func() error {
		return a.base.Delete(ctx, keys...)
	})
}

func (a *advancedCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := a.withMetrics("exists", 1, func() error {
		var err error
		exists, err = a.base.Exists(ctx, key)
		return err
	})
	return exists, err
}

func (a *advancedCache[T]) Close() error                   { return a.base.Close() }
func (a *advancedCache[T]) Ping(ctx context.Context) error { return a.base.Ping(ctx) }

func (a *advancedCache[T]) Clear(ctx context.Context) error {
	return a.withMetrics("clear", 1, func() error {
		return a.base.Clear(ctx)
	})
}

func (a *advancedCache[T]) Len(ctx context.Context) (int, error) {
	var length int
	err := a.withMetrics("length", 1, func() error {
		var err error
		length, err = a.base.Len(ctx)
		return err
	})
	return length, err
}

func (a *advancedCache[T]) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	del, ok := a.base.(interfaces.PrefixDeleter)
	if !ok {
		return 0, fmt.Errorf("DeleteByPrefix not supported by backend")
	}

	var count int64
	err := a.withMetrics("delete_by_prefix", 1, func() error {
		var err error
		count, err = del.DeleteByPrefix(ctx, prefix)
		return err
	})
	return count, err
}

func (a *advancedCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	baseStats := metrics.CacheStats{
		Backend:         "advanced",
		RefreshTTLOnHit: a.cfg.RefreshTTLOnHit,
	}

	if provider, ok := a.base.(interfaces.StatProvider); ok {
		baseStats = provider.Stats(ctx)
	}

	for _, op := range a.metrics.Metrics.Snapshot() {
		baseStats.Hits += op.Hits
		baseStats.Misses += op.Misses
	}

	baseStats.HitRate = metrics.CalculateHitRate(baseStats.Hits, baseStats.Misses)
	return baseStats
}

func (a *advancedCache[T]) Metrics() *metrics.Collector {
	return a.metrics.Metrics
}

func (a *advancedCache[T]) GetOrSet(ctx context.Context, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	return a.getOrSetImpl(ctx, key, ttl, fn, false)
}

func (a *advancedCache[T]) GetOrSetLocked(ctx context.Context, key string, ttl time.Duration, fn func() (T, error)) (T, error) {
	return a.getOrSetImpl(ctx, key, ttl, fn, true)
}

func (a *advancedCache[T]) getOrSetImpl(ctx context.Context, key string, ttl time.Duration, fn func() (T, error), locked bool) (T, error) {
	op := "get_or_set"
	if locked {
		op = "get_or_set_locked"
	}

	start := time.Now()
	defer func() { a.metrics.RecordOperation(op, time.Since(start), 1) }()

	var zero T

	val, err := a.Get(ctx, key)
	if err == nil {
		return val, nil
	}
	if !errors.Is(err, base.ErrCacheMiss) {
		return zero, err
	}

	if locked {
		if locker, ok := a.base.(interfaces.DistributedLocker); ok {
			lockKey := "lock:" + key
			acquired, _ := locker.TryLock(ctx, lockKey, 30*time.Second)
			if acquired {
				defer locker.Unlock(ctx, lockKey)
			}
		}
	}

	val, err = fn()
	if err != nil {
		return zero, err
	}

	_ = a.Set(ctx, key, val, ttl)
	return val, nil
}

// Concurrent execution helper
func (a *advancedCache[T]) concurrentExecute(
	ctx context.Context,
	items map[string]func() error,
	maxConcurrency int,
) error {
	var firstErr atomic.Value
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)

	for key, fn := range items {
		wg.Add(1)
		sem <- struct{}{}

		go func(k string, f func() error) {
			defer func() {
				<-sem
				wg.Done()
			}()

			if err := f(); err != nil && firstErr.Load() == nil {
				firstErr.Store(err)
			}
		}(key, fn)
	}

	wg.Wait()

	if err, ok := firstErr.Load().(error); ok {
		return err
	}
	return nil
}
