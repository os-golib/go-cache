package base

import (
	"time"

	"github.com/os-golib/go-cache/internal/metrics"
)

// Cache provides common cache functionality
type Cache struct {
	startTime time.Time
	metrics   *metrics.Collector
}

// NewCache creates a new base cache instance
func NewCache() *Cache {
	return &Cache{
		startTime: time.Now(),
		metrics:   metrics.NewCollector(),
	}
}

// Uptime returns the cache uptime
func (b *Cache) Uptime() time.Duration {
	return time.Since(b.startTime)
}

// Metrics returns the metrics collector
func (b *Cache) Metrics() *metrics.Collector {
	return b.metrics
}

// RecordOperation records an operation for metrics
func (b *Cache) RecordOperation(op string, d time.Duration, n int) {
	b.metrics.ObserveOperation(op, d, n)
}

// RecordHit records a cache hit
func (b *Cache) RecordHit(op string, n int64) {
	b.metrics.RecordHit(op, n)
}

// RecordMiss records a cache miss
func (b *Cache) RecordMiss(op string, n int64) {
	b.metrics.RecordMiss(op, n)
}

// RecordError records an operation error
func (b *Cache) RecordError(op string) {
	b.metrics.RecordError(op)
}
