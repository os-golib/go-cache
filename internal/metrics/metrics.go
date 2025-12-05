package metrics

// import (
// 	"sync"
// 	"time"
// )

// // Collector collects and reports cache metrics
// type Collector struct {
// 	mu         sync.RWMutex
// 	operations map[string]*OperationStats
// 	errors     map[string]int64
// }

// // OperationStats tracks stats for a single operation
// type OperationStats struct {
// 	Count         int64         `json:"count"`
// 	TotalDuration time.Duration `json:"total_duration"`
// 	MinDuration   time.Duration `json:"min_duration"`
// 	MaxDuration   time.Duration `json:"max_duration"`
// 	TotalItems    int64         `json:"total_items"`
// 	Hits          int64         `json:"hits"`
// 	Misses        int64         `json:"misses"`
// }

// // NewCollector returns a new metrics collector
// func NewCollector() *Collector {
// 	return &Collector{
// 		operations: make(map[string]*OperationStats),
// 		errors:     make(map[string]int64),
// 	}
// }

// // ObserveOperation tracks execution time + number of items processed.
// func (m *Collector) ObserveOperation(op string, dur time.Duration, itemCount int) {
// 	if op == "" || itemCount <= 0 {
// 		return
// 	}

// 	m.mu.Lock()
// 	defer m.mu.Unlock()

// 	stat := m.operations[op]
// 	if stat == nil {
// 		stat = &OperationStats{
// 			MinDuration: time.Duration(1<<63 - 1), // max int64
// 		}
// 		m.operations[op] = stat
// 	}

// 	stat.Count++
// 	stat.TotalItems += int64(itemCount)
// 	stat.TotalDuration += dur

// 	// min/max duration
// 	if dur < stat.MinDuration {
// 		stat.MinDuration = dur
// 	}
// 	if dur > stat.MaxDuration {
// 		stat.MaxDuration = dur
// 	}
// }

// // RecordHit records a cache hit
// func (m *Collector) RecordHit(op string, itemCount int64) {
// 	if op == "" || itemCount <= 0 {
// 		return
// 	}

// 	m.mu.Lock()
// 	defer m.mu.Unlock()

// 	opts := m.operations[op]
// 	if opts == nil {
// 		opts = &OperationStats{
// 			MinDuration: time.Duration(1<<63 - 1),
// 		}
// 		m.operations[op] = opts
// 	}
// 	opts.Hits += itemCount
// }

// // RecordMiss records a cache miss
// func (m *Collector) RecordMiss(op string, itemCount int64) {
// 	if op == "" || itemCount <= 0 {
// 		return
// 	}

// 	m.mu.Lock()
// 	defer m.mu.Unlock()

// 	opts := m.operations[op]
// 	if opts == nil {
// 		opts = &OperationStats{
// 			MinDuration: time.Duration(1<<63 - 1),
// 		}
// 		m.operations[op] = opts
// 	}
// 	opts.Misses += itemCount
// }

// // RecordError records an operation error
// func (m *Collector) RecordError(op string) {
// 	if op == "" {
// 		return
// 	}

// 	m.mu.Lock()
// 	defer m.mu.Unlock()

// 	m.errors[op]++
// }

// // Snapshot returns a snapshot of the current metrics
// func (m *Collector) Snapshot() map[string]OperationStats {
// 	m.mu.RLock()
// 	defer m.mu.RUnlock()

// 	out := make(map[string]OperationStats, len(m.operations))
// 	for k, v := range m.operations {
// 		out[k] = *v
// 	}
// 	return out
// }

// // ErrorCounts returns a snapshot of the current error counts
// func (m *Collector) ErrorCounts() map[string]int64 {
// 	m.mu.RLock()
// 	defer m.mu.RUnlock()

// 	snapshot := make(map[string]int64, len(m.errors))
// 	for k, v := range m.errors {
// 		snapshot[k] = v
// 	}
// 	return snapshot
// }

// // Reset resets the collector
// func (m *Collector) Reset() {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()

// 	m.operations = make(map[string]*OperationStats)
// 	m.errors = make(map[string]int64)
// }
