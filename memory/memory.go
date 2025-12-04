package memory

import (
	"container/list"
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/base"
	"github.com/os-golib/go-cache/internal/metrics"
)

// memoryItem holds a cache entry
type memoryItem[T any] struct {
	key        string
	value      T
	expiration time.Time
}

// memoryCache implements an in-memory LRU cache
type memoryCache[T any] struct {
	base.Base
	Cache           *base.Cache
	items           map[string]*list.Element
	lruList         *list.List
	mu              sync.RWMutex
	stopCh          chan struct{}
	capacity        int
	cleanupInterval time.Duration
	evictionPolicy  string
	_len            int64
	startTime       time.Time
}

// NewMemoryCache creates a new memory cache instance
func NewMemoryCache[T any](cfg config.Config) (*memoryCache[T], error) {
	return NewMemoryContext[T](context.Background(), cfg)
}

// NewMemoryContext creates a new memory cache instance using context for lifecycle control.
func NewMemoryContext[T any](ctx context.Context, cfg config.Config) (*memoryCache[T], error) {
	if cfg.MaxSize > 0 && cfg.EvictionPolicy != "" && cfg.EvictionPolicy != "lru" {
		return nil, config.ErrInvalidConfig
	}

	mc := &memoryCache[T]{
		Base:            base.Base{Cfg: cfg},
		Cache:           base.NewCache(),
		items:           make(map[string]*list.Element),
		lruList:         list.New(),
		stopCh:          make(chan struct{}),
		capacity:        cfg.MaxSize,
		cleanupInterval: cfg.CleanupInterval,
		evictionPolicy:  cfg.EvictionPolicy,
		startTime:       time.Now(),
	}

	// Run cleanup loop if interval > 0
	if mc.cleanupInterval > 0 {
		go mc.startCleanup(ctx)
	}

	return mc, nil
}

// Get retrieves a value from the cache
func (c *memoryCache[T]) Get(ctx context.Context, key string) (T, error) {
	start := time.Now()
	defer func() { c.Cache.RecordOperation("get", time.Since(start), 1) }()

	var zero T
	if err := c.CheckContext(ctx); err != nil {
		return zero, err
	}
	if err := c.ValidateKey(key); err != nil {
		return zero, err
	}
	fullKey := c.FullKey(key)

	c.mu.RLock()
	elem, ok := c.items[fullKey]
	if !ok {
		c.mu.RUnlock()
		c.Cache.RecordMiss("get", 1)
		return zero, config.ErrCacheMiss
	}
	item := elem.Value.(*memoryItem[T])
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		c.mu.RUnlock()
		c.mu.Lock()
		c.removeElement(elem)
		atomic.AddInt64(&c._len, -1)
		c.mu.Unlock()
		c.Cache.RecordMiss("get", 1)
		return zero, config.ErrCacheMiss
	}
	c.mu.RUnlock()

	// Promote to front for LRU
	c.mu.Lock()
	c.lruList.MoveToFront(elem)
	c.mu.Unlock()

	c.Cache.RecordHit("get", 1)
	return item.value, nil
}

// Set stores a value in the cache
func (c *memoryCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	start := time.Now()
	defer func() { c.Cache.RecordOperation("set", time.Since(start), 1) }()

	if err := c.CheckContext(ctx); err != nil {
		return err
	}
	if err := c.ValidateKey(key); err != nil {
		return err
	}
	fullKey := c.FullKey(key)
	effectiveTTL := c.TTL(ttl)

	var expiration time.Time
	if effectiveTTL > 0 {
		expiration = time.Now().Add(effectiveTTL)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[fullKey]; ok {
		item := elem.Value.(*memoryItem[T])
		item.value = value
		item.expiration = expiration
		c.lruList.MoveToFront(elem)
		return nil
	}

	// Eviction if at capacity
	if c.capacity > 0 && int(atomic.LoadInt64(&c._len)) >= c.capacity {
		c.evict()
	}

	it := &memoryItem[T]{
		key:        fullKey,
		value:      value,
		expiration: expiration,
	}
	elem := c.lruList.PushFront(it)
	c.items[fullKey] = elem
	atomic.AddInt64(&c._len, 1)
	return nil
}

// Delete removes keys from the cache
func (c *memoryCache[T]) Delete(ctx context.Context, keys ...string) error {
	start := time.Now()
	defer func() { c.Cache.RecordOperation("delete", time.Since(start), len(keys)) }()

	if len(keys) == 0 {
		return nil
	}
	if err := c.CheckContext(ctx); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, k := range keys {
		fk := c.FullKey(k)
		if elem, ok := c.items[fk]; ok {
			c.removeElement(elem)
			delete(c.items, fk)
			atomic.AddInt64(&c._len, -1)
		}
	}
	return nil
}

// Exists checks if a key exists in the cache
func (c *memoryCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	start := time.Now()
	defer func() { c.Cache.RecordOperation("exists", time.Since(start), 1) }()

	if err := c.CheckContext(ctx); err != nil {
		return false, err
	}
	if err := c.ValidateKey(key); err != nil {
		return false, err
	}
	fk := c.FullKey(key)

	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.items[fk]
	if !ok {
		return false, nil
	}
	item := elem.Value.(*memoryItem[T])
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		c.removeElement(elem)
		delete(c.items, fk)
		atomic.AddInt64(&c._len, -1)
		return false, nil
	}
	return true, nil
}

