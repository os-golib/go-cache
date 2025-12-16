package interfaces

import (
	"context"
	"time"

	"github.com/os-golib/go-cache/internal/metrics"
)

// Cache defines the core caching behaviour for all implementations.
type Cache[T any] interface {
	Getter[T]
	Setter[T]
	Deleter
	Checker
	Closer
	HealthChecker
	BulkOperations
}

type AdvancedCache[T any] interface {
	Cache[T]
	GetOrSet(ctx context.Context, key string, ttl time.Duration, fn func() (T, error)) (T, error)
	GetOrSetLocked(ctx context.Context, key string, ttl time.Duration, fn func() (T, error)) (T, error)
	GetManyPipeline(ctx context.Context, keys []string) (map[string]T, error)
	SetManyPipeline(ctx context.Context, items map[string]T, ttl time.Duration) error
	DeleteByPrefix(ctx context.Context, prefix string) (int64, error)
	Stats(ctx context.Context) metrics.CacheStats
	Metrics() *metrics.Collector
}

type Getter[T any] interface {
	Get(ctx context.Context, key string) (T, error)
}

type Setter[T any] interface {
	Set(ctx context.Context, key string, value T, ttl time.Duration) error
}

type Deleter interface {
	Delete(ctx context.Context, keys ...string) error
}

type Checker interface {
	Exists(ctx context.Context, key string) (bool, error)
}

type Closer interface {
	Close() error
}

type HealthChecker interface {
	Ping(ctx context.Context) error
}

type BulkOperations interface {
	Clear(ctx context.Context) error
	Len(ctx context.Context) (int, error)
}

type PipelineGetter[T any] interface {
	GetManyPipeline(ctx context.Context, keys []string) (map[string]T, error)
}

type PipelineSetter[T any] interface {
	SetManyPipeline(ctx context.Context, items map[string]T, ttl time.Duration) error
}

type PrefixDeleter interface {
	DeleteByPrefix(ctx context.Context, prefix string) (int64, error)
}

type StatProvider interface {
	Stats(ctx context.Context) metrics.CacheStats
}

type DistributedLocker interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string) error
}
