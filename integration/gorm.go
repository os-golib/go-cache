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

/* ------------------ Options ------------------ */

type GORMOptions struct {
	DefaultTTL time.Duration
	KeyPrefix  string
	SkipCache  bool
	WarmCache  bool
}

func DefaultGORMOptions() GORMOptions {
	return GORMOptions{
		DefaultTTL: 10 * time.Minute,
		KeyPrefix:  "gorm",
		SkipCache:  false,
		WarmCache:  false,
	}
}

/* ------------------ Cache Wrapper ------------------ */

type GORMCache[T any] struct {
	cache    interfaces.AdvancedCache[T]
	db       *gorm.DB
	opts     GORMOptions
	typeName string
}

/* ------------------ Constructor ------------------ */

func NewGORMCache[T any](
	cache interfaces.AdvancedCache[T],
	db *gorm.DB,
	opts ...GORMOptions,
) *GORMCache[T] {
	options := DefaultGORMOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	var t T
	rt := reflect.TypeOf(t)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	return &GORMCache[T]{
		cache:    cache,
		db:       db,
		opts:     options,
		typeName: rt.Name(),
	}
}

/* ------------------ Single Entity ------------------ */

func (g *GORMCache[T]) GetByID(
	ctx context.Context,
	id any,
	ttl ...time.Duration,
) (T, error) {
	if g.opts.SkipCache {
		return g.loadFromDB(ctx, id)
	}

	key := g.buildKey(id)
	cacheTTL := g.resolveTTL(ttl...)

	val, err := g.cache.GetOrSet(ctx, key, cacheTTL, func() (T, error) {
		return g.loadFromDB(ctx, id)
	})
	if err != nil {
		// Fail open
		return g.loadFromDB(ctx, id)
	}
	return val, nil
}

/* ------------------ Multiple Entities ------------------ */

func (g *GORMCache[T]) GetByIDs(
	ctx context.Context,
	ids []any,
	ttl ...time.Duration,
) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}

	if g.opts.SkipCache {
		return g.loadMultipleFromDB(ctx, ids)
	}

	cacheTTL := g.resolveTTL(ttl...)

	keys := make([]string, len(ids))
	keyToIndex := make(map[string]int, len(ids))
	for i, id := range ids {
		key := g.buildKey(id)
		keys[i] = key
		keyToIndex[key] = i
	}

	results := make([]T, len(ids))
	missing := make([]any, 0)

	cached, err := g.cache.GetManyPipeline(ctx, keys)
	if err != nil {
		return g.loadMultipleFromDB(ctx, ids)
	}

	for i, id := range ids {
		key := keys[i]
		if val, ok := cached[key]; ok {
			results[i] = val
		} else {
			missing = append(missing, id)
		}
	}

	if len(missing) == 0 {
		return results, nil
	}

	dbEntities, err := g.loadMultipleFromDB(ctx, missing)
	if err != nil {
		return nil, err
	}

	// Async cache fill (best effort)
	go func(parent context.Context) {
		ctx, cancel := detachWithTimeout(parent, g.opts.DefaultTTL)
		defer cancel()

		_ = g.cacheEntities(ctx, missing, dbEntities, cacheTTL)
	}(ctx)

	// Merge in O(n)
	for i, id := range ids {
		for j, mid := range missing {
			if reflect.DeepEqual(id, mid) {
				results[i] = dbEntities[j]
				break
			}
		}
	}

	return results, nil
}

/* ------------------ Preload ------------------ */

func (g *GORMCache[T]) Preload(
	ctx context.Context,
	id any,
	associations []string,
	ttl ...time.Duration,
) (T, error) {
	var entity T
	db := g.db.WithContext(ctx)

	for _, a := range associations {
		db = db.Preload(a)
	}

	if err := db.First(&entity, id).Error; err != nil {
		return entity, err
	}

	cacheTTL := g.resolveTTL(ttl...)

	go func(parent context.Context) {
		ctx, cancel := detachWithTimeout(parent, g.opts.DefaultTTL)
		defer cancel()

		_ = g.cache.Set(ctx, g.buildKey(id), entity, cacheTTL)
	}(ctx)

	return entity, nil
}

/* ------------------ Invalidation ------------------ */

func (g *GORMCache[T]) Invalidate(ctx context.Context, id any) error {
	return g.cache.Delete(ctx, g.buildKey(id))
}

func (g *GORMCache[T]) InvalidateByPrefix(ctx context.Context, prefix string) (int64, error) {
	return g.cache.DeleteByPrefix(ctx, prefix)
}

/* ------------------ Refresh ------------------ */

func (g *GORMCache[T]) Refresh(
	ctx context.Context,
	id any,
	ttl ...time.Duration,
) error {
	entity, err := g.loadFromDB(ctx, id)
	if err != nil {
		return err
	}

	return g.cache.Set(ctx, g.buildKey(id), entity, g.resolveTTL(ttl...))
}

/* ------------------ Stats ------------------ */

func (g *GORMCache[T]) Stats(ctx context.Context) metrics.CacheStats {
	return g.cache.Stats(ctx)
}

/* ------------------ Helpers ------------------ */

func detachWithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, d)
}

func (g *GORMCache[T]) resolveTTL(ttl ...time.Duration) time.Duration {
	if len(ttl) > 0 && ttl[0] > 0 {
		return ttl[0]
	}
	return g.opts.DefaultTTL
}

func (g *GORMCache[T]) buildKey(id any) string {
	return fmt.Sprintf("%s:%s:%v", g.opts.KeyPrefix, g.typeName, id)
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

func (g *GORMCache[T]) cacheEntities(
	ctx context.Context,
	ids []any,
	entities []T,
	ttl time.Duration,
) error {
	items := make(map[string]T, len(ids))
	for i, id := range ids {
		items[g.buildKey(id)] = entities[i]
	}
	return g.cache.SetManyPipeline(ctx, items, ttl)
}