// Close stops the cleanup goroutine
func (c *memoryCache[T]) Close() error {
	close(c.stopCh)
	return nil
}

// Ping checks if the cache is alive
func (c *memoryCache[T]) Ping(ctx context.Context) error {
	return c.CheckContext(ctx)
}

// Clear removes all items from the cache
func (c *memoryCache[T]) Clear(ctx context.Context) error {
	start := time.Now()
	defer func() { c.Cache.RecordOperation("clear", time.Since(start), 1) }()

	if err := c.CheckContext(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.lruList.Init()
	atomic.StoreInt64(&c._len, 0)
	return nil
}

// Len returns the number of items in the cache
func (c *memoryCache[T]) Len(ctx context.Context) (int, error) {
	if err := c.CheckContext(ctx); err != nil {
		return 0, err
	}
	return int(atomic.LoadInt64(&c._len)), nil
}

// DeleteByPrefix removes all keys with the given prefix
func (c *memoryCache[T]) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	start := time.Now()
	defer func() {
		c.Cache.RecordOperation("delete_by_prefix", time.Since(start), 1)
	}()

	if err := c.CheckContext(ctx); err != nil {
		return 0, err
	}
	fullPref := c.FullKey(prefix)

	c.mu.Lock()
	defer c.mu.Unlock()
	var deleted int64
	for k, elem := range c.items {
		if strings.HasPrefix(k, fullPref) {
			c.removeElement(elem)
			delete(c.items, k)
			atomic.AddInt64(&c._len, -1)
			deleted++
		}
	}
	return deleted, nil
}

// Stats returns cache statistics
func (c *memoryCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	// Get number of cached keys
	items, err := c.Len(ctx)
	items64 := int64(0)
	if err == nil && items >= 0 {
		items64 = int64(items)
	}

	// Collect hits & misses from metrics snapshot
	snapshot := c.Cache.Metrics().Snapshot()
	var hits, misses int64
	for _, stat := range snapshot {
		if stat.Hits > 0 {
			hits += stat.Hits
		}
		if stat.Misses > 0 {
			misses += stat.Misses
		}
	}

	return metrics.CacheStats{
		Backend:         "memory",
		Items:           items64,
		Hits:            hits,
		Misses:          misses,
		HitRate:         metrics.CalculateHitRate(hits, misses),
		Uptime:          c.Cache.Uptime(),
		RefreshTTLOnHit: c.Cfg.RefreshTTLOnHit,
	}
}

// startCleanup starts the background cleanup goroutine
func (c *memoryCache[T]) startCleanup(ctx context.Context) {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-ctx.Done(): // external shutdown
			return
		case <-c.stopCh: // internal shutdown
			return
		}
	}
}

// deleteExpired removes expired items
func (c *memoryCache[T]) deleteExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, elem := range c.items {
		item := elem.Value.(*memoryItem[T])
		if !item.expiration.IsZero() && time.Now().After(item.expiration) {
			c.removeElement(elem)
			atomic.AddInt64(&c._len, -1)
		}
	}
}

// evict removes the least recently used item
func (c *memoryCache[T]) evict() {
	if e := c.lruList.Back(); e != nil {
		c.removeElement(e)
		atomic.AddInt64(&c._len, -1)
	}
}

// removeElement removes an element from the list and map
func (c *memoryCache[T]) removeElement(elem *list.Element) {
	c.lruList.Remove(elem)
	item := elem.Value.(*memoryItem[T])
	delete(c.items, item.key)
}
