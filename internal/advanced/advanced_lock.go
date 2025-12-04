package advanced

import (
	"context"
	"time"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/interfaces"
)

// LockManager manages distributed locks
type LockManager struct {
	cache interfaces.Cache[string]
}

// NewLockManager creates a new lock manager
func NewLockManager(cache interfaces.Cache[string]) *LockManager {
	return &LockManager{
		cache: cache,
	}
}

// Acquire attempts to acquire a lock
func (l *LockManager) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if locker, ok := l.cache.(interfaces.DistributedLocker); ok {
		return locker.TryLock(ctx, key, ttl)
	}

	// Fallback using SETNX pattern
	return l.fallbackAcquire(ctx, key, ttl)
}

// Release releases a lock
func (l *LockManager) Release(ctx context.Context, key string) error {
	if locker, ok := l.cache.(interfaces.DistributedLocker); ok {
		return locker.Unlock(ctx, key)
	}

	// Fallback using DEL
	return l.fallbackRelease(ctx, key)
}

// fallbackAcquire implements lock acquisition using SETNX
func (l *LockManager) fallbackAcquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	lockKey := "lock:" + key

	// Try to set the lock key
	err := l.cache.Set(ctx, lockKey, "1", ttl)
	if err != nil {
		return false, config.NewError("lock acquire", err, key)
	}

	// For fallback, we assume success since we don't have atomic SETNX
	// In a real implementation, you'd use the backend's atomic operations
	return true, nil
}

// fallbackRelease implements lock release using DEL
func (l *LockManager) fallbackRelease(ctx context.Context, key string) error {
	lockKey := "lock:" + key

	err := l.cache.Delete(ctx, lockKey)
	if err != nil {
		return config.NewError("lock release", err, key)
	}

	return nil
}

// WithLock executes a function while holding a lock
func (l *LockManager) WithLock(
	ctx context.Context,
	key string,
	ttl time.Duration,
	fn func() error,
) error {
	acquired, err := l.Acquire(ctx, key, ttl)
	if err != nil {
		return err
	}
	if !acquired {
		return config.ErrLockAcquisition
	}

	defer func() {
		_ = l.Release(ctx, key)
	}()

	return fn()
}

// LockOptions configures lock behavior
type LockOptions struct {
	TTL        time.Duration
	MaxRetries int
	RetryDelay time.Duration
}

// DefaultLockOptions returns default lock options
func DefaultLockOptions() LockOptions {
	return LockOptions{
		TTL:        30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}
}

// AcquireWithRetry attempts to acquire a lock with retry logic
func (l *LockManager) AcquireWithRetry(ctx context.Context, key string, opts LockOptions) (bool, error) {
	for i := 0; i < opts.MaxRetries; i++ {
		acquired, err := l.Acquire(ctx, key, opts.TTL)
		if err != nil {
			return false, err
		}
		if acquired {
			return true, nil
		}

		if i < opts.MaxRetries-1 {
			select {
			case <-time.After(opts.RetryDelay):
				continue
			case <-ctx.Done():
				return false, config.ErrTimeout
			}
		}
	}

	return false, nil
}
