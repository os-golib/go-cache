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

/* ------------------ core factory ------------------ */

func newCache[T any](ctx context.Context, cfg config.Config) (interfaces.Cache[T], error) {
	if err := cfg.Validate(); err != nil {
		return nil, base.WrapError(base.OpInit, err, "")
	}

	switch cfg.Type {
	case config.TypeMemory:
		return memory.NewMemory[T](cfg)

	case config.TypeRedis:
		return redis.NewRedisContext[T](ctx, cfg)

	default:
		return nil, base.WrapError(base.OpInit, base.ErrInvalidConfig, string(cfg.Type))
	}
}

/* ------------------ public APIs ------------------ */

func New[T any](cfg config.Config) (interfaces.Cache[T], error) {
	return newCache[T](context.Background(), cfg)
}

func NewWithContext[T any](ctx context.Context, cfg config.Config) (interfaces.Cache[T], error) {
	return newCache[T](ctx, cfg)
}

func NewAdvanced[T any](cfg config.Config) (interfaces.AdvancedCache[T], error) {
	c, err := New[T](cfg)
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}
	return advanced.NewAdvancedCache[T](c, cfg), nil
}

func NewAdvancedWithContext[T any](ctx context.Context, cfg config.Config) (interfaces.AdvancedCache[T], error) {
	c, err := NewWithContext[T](ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}
	return advanced.NewAdvancedCache[T](c, cfg), nil
}

/* ------------------ helpers ------------------ */

func Must[T any](c interfaces.Cache[T], err error) interfaces.Cache[T] {
	if err != nil {
		panic(fmt.Errorf("cache init failed: %w", err))
	}
	return c
}

func MustAdvanced[T any](c interfaces.AdvancedCache[T], err error) interfaces.AdvancedCache[T] {
	if err != nil {
		panic(fmt.Errorf("advanced cache init failed: %w", err))
	}
	return c
}

/* ------------------ convenience ------------------ */

func NewMemory[T any]() (interfaces.Cache[T], error) {
	cfg := NewBuilder().
		WithMemory().
		WithMaxEntries(10_000).
		WithTTL(time.Minute).
		MustBuild()

	return New[T](cfg)
}

func NewRedis[T any](url string) (interfaces.Cache[T], error) {
	cfg := NewBuilder().
		WithRedis(url).
		WithPoolSize(10).
		MustBuild()

	return New[T](cfg)
}

func NewAdvancedMemory[T any]() (interfaces.AdvancedCache[T], error) {
	cfg := NewBuilder().
		WithMemory().
		WithMaxEntries(10_000).
		WithTTL(time.Minute).
		MustBuild()

	return NewAdvanced[T](cfg)
}

func NewAdvancedRedis[T any](url string) (interfaces.AdvancedCache[T], error) {
	cfg := NewBuilder().
		WithRedis(url).
		WithPoolSize(10).
		MustBuild()

	return NewAdvanced[T](cfg)
}
