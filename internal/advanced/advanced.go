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

// advancedCache wraps a base cache with advanced features
type advancedCache[T any] struct {
	base    interfaces.Cache[T]
	cfg     config.Config
	metrics *base.Cache
}

// NewAdvancedCache creates a new advanced cache wrapper
func NewAdvancedCache[T any](cache interfaces.Cache[T], cfg config.Config) interfaces.AdvancedCache[T] {
	return &advancedCache[T]{
		base:    cache,
		cfg:     cfg,
		metrics: base.NewCache(),
	}
}

// ========== BASIC PASSTHROUGH WITH METRICS ==========

func (a *advancedCache[T]) Get(ctx context.Context, key string) (T, error) {
	start := time.Now()
	defer func() { a.metrics.RecordOperation("get", time.Since(start), 1) }()

	val, err := a.base.Get(ctx, key)
	if err != nil {
		if errors.Is(err, config.ErrCacheMiss) {
			a.metrics.RecordMiss("get", 1)
		} else {
			a.metrics.RecordError("get")
		}
		return val, err
	}

	a.metrics.RecordHit("get", 1)

	// Refresh TTL on hit if configured
	if a.cfg.RefreshTTLOnHit {
		go func(ctx context.Context) {
			_ = a.base.Set(ctx, key, val, 0)
		}(ctx)
	}

	return val, nil
}

func (a *advancedCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	start := time.Now()
	defer func() { a.metrics.RecordOperation("set", time.Since(start), 1) }()

	err := a.base.Set(ctx, key, value, ttl)
	if err != nil {
		a.metrics.RecordError("set")
	}
	return err
}

func (a *advancedCache[T]) Delete(ctx context.Context, keys ...string) error {
	start := time.Now()
	defer func() { a.metrics.RecordOperation("delete", time.Since(start), len(keys)) }()

	err := a.base.Delete(ctx, keys...)
	if err != nil {
		a.metrics.RecordError("delete")
	}
	return err
}

func (a *advancedCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	start := time.Now()
	defer func() { a.metrics.RecordOperation("exists", time.Since(start), 1) }()

	exists, err := a.base.Exists(ctx, key)
	if err != nil {
		a.metrics.RecordError("exists")
	}
	return exists, err
}

func (a *advancedCache[T]) Close() error {
	return a.base.Close()
}

func (a *advancedCache[T]) Ping(ctx context.Context) error {
	return a.base.Ping(ctx)
}

func (a *advancedCache[T]) Clear(ctx context.Context) error {
	start := time.Now()
	defer func() { a.metrics.RecordOperation("clear", time.Since(start), 1) }()

	err := a.base.Clear(ctx)
	if err != nil {
		a.metrics.RecordError("clear")
	}
	return err
}

func (a *advancedCache[T]) Len(ctx context.Context) (int, error) {
	start := time.Now()
	defer func() { a.metrics.RecordOperation("length", time.Since(start), 1) }()

	length, err := a.base.Len(ctx)
	if err != nil {
		a.metrics.RecordError("length")
	}
	return length, err
}

// ========== DELETE BY PREFIX ==========

func (a *advancedCache[T]) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	start := time.Now()
	defer func() { a.metrics.RecordOperation("delete_by_prefix", time.Since(start), 1) }()

	if del, ok := a.base.(interfaces.PrefixDeleter); ok {
		return del.DeleteByPrefix(ctx, prefix)
	}
	return 0, fmt.Errorf("DeleteByPrefix not supported by backend")
}

// ========== STATS & METRICS ==========

func (a *advancedCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	// Get base stats if available
	var baseStats metrics.CacheStats
	if provider, ok := a.base.(interfaces.StatProvider); ok {
		baseStats = provider.Stats(ctx)
	} else {
		baseStats = metrics.CacheStats{
			Backend:         "advanced",
			RefreshTTLOnHit: a.cfg.RefreshTTLOnHit,
		}
	}

	// Merge with advanced metrics
	snapshot := a.metrics.Metrics().Snapshot()
	for _, op := range snapshot {
		baseStats.Hits += op.Hits
		baseStats.Misses += op.Misses
	}

	// Calculate hit rate
	total := baseStats.Hits + baseStats.Misses
	if total > 0 {
		baseStats.HitRate = float64(baseStats.Hits) / float64(total)
	}

	return baseStats
}

func (a *advancedCache[T]) Metrics() *metrics.Collector {
	return a.metrics.Metrics()
}

// ========== ADVANCED FEATURES ==========

