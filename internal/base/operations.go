package base

import (
	"context"
	"time"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/metrics"
)

type Base struct {
	Cfg       config.Config
	StartTime time.Time
	Metrics   *metrics.Collector
}

func NewBase(cfg config.Config) *Base {
	return &Base{
		Cfg:       cfg,
		StartTime: time.Now(),
		Metrics:   metrics.NewCollector(),
	}
}

func (b *Base) FullKey(key string) string {
	return b.Cfg.Prefix + key
}

func (b *Base) TTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return b.Cfg.TTL
}

func (b *Base) ValidateKey(key string) error {
	if key == "" {
		return ErrKeyEmpty
	}
	return nil
}

func (b *Base) CheckContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ErrTimeout
	default:
		return nil
	}
}

func (b *Base) Uptime() time.Duration {
	return time.Since(b.StartTime)
}

func (b *Base) RecordOperation(op string, d time.Duration, n int) {
	b.Metrics.RecordOperation(op, d, n)
}

func (b *Base) RecordHit(op string, n int64) {
	b.Metrics.RecordHit(op, n)
}

func (b *Base) RecordMiss(op string, n int64) {
	b.Metrics.RecordMiss(op, n)
}

func (b *Base) RecordError(op string) {
	b.Metrics.RecordError(op)
}
