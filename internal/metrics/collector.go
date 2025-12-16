package metrics

import (
	"sync"
	"time"
)

/* ------------------ Config ------------------ */

type Config struct {
	Enabled bool
}

func DefaultConfig() Config {
	return Config{
		Enabled: true,
	}
}

/* ------------------ Collector ------------------ */

type Collector struct {
	cfg Config

	mu         sync.RWMutex
	operations map[string]*OperationStats
	errors     map[string]int64
}

type OperationStats struct {
	Count         int64         `json:"count"`
	TotalItems    int64         `json:"total_items"`
	TotalDuration time.Duration `json:"total_duration"`

	MinDuration time.Duration `json:"min_duration"`
	MaxDuration time.Duration `json:"max_duration"`

	Hits   int64 `json:"hits"`
	Misses int64 `json:"misses"`
}

/* ------------------ Snapshots ------------------ */

type SnapshotStats struct {
	Count      int64 `json:"count"`
	TotalItems int64 `json:"total_items"`

	MinDuration time.Duration `json:"min_duration"`
	MaxDuration time.Duration `json:"max_duration"`
	AvgDuration time.Duration `json:"avg_duration"`

	Hits   int64 `json:"hits"`
	Misses int64 `json:"misses"`
	Errors int64 `json:"errors"`
}

/* ------------------ Constructor ------------------ */

func NewCollector() *Collector {
	return &Collector{
		operations: make(map[string]*OperationStats),
		errors:     make(map[string]int64),
	}
}

/* ------------------ Recording ------------------ */

func (m *Collector) RecordOperation(op string, dur time.Duration, itemCount int) {
	if !m.cfg.Enabled || op == "" || itemCount <= 0 {
		return
	}

	m.record(op, func(s *OperationStats) {
		s.Count++
		s.TotalItems += int64(itemCount)
		s.TotalDuration += dur

		if s.MinDuration == 0 || dur < s.MinDuration {
			s.MinDuration = dur
		}
		if dur > s.MaxDuration {
			s.MaxDuration = dur
		}
	})
}

func (m *Collector) RecordHit(op string, count int64) {
	if !m.cfg.Enabled || op == "" || count <= 0 {
		return
	}
	m.record(op, func(s *OperationStats) { s.Hits += count })
}

func (m *Collector) RecordMiss(op string, count int64) {
	if !m.cfg.Enabled || op == "" || count <= 0 {
		return
	}
	m.record(op, func(s *OperationStats) { s.Misses += count })
}

func (m *Collector) RecordError(op string) {
	if !m.cfg.Enabled || op == "" {
		return
	}

	m.mu.Lock()
	m.errors[op]++
	m.mu.Unlock()
}

/* ------------------ Snapshot ------------------ */

func (m *Collector) Snapshot() map[string]SnapshotStats {
	if !m.cfg.Enabled {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]SnapshotStats, len(m.operations))

	for op, s := range m.operations {
		var avg time.Duration
		if s.Count > 0 {
			avg = time.Duration(int64(s.TotalDuration) / s.Count)
		}

		out[op] = SnapshotStats{
			Count:       s.Count,
			TotalItems:  s.TotalItems,
			MinDuration: s.MinDuration,
			MaxDuration: s.MaxDuration,
			AvgDuration: avg,
			Hits:        s.Hits,
			Misses:      s.Misses,
			Errors:      m.errors[op],
		}
	}

	return out
}

/* ------------------ Maintenance ------------------ */

func (m *Collector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.operations = make(map[string]*OperationStats)
	m.errors = make(map[string]int64)
}

/* ------------------ Helpers ------------------ */

func (m *Collector) record(op string, fn func(*OperationStats)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stat := m.operations[op]
	if stat == nil {
		stat = &OperationStats{}
		m.operations[op] = stat
	}

	fn(stat)
}
