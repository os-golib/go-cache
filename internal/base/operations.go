package base

import (
	"context"
	"strings"
	"time"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/metrics"
)

/* ------------------ Base ------------------ */

type Base struct {
	Cfg       config.Config
	StartTime time.Time
	Collector *metrics.Collector
}

/* ------------------ Constructor ------------------ */

func NewBase(cfg config.Config) *Base {
	return &Base{
		Cfg:       cfg,
		StartTime: time.Now(),
		Collector: metrics.NewCollector(),
	}
}

/* ------------------ Key helpers ------------------ */

func (b *Base) FullKey(key string) string {
	if b.Cfg.Prefix == "" {
		return key
	}
	return b.Cfg.Prefix + key
}

func (b *Base) ValidateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return ErrKeyEmpty
	}
	return nil
}

/* ------------------ TTL helpers ------------------ */

func (b *Base) ResolveTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return b.Cfg.TTL
}

/* ------------------ Context helpers ------------------ */

func (b *Base) CheckContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

/* ------------------ Lifecycle ------------------ */

func (b *Base) Uptime() time.Duration {
	return time.Since(b.StartTime)
}

/* ------------------ Metrics helpers ------------------ */

func (b *Base) Metrics() *metrics.Collector {
	return b.Collector
}

func (b *Base) RecordOperation(op string, d time.Duration, n int) {
	if b.Collector != nil {
		b.Collector.RecordOperation(op, d, n)
	}
}

func (b *Base) RecordHit(op string, n int64) {
	if b.Collector != nil {
		b.Collector.RecordHit(op, n)
	}
}

func (b *Base) RecordMiss(op string, n int64) {
	if b.Collector != nil {
		b.Collector.RecordMiss(op, n)
	}
}

func (b *Base) RecordError(op string) {
	if b.Collector != nil {
		b.Collector.RecordError(op)
	}
}
