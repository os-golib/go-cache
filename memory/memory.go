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
	*base.Base
	items    map[string]*list.Element
	lruList  *list.List
	mu       sync.RWMutex
	stopCh   chan struct{}
	capacity int
	len      int64
}

// NewMemoryCache creates a new memory cache instance
func NewMemoryCache[T any](cfg config.Config) (*memoryCache[T], error) {
	return NewMemoryContext[T](context.Background(), cfg)
}

// NewMemoryContext creates a new memory cache with context for lifecycle control
func NewMemoryContext[T any](ctx context.Context, cfg config.Config) (*memoryCache[T], error) {
	if cfg.MaxSize > 0 && cfg.EvictionPolicy != "" && cfg.EvictionPolicy != "lru" {
		return nil, base.ErrInvalidConfig
	}

	mc := &memoryCache[T]{
		Base:     base.NewBase(cfg),
		items:    make(map[string]*list.Element),
		lruList:  list.New(),
		stopCh:   make(chan struct{}),
		capacity: cfg.MaxSize,
	}

	// Run cleanup loop if interval > 0
	if cfg.CleanupInterval > 0 {
		go mc.startCleanup(ctx, cfg.CleanupInterval)
	}

	return mc, nil
}

// Unified operation wrapper with metrics and locking
func (c *memoryCache[T]) execute(ctx context.Context, op string, itemCount int, fn func() error) error {
	start := time.Now()
	defer func() {
		c.RecordOperation(op, time.Since(start), itemCount)
	}()

	if err := c.CheckContext(ctx); err != nil {
		return err
	}

	if err := fn(); err != nil {
		c.RecordError(op)
		return err
	}
	return nil
}

// isExpired checks if an item has expired
func (c *memoryCache[T]) isExpired(item *memoryItem[T]) bool {
	return !item.expiration.IsZero() && time.Now().After(item.expiration)
}

// removeElement removes an element from the list and map (must hold write lock)
func (c *memoryCache[T]) removeElement(elem *list.Element) {
	c.lruList.Remove(elem)
	item := elem.Value.(*memoryItem[T])
	delete(c.items, item.key)
	atomic.AddInt64(&c.len, -1)
}

// evict removes the least recently used item (must hold write lock)
func (c *memoryCache[T]) evict() {
	if elem := c.lruList.Back(); elem != nil {
		c.removeElement(elem)
	}
}

// Get retrieves a value from the cache
func (c *memoryCache[T]) Get(ctx context.Context, key string) (T, error) {
	var val T

	err := c.execute(ctx, "get", 1, func() error {
		if err := c.ValidateKey(key); err != nil {
			return err
		}

		fullKey := c.FullKey(key)

		// Try read lock first
		c.mu.RLock()
		elem, ok := c.items[fullKey]
		if !ok {
			c.mu.RUnlock()
			c.RecordMiss("get", 1)
			return base.ErrCacheMiss
		}

		item := elem.Value.(*memoryItem[T])

		// Check expiration
		if c.isExpired(item) {
			c.mu.RUnlock()

			// Upgrade to write lock to remove
			c.mu.Lock()
			c.removeElement(elem)
			c.mu.Unlock()

			c.RecordMiss("get", 1)
			return base.ErrCacheMiss
		}
		c.mu.RUnlock()

		// Promote to front for LRU (needs write lock)
		c.mu.Lock()
		c.lruList.MoveToFront(elem)
		c.mu.Unlock()

		val = item.value
		c.RecordHit("get", 1)
		return nil
	})

	return val, err
}

// Set stores a value in the cache
func (c *memoryCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	return c.execute(ctx, "set", 1, func() error {
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

		// Update existing item
		if elem, ok := c.items[fullKey]; ok {
			item := elem.Value.(*memoryItem[T])
			item.value = value
			item.expiration = expiration
			c.lruList.MoveToFront(elem)
			return nil
		}

		// Evict if at capacity
		if c.capacity > 0 && int(atomic.LoadInt64(&c.len)) >= c.capacity {
			c.evict()
		}

		// Add new item
		item := &memoryItem[T]{
			key:        fullKey,
			value:      value,
			expiration: expiration,
		}
		elem := c.lruList.PushFront(item)
		c.items[fullKey] = elem
		atomic.AddInt64(&c.len, 1)

		return nil
	})
}

// Delete removes keys from the cache
func (c *memoryCache[T]) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	return c.execute(ctx, "delete", len(keys), func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		for _, k := range keys {
			fk := c.FullKey(k)
			if elem, ok := c.items[fk]; ok {
				c.removeElement(elem)
			}
		}
		return nil
	})
}

// Exists checks if a key exists in the cache
func (c *memoryCache[T]) Exists(ctx context.Context, key string) (bool, error) {
	var exists bool

	err := c.execute(ctx, "exists", 1, func() error {
		if err := c.ValidateKey(key); err != nil {
			return err
		}

		fk := c.FullKey(key)

		c.mu.Lock()
		defer c.mu.Unlock()

		elem, ok := c.items[fk]
		if !ok {
			exists = false
			return nil
		}

		item := elem.Value.(*memoryItem[T])
		if c.isExpired(item) {
			c.removeElement(elem)
			exists = false
			return nil
		}

		exists = true
		return nil
	})

	return exists, err
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
	return c.execute(ctx, "clear", 1, func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		c.items = make(map[string]*list.Element)
		c.lruList.Init()
		atomic.StoreInt64(&c.len, 0)
		return nil
	})
}

// Len returns the number of items in the cache
func (c *memoryCache[T]) Len(ctx context.Context) (int, error) {
	if err := c.CheckContext(ctx); err != nil {
		return 0, err
	}
	return int(atomic.LoadInt64(&c.len)), nil
}

// DeleteByPrefix removes all keys with the given prefix
func (c *memoryCache[T]) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	var deleted int64

	err := c.execute(ctx, "delete_by_prefix", 1, func() error {
		fullPrefix := c.FullKey(prefix)

		c.mu.Lock()
		defer c.mu.Unlock()

		for k, elem := range c.items {
			if strings.HasPrefix(k, fullPrefix) {
				c.removeElement(elem)
				deleted++
			}
		}
		return nil
	})

	return deleted, err
}

// Stats returns cache statistics
func (c *memoryCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	items, _ := c.Len(ctx)

	snapshot := c.Metrics.Snapshot()
	var hits, misses int64

	for _, stat := range snapshot {
		hits += stat.Hits
		misses += stat.Misses
	}

	return metrics.CacheStats{
		Backend:         "memory",
		Items:           int64(items),
		Hits:            hits,
		Misses:          misses,
		HitRate:         metrics.CalculateHitRate(hits, misses),
		Uptime:          c.Uptime(),
		RefreshTTLOnHit: c.Cfg.RefreshTTLOnHit,
	}
}

// startCleanup runs the background cleanup goroutine
func (c *memoryCache[T]) startCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		}
	}
}

// deleteExpired removes expired items
func (c *memoryCache[T]) deleteExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, elem := range c.items {
		item := elem.Value.(*memoryItem[T])
		if !item.expiration.IsZero() && now.After(item.expiration) {
			c.removeElement(elem)
		}
	}
}
