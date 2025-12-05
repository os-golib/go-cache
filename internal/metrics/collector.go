package metrics

import (
	"sync"
	"time"
)

type Collector struct {
	mu         sync.RWMutex
	operations map[string]*OperationStats
	errors     map[string]int64
}

type OperationStats struct {
	Count         int64         `json:"count"`
	TotalDuration time.Duration `json:"total_duration"`
	MinDuration   time.Duration `json:"min_duration"`
	MaxDuration   time.Duration `json:"max_duration"`
	TotalItems    int64         `json:"total_items"`
	Hits          int64         `json:"hits"`
	Misses        int64         `json:"misses"`
}

func NewCollector() *Collector {
	return &Collector{
		operations: make(map[string]*OperationStats),
		errors:     make(map[string]int64),
	}
}

func (m *Collector) RecordOperation(op string, dur time.Duration, itemCount int) {
	if itemCount <= 0 {
		return
	}

	m.record(op, func(s *OperationStats) {
		s.Count++
		s.TotalItems += int64(itemCount)
		s.TotalDuration += dur

		if dur < s.MinDuration {
			s.MinDuration = dur
		}
		if dur > s.MaxDuration {
			s.MaxDuration = dur
		}
	})
}

func (m *Collector) RecordHit(op string, count int64) {
	if count <= 0 {
		return
	}
	m.record(op, func(s *OperationStats) { s.Hits += count })
}

func (m *Collector) RecordMiss(op string, count int64) {
	if count <= 0 {
		return
	}
	m.record(op, func(s *OperationStats) { s.Misses += count })
}

func (m *Collector) RecordError(op string) {
	m.record(op, func(s *OperationStats) {})

	m.mu.Lock()
	m.errors[op]++
	m.mu.Unlock()
}

func (m *Collector) Snapshot() map[string]OperationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]OperationStats, len(m.operations))
	for k, v := range m.operations {
		// compute dynamic values
		s := *v
		if s.Count > 0 {
			avg := time.Duration(int64(s.TotalDuration) / s.Count)
			s.TotalDuration = avg // store average in snapshot
		}
		out[k] = s
	}
	return out
}

func (m *Collector) ErrorCounts() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := make(map[string]int64, len(m.errors))
	for k, v := range m.errors {
		snapshot[k] = v
	}
	return snapshot
}

func (m *Collector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.operations = make(map[string]*OperationStats)
	m.errors = make(map[string]int64)
}

// ============================================================================
// helpers
// ============================================================================
func (m *Collector) record(op string, fn func(*OperationStats)) {
	if op == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	stat := m.operations[op]
	if stat == nil {
		stat = &OperationStats{MinDuration: time.Duration(1<<63 - 1)}
		m.operations[op] = stat
	}

	fn(stat)
}
