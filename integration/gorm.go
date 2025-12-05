package integration

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"gorm.io/gorm"
	
	"github.com/os-golib/go-cache/internal/interfaces"
	"github.com/os-golib/go-cache/internal/metrics"
)

// GORMOptions configures GORM cache behavior
type GORMOptions struct {
	DefaultTTL time.Duration
	KeyPrefix  string
	SkipCache  bool
	WarmCache  bool
}

// DefaultGORMOptions returns default options
func DefaultGORMOptions() GORMOptions {
	return GORMOptions{
		DefaultTTL: 10 * time.Minute,
		KeyPrefix:  "gorm",
		SkipCache:  false,
		WarmCache:  false,
	}
}

// GORMCache provides GORM integration
type GORMCache[T any] struct {
	cache interfaces.AdvancedCache[T]
	db    *gorm.DB
	opts  GORMOptions
}

// NewGORMCache creates a new GORM cache instance
func NewGORMCache[T any](
	cache interfaces.AdvancedCache[T],
	db *gorm.DB,
	opts ...GORMOptions,
) *GORMCache[T] {
	options := DefaultGORMOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	gc := &GORMCache[T]{
		cache: cache,
		db:    db,
		opts:  options,
	}

	if options.WarmCache {
		executeAsync("gorm_warm_cache", func() error {
			return gc.warmCache(context.Background())
		})
	}

	return gc
}

// GetByID retrieves an entity by ID with caching
func (g *GORMCache[T]) GetByID(ctx context.Context, id any, ttl ...time.Duration) (T, error) {
	if g.opts.SkipCache {
		return g.loadFromDB(ctx, id)
	}

	cacheTTL := g.resolveTTL(ttl...)
	key := g.buildKey(id)

	return g.cache.GetOrSet(ctx, key, cacheTTL, func() (T, error) {
		return g.loadFromDB(ctx, id)
	})
}

// GetByIDs retrieves multiple entities by IDs with caching
func (g *GORMCache[T]) GetByIDs(ctx context.Context, ids []any, ttl ...time.Duration) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}

	if g.opts.SkipCache {
		return g.loadMultipleFromDB(ctx, ids)
	}

	cacheTTL := g.resolveTTL(ttl...)

	// Build cache keys and mapping
	keys := make([]string, len(ids))
	idToKey := make(map[any]string, len(ids))
	for i, id := range ids {
		key := g.buildKey(id)
		keys[i] = key
		idToKey[id] = key
	}

	// Try to get from cache
	cached, err := g.cache.GetMany(ctx, keys)
	if err != nil {
		return g.loadMultipleFromDB(ctx, ids)
	}

	// Split hits and misses
	results, missingIDs := g.splitResults(ids, idToKey, cached)

	// If all found in cache, return
	if len(missingIDs) == 0 {
		return results, nil
	}

	// Load missing from DB
	dbEntities, err := g.loadMultipleFromDB(ctx, missingIDs)
	if err != nil {
		return nil, fmt.Errorf("load from db: %w", err)
	}

	// Cache missing entities asynchronously
	executeAsync("gorm_cache_missing", func() error {
		return g.cacheEntities(ctx, missingIDs, dbEntities, cacheTTL)
	})

	// Merge results
	g.mergeResults(ids, missingIDs, dbEntities, results)

	return results, nil
}

// Preload preloads associations with caching
func (g *GORMCache[T]) Preload(ctx context.Context, id any, associations []string, ttl ...time.Duration) (T, error) {
	var entity T

	db := g.db.WithContext(ctx)
	for _, assoc := range associations {
		db = db.Preload(assoc)
	}

	if err := db.First(&entity, id).Error; err != nil {
		return entity, err
	}

	// Update cache asynchronously
	cacheTTL := g.resolveTTL(ttl...)
	executeAsync("gorm_preload_cache", func() error {
		return g.cache.Set(ctx, g.buildKey(id), entity, cacheTTL)
	})

	return entity, nil
}

// Invalidate removes an entity from cache
func (g *GORMCache[T]) Invalidate(ctx context.Context, id any) error {
	return g.cache.Delete(ctx, g.buildKey(id))
}

// InvalidateByPattern removes entities matching a pattern
func (g *GORMCache[T]) InvalidateByPattern(ctx context.Context, pattern string) (int64, error) {
	fullPattern := fmt.Sprintf("%s:*%s*", g.opts.KeyPrefix, pattern)
	return g.cache.DeleteByPrefix(ctx, fullPattern)
}

// Refresh refreshes an entity in cache from database
func (g *GORMCache[T]) Refresh(ctx context.Context, id any, ttl ...time.Duration) error {
	entity, err := g.loadFromDB(ctx, id)
	if err != nil {
		return err
	}

	cacheTTL := g.resolveTTL(ttl...)
	return g.cache.Set(ctx, g.buildKey(id), entity, cacheTTL)
}

// Stats returns cache statistics
func (g *GORMCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	return g.cache.Stats(ctx)
}

// Helper methods

func (g *GORMCache[T]) resolveTTL(ttl ...time.Duration) time.Duration {
	if len(ttl) > 0 && ttl[0] > 0 {
		return ttl[0]
	}
	return g.opts.DefaultTTL
}

func (g *GORMCache[T]) buildKey(id any) string {
	var t T
	rt := reflect.TypeOf(t)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	return fmt.Sprintf("%s:%s:%v", g.opts.KeyPrefix, rt.Name(), id)
}

func (g *GORMCache[T]) loadFromDB(ctx context.Context, id any) (T, error) {
	var entity T
	err := g.db.WithContext(ctx).First(&entity, id).Error
	return entity, err
}

func (g *GORMCache[T]) loadMultipleFromDB(ctx context.Context, ids []any) ([]T, error) {
	var entities []T
	err := g.db.WithContext(ctx).Find(&entities, ids).Error
	return entities, err
}

func (g *GORMCache[T]) cacheEntities(ctx context.Context, ids []any, entities []T, ttl time.Duration) error {
	items := make(map[string]T, len(ids))
	for i, id := range ids {
		items[g.buildKey(id)] = entities[i]
	}
	return g.cache.SetMany(ctx, items, ttl)
}

func (g *GORMCache[T]) warmCache(ctx context.Context) error {
	var zero T
	return g.cache.Set(ctx, g.buildKey(zero), zero, g.opts.DefaultTTL)
}

func (g *GORMCache[T]) splitResults(
	ids []any,
	idToKey map[any]string,
	cached map[string]T,
) (results []T, missing []any) {
	results = make([]T, len(ids))
	
	for i, id := range ids {
		key := idToKey[id]
		if val, ok := cached[key]; ok {
			results[i] = val
		} else {
			var zero T
			results[i] = zero
			missing = append(missing, id)
		}
	}
	
	return results, missing
}

func (g *GORMCache[T]) mergeResults(
	allIDs []any,
	missingIDs []any,
	dbEntities []T,
	results []T,
) {
	for i, id := range allIDs {
		for j, missID := range missingIDs {
			if reflect.DeepEqual(id, missID) {
				results[i] = dbEntities[j]
				break
			}
		}
	}
}