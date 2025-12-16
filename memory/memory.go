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

/* ------------------ Types ------------------ */

type memoryItem[T any] struct {
	key       string
	value     T
	expiresAt time.Time
}

type memoryCache[T any] struct {
	base     *base.Base
	items    map[string]*list.Element
	lru      *list.List
	mu       sync.RWMutex
	stopCh   chan struct{}
	capacity int
	length   int64
}

/* ------------------ Constructor ------------------ */

func NewMemory[T any](cfg config.Config) (*memoryCache[T], error) {
	if cfg.EvictionPolicy != "" && cfg.EvictionPolicy != config.EvictLRU {
		return nil, base.WrapError(base.OpSet, base.ErrInvalidConfig, "")
	}

	mc := &memoryCache[T]{
		base:     base.NewBase(cfg),
		items:    make(map[string]*list.Element),
		lru:      list.New(),
		stopCh:   make(chan struct{}),
		capacity: cfg.MaxSize,
	}

	if cfg.CleanupInterval > 0 {
		go mc.cleanupLoop(context.Background(), cfg.CleanupInterval)
	}

	return mc, nil
}

/* ------------------ Helpers ------------------ */

func (c *memoryCache[T]) expired(it *memoryItem[T]) bool {
	return !it.expiresAt.IsZero() && time.Now().After(it.expiresAt)
}

func (c *memoryCache[T]) remove(elem *list.Element) {
	c.lru.Remove(elem)
	item := elem.Value.(*memoryItem[T])
	delete(c.items, item.key)
	atomic.AddInt64(&c.length, -1)
}

func (c *memoryCache[T]) evict() {
	if e := c.lru.Back(); e != nil {
		c.remove(e)
	}
}

/* ------------------ Cache API ------------------ */

func (c *memoryCache[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T

	if err := c.base.ValidateKey(key); err != nil {
		return zero, err
	}
	if err := c.base.CheckContext(ctx); err != nil {
		return zero, err
	}

	fk := c.base.FullKey(key)

	c.mu.RLock()
	elem, ok := c.items[fk]
	if !ok {
		c.mu.RUnlock()
		return zero, base.WrapError(base.OpGet, base.ErrCacheMiss, key)
	}

	item := elem.Value.(*memoryItem[T])
	if c.expired(item) {
		c.mu.RUnlock()
		c.mu.Lock()
		c.remove(elem)
		c.mu.Unlock()
		return zero, base.WrapError(base.OpGet, base.ErrCacheMiss, key)
	}
	c.mu.RUnlock()

	c.mu.Lock()
	c.lru.MoveToFront(elem)
	c.mu.Unlock()

	return item.value, nil
}

func (c *memoryCache[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	if err := c.base.ValidateKey(key); err != nil {
		return err
	}
	if err := c.base.CheckContext(ctx); err != nil {
		return err
	}

	fk := c.base.FullKey(key)
	ttl = c.base.ResolveTTL(ttl)

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[fk]; ok {
		it := elem.Value.(*memoryItem[T])
		it.value = value
		it.expiresAt = expiresAt
		c.lru.MoveToFront(elem)
		return nil
	}

	if c.capacity > 0 && int(atomic.LoadInt64(&c.length)) >= c.capacity {
		c.evict()
	}

	it := &memoryItem[T]{key: fk, value: value, expiresAt: expiresAt}
	elem := c.lru.PushFront(it)
	c.items[fk] = elem
	atomic.AddInt64(&c.length, 1)

	return nil
}

func (c *memoryCache[T]) Delete(ctx context.Context, keys ...string) error {
	if err := c.base.CheckContext(ctx); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, k := range keys {
		fk := c.base.FullKey(k)
		if e, ok := c.items[fk]; ok {
			c.remove(e)
		}
	}
	return nil
}

func (c *memoryCache[T]) Exists(_ context.Context, key string) (bool, error) {
	if err := c.base.ValidateKey(key); err != nil {
		return false, err
	}

	fk := c.base.FullKey(key)

	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[fk]
	if !ok {
		return false, nil
	}

	it := elem.Value.(*memoryItem[T])
	if c.expired(it) {
		c.remove(elem)
		return false, nil
	}
	return true, nil
}

func (c *memoryCache[T]) Clear(ctx context.Context) error {
	if err := c.base.CheckContext(ctx); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.lru.Init()
	atomic.StoreInt64(&c.length, 0)
	return nil
}

func (c *memoryCache[T]) Len(ctx context.Context) (int, error) {
	if err := c.base.CheckContext(ctx); err != nil {
		return 0, err
	}
	return int(atomic.LoadInt64(&c.length)), nil
}

func (c *memoryCache[T]) DeleteByPrefix(_ context.Context, prefix string) (int64, error) {
	fp := c.base.FullKey(prefix)
	var n int64

	c.mu.Lock()
	defer c.mu.Unlock()

	for k, e := range c.items {
		if strings.HasPrefix(k, fp) {
			c.remove(e)
			n++
		}
	}
	return n, nil
}

func (c *memoryCache[T]) Close() error {
	close(c.stopCh)
	return nil
}

func (c *memoryCache[T]) Ping(ctx context.Context) error {
	return c.base.CheckContext(ctx)
}

/* ------------------ Stats ------------------ */

func (c *memoryCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	items, _ := c.Len(ctx)

	snap := c.base.Metrics().Snapshot()
	var hits, misses int64
	for _, s := range snap {
		hits += s.Hits
		misses += s.Misses
	}

	return metrics.CacheStats{
		Backend: "memory",
		Items:   int64(items),
		Hits:    hits,
		Misses:  misses,
		HitRate: metrics.CalculateHitRate(hits, misses),
		Uptime:  c.base.Uptime(),
	}
}

/* ------------------ Cleanup ------------------ */

func (c *memoryCache[T]) cleanupLoop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			c.deleteExpired()
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		}
	}
}

func (c *memoryCache[T]) deleteExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, e := range c.items {
		it := e.Value.(*memoryItem[T])
		if !it.expiresAt.IsZero() && now.After(it.expiresAt) {
			c.remove(e)
		}
	}
}
