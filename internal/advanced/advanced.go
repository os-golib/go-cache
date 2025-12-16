package advanced

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/base"
	"github.com/os-golib/go-cache/internal/interfaces"
	"github.com/os-golib/go-cache/internal/metrics"
)

/* ------------------ Types ------------------ */

type advancedCache[T any] struct {
	cache interfaces.Cache[T]
	base  *base.Base
	cfg   config.Config
}

/* ------------------ Constructor ------------------ */

func NewAdvancedCache[T any](
	cache interfaces.Cache[T],
	cfg config.Config,
) interfaces.AdvancedCache[T] {
	return &advancedCache[T]{
		cache: cache,
		cfg:   cfg,
		base:  base.NewBase(cfg),
	}
}

/* ------------------ Helpers ------------------ */

func (a *advancedCache[T]) withMetrics(
	op string,
	items int,
	fn func() error,
) error {
	start := time.Now()
	err := fn()

	a.base.RecordOperation(op, time.Since(start), items)
	if err != nil {
		a.base.RecordError(op)
	}
	return err
}

/* ------------------ Core Operations ------------------ */

func (a *advancedCache[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T

	if err := a.base.ValidateKey(key); err != nil {
		return zero, err
	}
	if err := a.base.CheckContext(ctx); err != nil {
		return zero, err
	}

	var val T
	err := a.withMetrics("get", 1, func() error {
		v, err := a.cache.Get(ctx, key)
		if err != nil {
			if errors.Is(err, base.ErrCacheMiss) {
				a.base.RecordMiss("get", 1)
			}
			return err
		}

		a.base.RecordHit("get", 1)
		val = v
		return nil
	})

	return val, err
}

func (a *advancedCache[T]) Set(
	ctx context.Context,
	key string,
	value T,
	ttl time.Duration,
) error {
	if err := a.base.ValidateKey(key); err != nil {
		return err
	}

	ttl = a.base.ResolveTTL(ttl)

	return a.withMetrics("set", 1, func() error {
		return a.cache.Set(ctx, key, value, ttl)
	})
}

func (a *advancedCache[T]) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	return a.withMetrics("delete", len(keys), func() error {
		return a.cache.Delete(ctx, keys...)
	})
}

func (a *advancedCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := a.withMetrics("exists", 1, func() error {
		v, err := a.cache.Exists(ctx, key)
		exists = v
		return err
	})
	return exists, err
}

/* ------------------ Utility ------------------ */

func (a *advancedCache[T]) Clear(ctx context.Context) error {
	return a.withMetrics("clear", 1, func() error {
		return a.cache.Clear(ctx)
	})
}

func (a *advancedCache[T]) Len(ctx context.Context) (int, error) {
	var n int
	err := a.withMetrics("len", 1, func() error {
		v, err := a.cache.Len(ctx)
		n = v
		return err
	})
	return n, err
}

func (a *advancedCache[T]) Close() error {
	return a.cache.Close()
}

func (a *advancedCache[T]) Ping(ctx context.Context) error {
	return a.cache.Ping(ctx)
}

/* ------------------ Prefix Ops ------------------ */

func (a *advancedCache[T]) DeleteByPrefix(
	ctx context.Context,
	prefix string,
) (int64, error) {
	deleter, ok := a.cache.(interfaces.PrefixDeleter)
	if !ok {
		return 0, fmt.Errorf("DeleteByPrefix not supported")
	}

	var count int64
	err := a.withMetrics("delete_by_prefix", 1, func() error {
		v, err := deleter.DeleteByPrefix(ctx, prefix)
		count = v
		return err
	})
	return count, err
}

/* ------------------ Stats & Metrics ------------------ */

func (a *advancedCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	stats := metrics.CacheStats{
		Backend:         "advanced",
		RefreshTTLOnHit: a.cfg.RefreshTTLOnHit,
	}

	if sp, ok := a.cache.(interfaces.StatProvider); ok {
		stats = sp.Stats(ctx)
	}

	for _, op := range a.base.Metrics().Snapshot() {
		stats.Hits += op.Hits
		stats.Misses += op.Misses
	}

	stats.HitRate = metrics.CalculateHitRate(stats.Hits, stats.Misses)
	return stats
}

func (a *advancedCache[T]) Metrics() *metrics.Collector {
	return a.base.Metrics()
}

/* ------------------ GetOrSet ------------------ */

func (a *advancedCache[T]) GetOrSet(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func() (T, error),
) (T, error) {
	return a.getOrSet(ctx, key, ttl, fn, false)
}

func (a *advancedCache[T]) GetOrSetLocked(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func() (T, error),
) (T, error) {
	return a.getOrSet(ctx, key, ttl, fn, true)
}

func (a *advancedCache[T]) getOrSet(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func() (T, error),
	locked bool,
) (T, error) {
	var _ T
	op := "get_or_set"
	if locked {
		op = "get_or_set_locked"
	}

	var result T
	err := a.withMetrics(op, 1, func() error {
		val, err := a.Get(ctx, key)
		if err == nil {
			result = val
			return nil
		}
		if !errors.Is(err, base.ErrCacheMiss) {
			return err
		}

		if locked {
			if err := a.tryLock(ctx, key); err != nil {
				return err
			}
			defer a.unlock(ctx, key)
		}

		val, err = fn()
		if err != nil {
			return err
		}

		_ = a.Set(ctx, key, val, ttl)
		result = val
		return nil
	})

	return result, err
}

// tryLock handles distributed lock acquisition
func (a *advancedCache[T]) tryLock(ctx context.Context, key string) error {
	locker, ok := a.cache.(interfaces.DistributedLocker)
	if !ok {
		return nil
	}
	ok, err := locker.TryLock(ctx, "lock:"+key, 30*time.Second)
	if err != nil || !ok {
		return err
	}
	return nil
}

// unlock handles releasing the lock
func (a *advancedCache[T]) unlock(ctx context.Context, key string) {
	if locker, ok := a.cache.(interfaces.DistributedLocker); ok {
		if err := locker.Unlock(ctx, "lock:"+key); err != nil {
			a.base.RecordError("get_or_set_locked")
		}
	}
}
