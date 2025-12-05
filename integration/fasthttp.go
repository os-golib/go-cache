package integration

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/interfaces"
)

// HTTPCacheOptions configures HTTP cache behavior
type HTTPCacheOptions struct {
	Timeout           time.Duration
	SkipMethods       []string
	CacheableStatuses []int
	VaryHeaders       []string
	BypassHeader      string
}

// DefaultHTTPCacheOptions returns default options
func DefaultHTTPCacheOptions() HTTPCacheOptions {
	return HTTPCacheOptions{
		Timeout:           DefaultTimeout,
		SkipMethods:       []string{"POST", "PUT", "PATCH", "DELETE"},
		CacheableStatuses: []int{200, 203, 204, 206, 300, 301, 308, 404, 405, 410, 414, 501},
		VaryHeaders:       []string{"Accept", "Accept-Encoding", "Authorization"},
		BypassHeader:      "X-Cache-Bypass",
	}
}

// HTTPCacheMiddleware provides HTTP caching
type HTTPCacheMiddleware[T any] struct {
	cache      interfaces.AdvancedCache[T]
	serializer config.Serializer[T]
	ttl        time.Duration
	opts       HTTPCacheOptions
	keyGen     func(*fasthttp.RequestCtx) string
	shouldSkip func(*fasthttp.RequestCtx) bool
}

// NewHTTPCache creates a new HTTP cache middleware
func NewHTTPCache[T any](
	cache interfaces.AdvancedCache[T],
	ttl time.Duration,
	opts ...HTTPCacheOptions,
) *HTTPCacheMiddleware[T] {
	options := DefaultHTTPCacheOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	m := &HTTPCacheMiddleware[T]{
		cache:      cache,
		ttl:        ttl,
		opts:       options,
		serializer: &config.JsonSerializer[T]{},
	}

	m.keyGen = m.defaultKeyGenerator()
	m.shouldSkip = m.defaultSkipChecker()

	return m
}

// Fluent configuration methods
func (m *HTTPCacheMiddleware[T]) WithKeyGenerator(fn func(*fasthttp.RequestCtx) string) *HTTPCacheMiddleware[T] {
	if fn != nil {
		m.keyGen = fn
	}
	return m
}

func (m *HTTPCacheMiddleware[T]) WithSkipCondition(fn func(*fasthttp.RequestCtx) bool) *HTTPCacheMiddleware[T] {
	if fn != nil {
		m.shouldSkip = fn
	}
	return m
}

func (m *HTTPCacheMiddleware[T]) WithSerializer(serializer config.Serializer[T]) *HTTPCacheMiddleware[T] {
	if serializer != nil {
		m.serializer = serializer
	}
	return m
}

func (m *HTTPCacheMiddleware[T]) WithTimeout(timeout time.Duration) *HTTPCacheMiddleware[T] {
	if timeout > 0 {
		m.opts.Timeout = timeout
	}
	return m
}

// Handler returns the fasthttp request handler
func (m *HTTPCacheMiddleware[T]) Handler(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if m.shouldSkip(ctx) {
			next(ctx)
			return
		}

		ctx.Response.Header.Set("Server", "sws")

		key := m.keyGen(ctx)

		// Try cache
		cctx, cancel := context.WithTimeout(context.Background(), m.opts.Timeout)
		defer cancel()

		if cached, err := m.cache.Get(cctx, key); err == nil {
			m.serveFromCache(ctx, cached)
			return
		}

		// Cache miss
		ctx.Response.Header.Set("X-Cache", "MISS")
		next(ctx)

		// Cache response asynchronously
		if m.isCacheableResponse(ctx) {
			executeAsync("http_cache_set", func() error {
				return m.cacheResponse(key, ctx.Response.Body())
			})
		}
	}
}

// serveFromCache serves a cached response
func (m *HTTPCacheMiddleware[T]) serveFromCache(ctx *fasthttp.RequestCtx, cached T) {
	body, err := m.serializer.Encode(cached)
	if err != nil {
		ctx.Response.Header.Set("X-Cache", "MISS")
		return
	}

	ctx.Response.Header.Set("X-Cache", "HIT")
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.Header.SetContentType("application/json; charset=utf-8")
	ctx.Response.SetBody(body)
}

// cacheResponse caches the response
func (m *HTTPCacheMiddleware[T]) cacheResponse(key string, body []byte) error {
	response, err := m.serializer.Decode(body)
	if err != nil {
		return err
	}

	return m.cache.Set(context.Background(), key, response, m.ttl)
}

// isCacheableResponse checks if response should be cached
func (m *HTTPCacheMiddleware[T]) isCacheableResponse(ctx *fasthttp.RequestCtx) bool {
	// Check status code
	if !m.isCacheableStatus(ctx.Response.StatusCode()) {
		return false
	}

	// Check Cache-Control header
	cc := ctx.Response.Header.Peek("Cache-Control")
	if bytes.Contains(cc, []byte("no-cache")) || bytes.Contains(cc, []byte("no-store")) {
		return false
	}

	return true
}

// isCacheableStatus checks if status code is cacheable
func (m *HTTPCacheMiddleware[T]) isCacheableStatus(status int) bool {
	for _, code := range m.opts.CacheableStatuses {
		if code == status {
			return true
		}
	}
	return false
}

// defaultKeyGenerator creates the default cache key generator
func (m *HTTPCacheMiddleware[T]) defaultKeyGenerator() func(*fasthttp.RequestCtx) string {
	return func(ctx *fasthttp.RequestCtx) string {
		var key strings.Builder

		key.WriteString(string(ctx.Method()))
		key.WriteByte(':')
		key.WriteString(string(ctx.Path()))

		if query := ctx.QueryArgs().QueryString(); len(query) > 0 {
			key.WriteByte('?')
			key.WriteString(string(query))
		}

		for _, header := range m.opts.VaryHeaders {
			if value := ctx.Request.Header.Peek(header); len(value) > 0 {
				key.WriteByte('|')
				key.WriteString(header)
				key.WriteByte(':')
				key.WriteString(string(value))
			}
		}

		return key.String()
	}
}

// defaultSkipChecker creates the default skip condition checker
func (m *HTTPCacheMiddleware[T]) defaultSkipChecker() func(*fasthttp.RequestCtx) bool {
	skipMethods := make(map[string]bool)
	for _, method := range m.opts.SkipMethods {
		skipMethods[method] = true
	}

	return func(ctx *fasthttp.RequestCtx) bool {
		// Skip non-cacheable methods
		if skipMethods[string(ctx.Method())] {
			return true
		}

		// Skip if bypass header present
		if m.opts.BypassHeader != "" && ctx.Request.Header.Peek(m.opts.BypassHeader) != nil {
			return true
		}

		// Skip if no-store in request
		if cc := ctx.Request.Header.Peek("Cache-Control"); bytes.Contains(cc, []byte("no-store")) {
			return true
		}

		return false
	}
}
