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

// GORMCache provides GORM integration for caching database entities
type GORMCache[T any] struct {
	cache interfaces.AdvancedCache[T]
	db    *gorm.DB
	opts  GORMOptions
}

// GORMOptions configures GORM cache behavior
type GORMOptions struct {
	// DefaultTTL is the default cache TTL for entities
	DefaultTTL time.Duration
	// KeyPrefix is the prefix for cache keys
	KeyPrefix string
	// SkipCache controls whether to bypass cache entirely
	SkipCache bool
	// WarmCache enables cache warming on application start
	WarmCache bool
}

// DefaultGORMOptions returns default GORM cache options
func DefaultGORMOptions() GORMOptions {
	return GORMOptions{
		DefaultTTL: 10 * time.Minute,
		KeyPrefix:  "gorm",
		SkipCache:  false,
		WarmCache:  false,
	}
}

// NewGORMCache creates a new GORM cache instance
func NewGORMCache[T any](cache interfaces.AdvancedCache[T], db *gorm.DB, opts ...GORMOptions) *GORMCache[T] {
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
		go gc.warmCache(context.Background())
	}

	return gc
}

// GetByID retrieves an entity by ID, using cache if available
func (g *GORMCache[T]) GetByID(ctx context.Context, id any, ttl ...time.Duration) (T, error) {
	var zero T

	if g.opts.SkipCache {
		return g.loadFromDB(ctx, id)
	}

	cacheTTL := g.opts.DefaultTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		cacheTTL = ttl[0]
	}

	key := g.buildKey(id)

	entity, err := g.cache.GetOrSet(ctx, key, cacheTTL, func() (T, error) {
		return g.loadFromDB(ctx, id)
	})
	if err != nil {
		return zero, fmt.Errorf("gorm cache get: %w", err)
	}

	return entity, nil
}

// GetByIDs retrieves multiple entities by IDs with reduced cognitive complexity.
func (g *GORMCache[T]) GetByIDs(ctx context.Context, ids []any, ttl ...time.Duration) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}

	if g.opts.SkipCache {
		return g.loadMultipleFromDB(ctx, ids)
	}

	cacheTTL := g.opts.DefaultTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		cacheTTL = ttl[0]
	}

	// Build cache keys
	keys := make([]string, len(ids))
	idToKey := make(map[any]string)
	for in, id := range ids {
		key := g.buildKey(id)
		keys[in] = key
		idToKey[id] = key
	}

	cacheResults, err := g.cache.GetMany(ctx, keys)
	if err != nil {
		return g.loadMultipleFromDB(ctx, ids)
	}

	results, missingIDs := g.splitCacheHits(ids, idToKey, cacheResults)

	if len(missingIDs) == 0 {
		return results, nil
	}

	dbEntities, err := g.loadMultipleFromDB(ctx, missingIDs)
	if err != nil {
		return nil, err
	}

	go g.cacheEntities(ctx, missingIDs, dbEntities, cacheTTL)

	g.mergeDBResults(ids, missingIDs, dbEntities, results)

	return results, nil
}

// Preload preloads associations for an entity with caching
func (g *GORMCache[T]) Preload(ctx context.Context, id any, associations []string, ttl ...time.Duration) (T, error) {
	var zero T

	entity, err := g.GetByID(ctx, id, ttl...)
	if err != nil {
		return zero, err
	}

	// Create a new instance for preloading
	db := g.db.WithContext(ctx)
	for _, assoc := range associations {
		db = db.Preload(assoc)
	}

	err = db.First(&entity, id).Error
	if err != nil {
		return zero, err
	}

	// Update cache with preloaded entity
	go func(ctx context.Context) {
		cacheTTL := g.opts.DefaultTTL
		if len(ttl) > 0 && ttl[0] > 0 {
			cacheTTL = ttl[0]
		}
		_ = g.cache.Set(ctx, g.buildKey(id), entity, cacheTTL)
	}(ctx)

	return entity, nil
}

// Invalidate removes an entity from cache
func (g *GORMCache[T]) Invalidate(ctx context.Context, id any) error {
	key := g.buildKey(id)
	return g.cache.Delete(ctx, key)
}

// InvalidateByPattern removes entities matching a pattern from cache
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

	cacheTTL := g.opts.DefaultTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		cacheTTL = ttl[0]
	}

	return g.cache.Set(ctx, g.buildKey(id), entity, cacheTTL)
}

