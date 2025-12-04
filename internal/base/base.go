package base

import (
	"context"
	"time"

	"github.com/os-golib/go-cache/config"
)

// Base provides common cache functionality
type Base struct {
	Cfg config.Config
}

// FullKey returns the full key with prefix
func (b *Base) FullKey(key string) string {
	return b.Cfg.Prefix + key
}

// TTL returns the effective TTL for an operation
func (b *Base) TTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return b.Cfg.TTL
}

// ValidateKey validates a cache key
func (b *Base) ValidateKey(key string) error {
	if key == "" {
		return config.ErrKeyEmpty
	}
	return nil
}

// CheckContext checks if the context is done
func (b *Base) CheckContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return config.ErrTimeout
	default:
		return nil
	}
}
