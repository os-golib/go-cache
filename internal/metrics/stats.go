package metrics

import (
	"time"
)

// StatsBuilder helps build cache statistics
type StatsBuilder struct {
	stats CacheStats
}

// NewStatsBuilder creates a new stats builder
func NewStatsBuilder(backend string) *StatsBuilder {
	return &StatsBuilder{
		stats: CacheStats{
			Backend: backend,
		},
	}
}

// WithItems sets the number of cached items
func (b *StatsBuilder) WithItems(items int64) *StatsBuilder {
	if items >= 0 {
		b.stats.Items = items
	}
	return b
}

// WithHits sets the number of hits
func (b *StatsBuilder) WithHits(hits int64) *StatsBuilder {
	if hits >= 0 {
		b.stats.Hits = hits
	}
	return b
}

// WithMisses sets the number of misses
func (b *StatsBuilder) WithMisses(misses int64) *StatsBuilder {
	if misses >= 0 {
		b.stats.Misses = misses
	}
	return b
}

// WithUptime sets the cache uptime
func (b *StatsBuilder) WithUptime(uptime time.Duration) *StatsBuilder {
	if uptime >= 0 {
		b.stats.Uptime = uptime
	}
	return b
}

// WithRefreshTTL sets the refresh TTL
func (b *StatsBuilder) WithRefreshTTL(refresh bool) *StatsBuilder {
	b.stats.RefreshTTLOnHit = refresh
	return b
}

// AddHits adds hits
func (b *StatsBuilder) AddHits(n int64) *StatsBuilder {
	if n > 0 {
		b.stats.Hits += n
	}
	return b
}

// AddMisses adds misses
func (b *StatsBuilder) AddMisses(n int64) *StatsBuilder {
	if n > 0 {
		b.stats.Misses += n
	}
	return b
}

// AddItems adds items
func (b *StatsBuilder) AddItems(n int64) *StatsBuilder {
	if n > 0 {
		b.stats.Items += n
	}
	return b
}

// Build returns the final cache stats
func (b *StatsBuilder) Build() CacheStats {
	// Fix negative values if set incorrectly
	if b.stats.Hits < 0 {
		b.stats.Hits = 0
	}
	if b.stats.Misses < 0 {
		b.stats.Misses = 0
	}
	if b.stats.Items < 0 {
		b.stats.Items = 0
	}

	// Calculate hit rate
	total := b.stats.Hits + b.stats.Misses
	if total > 0 {
		b.stats.HitRate = float64(b.stats.Hits) / float64(total)
	} else {
		b.stats.HitRate = 0
	}

	return b.stats
}

// CalculateHitRate calculates hit rate from hits and misses
func CalculateHitRate(hits, misses int64) float64 {
	if hits < 0 || misses < 0 {
		return 0
	}
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total)
}

// MergeStats merges multiple cache stats
func MergeStats(stats ...CacheStats) CacheStats {
	if len(stats) == 0 {
		return CacheStats{}
	}

	merged := CacheStats{
		Backend: "merged",
	}

	for _, sts := range stats {
		if sts.Items > 0 {
			merged.Items += sts.Items
		}
		if sts.Hits > 0 {
			merged.Hits += sts.Hits
		}
		if sts.Misses > 0 {
			merged.Misses += sts.Misses
		}
		if sts.Uptime > merged.Uptime {
			merged.Uptime = sts.Uptime
		}

		// If any backend has refresh enabled, enable it in merged
		if sts.RefreshTTLOnHit {
			merged.RefreshTTLOnHit = true
		}
	}

	merged.HitRate = CalculateHitRate(merged.Hits, merged.Misses)
	return merged
}