func (a *advancedCache[T]) GetOrSet(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func() (T, error),
) (T, error) {
	const op = "get_or_set"
	start := time.Now()
	defer func() { a.metrics.RecordOperation(op, time.Since(start), 1) }()

	var zero T

	defer func() {
		if r := recover(); r != nil {
			a.metrics.RecordError(op)
		}
	}()

	val, err := a.Get(ctx, key)
	if err == nil {
		return val, nil
	}
	if !errors.Is(err, config.ErrCacheMiss) {
		return zero, err
	}

	val, err = fn()
	if err != nil {
		return zero, err
	}

	_ = a.Set(ctx, key, val, ttl)
	return val, nil
}

func (a *advancedCache[T]) GetOrSetLocked(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func() (T, error),
) (T, error) {
	const op = "get_or_set_locked"
	start := time.Now()
	defer func() { a.metrics.RecordOperation(op, time.Since(start), 1) }()

	lockKey := "lock:" + key

	// Try to acquire distributed lock if supported
	if locker, ok := a.base.(interfaces.DistributedLocker); ok {
		acquired, err := locker.TryLock(ctx, lockKey, 30*time.Second)
		if err != nil {
			return a.GetOrSet(ctx, key, ttl, fn)
		}
		if !acquired {
			// If lock not acquired, fall back to regular GetOrSet
			return a.GetOrSet(ctx, key, ttl, fn)
		}
		defer func() {
			_ = locker.Unlock(ctx, lockKey)
		}()
	}

	return a.GetOrSet(ctx, key, ttl, fn)
}

// ========== BULK OPERATIONS ==========

func (a *advancedCache[T]) GetMany(ctx context.Context, keys []string) (map[string]T, error) {
	start := time.Now()
	defer func() { a.metrics.RecordOperation("get_many", time.Since(start), len(keys)) }()

	// Use pipeline if available
	if pg, ok := a.base.(interfaces.PipelineGetter[T]); ok {
		return pg.GetManyPipeline(ctx, keys)
	}

	// Fallback to concurrent individual gets
	return a.fallbackGetMany(ctx, keys)
}

func (a *advancedCache[T]) SetMany(ctx context.Context, items map[string]T, ttl time.Duration) error {
	start := time.Now()
	defer func() { a.metrics.RecordOperation("set_many", time.Since(start), len(items)) }()

	// Use pipeline if available
	if ps, ok := a.base.(interfaces.PipelineSetter[T]); ok {
		return ps.SetManyPipeline(ctx, items, ttl)
	}

	// Fallback to individual sets
	return a.fallbackSetMany(ctx, items, ttl)
}

func (a *advancedCache[T]) GetManyPipeline(ctx context.Context, keys []string) (map[string]T, error) {
	return a.GetMany(ctx, keys)
}

func (a *advancedCache[T]) SetManyPipeline(ctx context.Context, items map[string]T, ttl time.Duration) error {
	return a.SetMany(ctx, items, ttl)
}

// ========== FALLBACK IMPLEMENTATIONS ==========

func (a *advancedCache[T]) fallbackGetMany(ctx context.Context, keys []string) (map[string]T, error) {
	result := make(map[string]T, len(keys))
	var mu sync.Mutex
	var wg sync.WaitGroup

	var firstErr atomic.Value

	// Limit concurrency
	sem := make(chan struct{}, 10)

	for _, key := range keys {
		wg.Add(1)
		sem <- struct{}{}

		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()

			val, err := a.Get(ctx, key)
			if err == nil {
				mu.Lock()
				result[key] = val
				mu.Unlock()
			} else if !errors.Is(err, config.ErrCacheMiss) {
				firstErr.Store(err)
			}
		}()
	}

	wg.Wait()

	if err, ok := firstErr.Load().(error); ok && err != nil {
		a.metrics.RecordError("get_many")
		return result, err
	}
	return result, nil
}

func (a *advancedCache[T]) fallbackSetMany(ctx context.Context, items map[string]T, ttl time.Duration) error {
	var firstErr atomic.Value
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Limit concurrency
	sem := make(chan struct{}, 10)

	for k, v := range items {
		key, val := k, v
		wg.Add(1)
		sem <- struct{}{}

		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()

			err := a.Set(ctx, key, val, ttl)
			if err != nil {
				mu.Lock()
				if firstErr.Load() == nil {
					firstErr.Store(err)
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if err, ok := firstErr.Load().(error); ok && err != nil {
		a.metrics.RecordError("set_many")
		return err
	}
	return nil
}
