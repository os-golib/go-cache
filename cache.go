package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/advanced"
	"github.com/os-golib/go-cache/internal/base"
	"github.com/os-golib/go-cache/internal/interfaces"
	"github.com/os-golib/go-cache/memory"
	"github.com/os-golib/go-cache/redis"
)

// New creates a new cache instance based on configuration
// This is the main entry point for creating cache instances
func New[T any](cfg config.Config) (interfaces.Cache[T], error) {
	if err := cfg.Validate(); err != nil {
		return nil, base.NewError("config validation", err, "")
	}

	switch cfg.Type {
	case config.TypeRedis:
		return redis.NewRedisCache[T](cfg)
	case config.TypeMemory:
		return memory.NewMemoryCache[T](cfg)
	default:
		return nil, base.NewError("factory", base.ErrInvalidConfig, string(cfg.Type))
	}
}

// NewWithContext creates a new cache instance with context for initialization
func NewWithContext[T any](ctx context.Context, cfg config.Config) (interfaces.Cache[T], error) {
	if err := cfg.Validate(); err != nil {
		return nil, base.NewError("config validation", err, "")
	}

	switch cfg.Type {
	case config.TypeRedis:
		return redis.NewRedisContext[T](ctx, cfg)
	case config.TypeMemory:
		return memory.NewMemoryContext[T](ctx, cfg)
	default:
		return nil, base.NewError("factory", base.ErrInvalidConfig, string(cfg.Type))
	}
}

// NewAdvanced creates an advanced cache instance with additional features
// Advanced cache includes metrics, bulk operations, and distributed locking
func NewAdvanced[T any](cfg config.Config) (interfaces.AdvancedCache[T], error) {
	Cache, err := New[T](cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create base cache: %w", err)
	}
	return advanced.NewAdvancedCache[T](Cache, cfg), nil
}

// NewAdvancedWithContext creates an advanced cache instance with context
func NewAdvancedWithContext[T any](ctx context.Context, cfg config.Config) (interfaces.AdvancedCache[T], error) {
	Cache, err := NewWithContext[T](ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create base cache: %w", err)
	}
	return advanced.NewAdvancedCache[T](Cache, cfg), nil
}

// NewMemory creates a memory cache with default configuration
// Convenience function for quick memory cache setup
func NewMemory[T any]() (interfaces.Cache[T], error) {
	cfg, _ := config.NewBuilder(config.TypeMemory).WithMemoryConfig(10000, time.Minute).Build()
	return New[T](cfg)
}

// NewRedis creates a Redis cache with the provided URL
// Convenience function for quick Redis cache setup
func NewRedis[T any](url string) (interfaces.Cache[T], error) {
	cfg, _ := config.NewBuilder(config.TypeRedis).WithRedisConfig(url, 10).Build()
	return New[T](cfg)
}

// NewMemoryAdvanced creates an advanced memory cache
func NewMemoryAdvanced[T any]() (interfaces.AdvancedCache[T], error) {
	cfg, _ := config.NewBuilder(config.TypeMemory).WithMemoryConfig(10000, time.Minute).Build()
	return NewAdvanced[T](cfg)
}

// NewRedisAdvanced creates an advanced Redis cache
func NewRedisAdvanced[T any](url string) (interfaces.AdvancedCache[T], error) {
	cfg, _ := config.NewBuilder(config.TypeRedis).WithRedisConfig(url, 10).Build()
	return NewAdvanced[T](cfg)
}

// Must is a helper that wraps a call to a function returning (Cache, error)
// and panics if the error is non-nil. It is intended for use in variable
// initializations and tests where cache creation should not fail.
func Must[T any](cache interfaces.Cache[T], err error) interfaces.Cache[T] {
	if err != nil {
		panic(fmt.Sprintf("cache creation failed: %v", err))
	}
	return cache
}

// MustAdvanced is similar to Must but for AdvancedCache
func MustAdvanced[T any](cache interfaces.AdvancedCache[T], err error) interfaces.AdvancedCache[T] {
	if err != nil {
		panic(fmt.Sprintf("advanced cache creation failed: %v", err))
	}
	return cache
}
