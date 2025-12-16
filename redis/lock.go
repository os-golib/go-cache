package redis

import (
	"context"
	"time"

	"github.com/os-golib/go-cache/internal/base"
)

const lockValue = "1"

/* ------------------ Lock API ------------------ */

// TryLock attempts to acquire a distributed lock using SET NX
func (r *redisCache[T]) TryLock(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (bool, error) {
	if err := r.base.ValidateKey(key); err != nil {
		return false, err
	}
	if err := r.base.CheckContext(ctx); err != nil {
		return false, err
	}

	ttl = r.base.ResolveTTL(ttl)
	lockKey := r.base.FullKey("lock:" + key)

	acquired, err := r.client.SetNX(ctx, lockKey, lockValue, ttl).Result()
	if err != nil {
		return false, base.WrapError(base.OpLock, err, key)
	}

	return acquired, nil
}

// Unlock releases a distributed lock
func (r *redisCache[T]) Unlock(
	ctx context.Context,
	key string,
) error {
	if err := r.base.ValidateKey(key); err != nil {
		return err
	}
	if err := r.base.CheckContext(ctx); err != nil {
		return err
	}

	lockKey := r.base.FullKey("lock:" + key)

	if err := r.client.Del(ctx, lockKey).Err(); err != nil {
		return base.WrapError(base.OpUnlock, err, key)
	}

	return nil
}

// WithLock executes a function while holding a distributed lock
func (r *redisCache[T]) WithLock(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func() error,
) error {
	acquired, err := r.TryLock(ctx, key, ttl)
	if err != nil {
		return err
	}
	if !acquired {
		return base.WrapError(base.OpLock, base.ErrLockAcquire, key)
	}

	defer func() {
		_ = r.Unlock(ctx, key)
	}()

	return fn()
}
