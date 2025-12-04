package redis

import (
	"context"
	"time"

	"github.com/os-golib/go-cache/config"
)

// TryLock attempts to acquire a distributed lock
func (r *redisCache[T]) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	lockKey := r.FullKey("lock:" + key)

	result, err := r.client.SetNX(ctx, lockKey, "1", ttl).Result()
	if err != nil {
		return false, config.NewError("redis lock", err, key)
	}

	return result, nil
}

// Unlock releases a distributed lock
func (r *redisCache[T]) Unlock(ctx context.Context, key string) error {
	lockKey := r.FullKey("lock:" + key)

	err := r.client.Del(ctx, lockKey).Err()
	if err != nil {
		return config.NewError("redis unlock", err, key)
	}

	return nil
}

// LockWithRetry attempts to acquire a lock with retry logic
func (r *redisCache[T]) LockWithRetry(ctx context.Context, key string, ttl time.Duration, maxRetries int, retryDelay time.Duration) (bool, error) {
	for i := 0; i < maxRetries; i++ {
		acquired, err := r.TryLock(ctx, key, ttl)
		if err != nil {
			return false, err
		}
		if acquired {
			return true, nil
		}

		if i < maxRetries-1 {
			select {
			case <-time.After(retryDelay):
				continue
			case <-ctx.Done():
				return false, config.ErrTimeout
			}
		}
	}

	return false, nil
}

// LockInfo represents lock information
type LockInfo struct {
	Key       string
	Holder    string
	CreatedAt time.Time
	TTL       time.Duration
}

// GetLockInfo retrieves information about a lock
func (r *redisCache[T]) GetLockInfo(ctx context.Context, key string) (*LockInfo, error) {
	lockKey := r.FullKey("lock:" + key)

	// Check if lock exists
	exists, err := r.client.Exists(ctx, lockKey).Result()
	if err != nil {
		return nil, config.NewError("redis lock info", err, key)
	}

	if exists == 0 {
		return nil, nil // Lock doesn't exist
	}

	// Get TTL
	ttl, err := r.client.TTL(ctx, lockKey).Result()
	if err != nil {
		return nil, config.NewError("redis lock ttl", err, key)
	}

	return &LockInfo{
		Key:       key,
		TTL:       ttl,
		CreatedAt: time.Now().Add(-ttl), // Approximate creation time
	}, nil
}