// Stats returns cache statistics
func (g *GORMCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	return g.cache.Stats(ctx)
}

// ========== PRIVATE METHODS ==========

// buildKey creates a cache key for an entity
func (g *GORMCache[T]) buildKey(id any) string {
	var t T
	rt := reflect.TypeOf(t)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	return fmt.Sprintf("%s:%s:%v", g.opts.KeyPrefix, rt.Name(), id)
}

// loadFromDB loads an entity from database
func (g *GORMCache[T]) loadFromDB(ctx context.Context, id any) (T, error) {
	var entity T
	err := g.db.WithContext(ctx).First(&entity, id).Error
	return entity, err
}

// loadMultipleFromDB loads multiple entities from database
func (g *GORMCache[T]) loadMultipleFromDB(ctx context.Context, ids []any) ([]T, error) {
	var entities []T
	err := g.db.WithContext(ctx).Find(&entities, ids).Error
	return entities, err
}

// cacheEntities caches multiple entities
func (g *GORMCache[T]) cacheEntities(ctx context.Context, ids []any, entities []T, ttl time.Duration) {
	items := make(map[string]T)
	for in, id := range ids {
		key := g.buildKey(id)
		items[key] = entities[in]
	}
	_ = g.cache.SetMany(ctx, items, ttl)
}

// warmCache preloads frequently accessed entities
func (g *GORMCache[T]) warmCache(ctx context.Context) {
	_ = g.cache.Set(ctx, g.buildKey(zeroValue[T]()), zeroValue[T](), g.opts.DefaultTTL)
}

// zeroValue returns the zero value for type T
func zeroValue[T any]() T {
	var zero T
	return zero
}

// equalIDs checks if two IDs are equal
func equalIDs(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// GORMHook integrates cache invalidation with GORM hooks
type GORMHook struct {
	cache *GORMCache[any]
}

// NewGORMHook creates a new GORM hook for cache invalidation
func NewGORMHook(cache *GORMCache[any]) *GORMHook {
	return &GORMHook{cache: cache}
}

// // Register registers the hook with GORM
// func (h *GORMHook) Register(db *gorm.DB) {
// 	_ = db.Callback().
// 		Create().
// 		After("gorm:after_create").
// 		Register("cache:invalidate_create", h.afterCreate)

// 	_ = db.Callback().
// 		Update().
// 		After("gorm:after_update").
// 		Register("cache:invalidate_update", h.afterUpdate)

// 	_ = db.Callback().
// 		Delete().
// 		After("gorm:after_delete").
// 		Register("cache:invalidate_delete", h.afterDelete)
// }

// func (h *GORMHook) afterCreate(db *gorm.DB) {
// 	if db.Error == nil {
// 		// Invalidate relevant cache entries
// 		// This would need to be customized based on your data model
// 	}
// }

// func (h *GORMHook) afterUpdate(db *gorm.DB) {
// 	if db.Error == nil {
// 		// Invalidate updated entity cache
// 		if model := db.Statement.Model; model != nil {
// 			// Extract ID and invalidate cache
// 			// Implementation depends on your model structure
// 		}
// 	}
// }

// func (h *GORMHook) afterDelete(db *gorm.DB) {
// 	if db.Error == nil {
// 		// Invalidate deleted entity cache
// 		if model := db.Statement.Model; model != nil {
// 			// Extract ID and invalidate cache
// 			// Implementation depends on your model structure
// 		}
// 	}
// }

func (g *GORMCache[T]) splitCacheHits(
	ids []any,
	idToKey map[any]string,
	cacheResults map[string]T,
) (results []T, missing []any) {
	results = make([]T, len(ids))

	for inx, id := range ids {
		key := idToKey[id]

		if val, ok := cacheResults[key]; ok {
			results[inx] = val
		} else {
			results[inx] = zeroValue[T]()
			missing = append(missing, id)
		}
	}

	return
}

func (g *GORMCache[T]) mergeDBResults(
	allIDs []any,
	missingIDs []any,
	dbEntities []T,
	results []T,
) {
	for inx, id := range allIDs {
		for j, missID := range missingIDs {
			if equalIDs(id, missID) {
				results[inx] = dbEntities[j]
				break
			}
		}
	}
}
