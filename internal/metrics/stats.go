package metrics

import "time"

type CacheStats struct {
	Backend         string        `json:"backend"`
	Items           int64         `json:"items"`
	Hits            int64         `json:"hits"`
	Misses          int64         `json:"misses"`
	HitRate         float64       `json:"hit_rate"`
	Uptime          time.Duration `json:"uptime"`
	RefreshTTLOnHit bool          `json:"refresh_on_hit"`
}

type StatsBuilder struct {
	stats CacheStats
}

func NewStatsBuilder(backend string) *StatsBuilder {
	return &StatsBuilder{stats: CacheStats{Backend: backend}}
}

func (b *StatsBuilder) WithItems(items int64) *StatsBuilder {
	b.stats.Items = max(0, items)
	return b
}

func (b *StatsBuilder) WithHits(hits int64) *StatsBuilder {
	b.stats.Hits = max(0, hits)
	return b
}

func (b *StatsBuilder) WithMisses(misses int64) *StatsBuilder {
	b.stats.Misses = max(0, misses)
	return b
}

func (b *StatsBuilder) WithUptime(uptime time.Duration) *StatsBuilder {
	b.stats.Uptime = max(0, uptime)
	return b
}

func (b *StatsBuilder) WithRefreshTTL(refresh bool) *StatsBuilder {
	b.stats.RefreshTTLOnHit = refresh
	return b
}

func (b *StatsBuilder) AddHits(n int64) *StatsBuilder {
	if n > 0 {
		b.stats.Hits += n
	}
	return b
}

func (b *StatsBuilder) AddMisses(n int64) *StatsBuilder {
	if n > 0 {
		b.stats.Misses += n
	}
	return b
}

func (b *StatsBuilder) AddItems(n int64) *StatsBuilder {
	if n > 0 {
		b.stats.Items += n
	}
	return b
}

func (b *StatsBuilder) Build() CacheStats {
	b.stats.Hits = max(0, b.stats.Hits)
	b.stats.Misses = max(0, b.stats.Misses)
	b.stats.Items = max(0, b.stats.Items)
	b.stats.HitRate = CalculateHitRate(b.stats.Hits, b.stats.Misses)
	return b.stats
}

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

func MergeStats(stats ...CacheStats) CacheStats {
	if len(stats) == 0 {
		return CacheStats{}
	}

	builder := NewStatsBuilder("merged")
	maxUptime := time.Duration(0)
	refreshEnabled := false

	for _, s := range stats {
		builder.AddItems(s.Items).AddHits(s.Hits).AddMisses(s.Misses)
		maxUptime = max(maxUptime, s.Uptime)
		refreshEnabled = refreshEnabled || s.RefreshTTLOnHit
	}

	return builder.WithUptime(maxUptime).WithRefreshTTL(refreshEnabled).Build()
}
