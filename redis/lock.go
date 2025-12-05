package redis

import (
	"context"
	"time"

	"github.com/os-golib/go-cache/internal/base"
)

const lockValue = "1"

// TryLock attempts to acquire a distributed lock
func (r *redisCache[T]) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if err := r.CheckContext(ctx); err != nil {
		return false, err
	}

	lockKey := r.FullKey("lock:" + key)

	// Use SET NX (set if not exists)
	acquired, err := r.client.SetNX(ctx, lockKey, lockValue, ttl).Result()
	if err != nil {
		return false, base.NewError("lock acquire", err, key)
	}

	return acquired, nil
}

// Unlock releases a distributed lock
func (r *redisCache[T]) Unlock(ctx context.Context, key string) error {
	if err := r.CheckContext(ctx); err != nil {
		return err
	}

	lockKey := r.FullKey("lock:" + key)

	if err := r.client.Del(ctx, lockKey).Err(); err != nil {
		return base.NewError("lock release", err, key)
	}

	return nil
}

// WithLock executes a function while holding a lock
func (r *redisCache[T]) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	acquired, err := r.TryLock(ctx, key, ttl)
	if err != nil {
		return err
	}
	if !acquired {
		return base.ErrLockAcquisition
	}

	defer func() {
		_ = r.Unlock(ctx, key)
	}()

	return fn()
}
