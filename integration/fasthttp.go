package integration

import (
	"bytes"
	"context"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/internal/interfaces"
)

// HTTPCacheMiddleware provides HTTP caching for fasthttp
type HTTPCacheMiddleware[T any] struct {
	cache      interfaces.AdvancedCache[T]
	keyGen     func(*fasthttp.RequestCtx) string
	shouldSkip func(*fasthttp.RequestCtx) bool
	ttl        time.Duration
	serializer config.Serializer[T]
	timeout    time.Duration
	opts       HTTPCacheOptions
}

// HTTPCacheOptions configures HTTP cache behavior
type HTTPCacheOptions struct {
	// Timeout for cache operations
	Timeout time.Duration
	// SkipMethods HTTP methods to skip caching for
	SkipMethods []string
	// CacheableStatuses HTTP status codes to cache
	CacheableStatuses []int
	// VaryHeaders headers that affect cache key
	VaryHeaders []string
	// BypassHeader header that forces cache bypass
	BypassHeader string
}

// DefaultHTTPCacheOptions returns default HTTP cache options
func DefaultHTTPCacheOptions() HTTPCacheOptions {
	return HTTPCacheOptions{
		Timeout:           3 * time.Second,
		SkipMethods:       []string{"POST", "PUT", "PATCH", "DELETE"},
		CacheableStatuses: []int{200, 203, 204, 206, 300, 301, 308, 404, 405, 410, 414, 501},
		VaryHeaders:       []string{"Accept", "Accept-Encoding", "Authorization"},
		BypassHeader:      "X-Cache-Bypass",
	}
}

// NewHTTPCache creates a new HTTP cache middleware
func NewHTTPCache[T any](
	acache interfaces.AdvancedCache[T],
	ttl time.Duration,
	opts ...HTTPCacheOptions,
) *HTTPCacheMiddleware[T] {
	options := DefaultHTTPCacheOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	skipMethods := make(map[string]bool)
	for _, method := range options.SkipMethods {
		skipMethods[method] = true
	}

	cacheableStatuses := make(map[int]bool)
	for _, status := range options.CacheableStatuses {
		cacheableStatuses[status] = true
	}

	return &HTTPCacheMiddleware[T]{
		cache:      acache,
		ttl:        ttl,
		timeout:    options.Timeout,
		serializer: &config.JsonSerializer[T]{},
		keyGen:     defaultKeyGenerator(options.VaryHeaders),
		shouldSkip: defaultSkipChecker(skipMethods, cacheableStatuses, options.BypassHeader),
		opts:       options,
	}
}

// Handler returns the fasthttp request handler
func (m *HTTPCacheMiddleware[T]) Handler(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		// Check if we should skip caching for this request
		if m.shouldSkip(ctx) {
			next(ctx)
			return
		}

		ctx.Response.Header.Set("Server", "sws")

		// Generate cache key
		key := m.keyGen(ctx)

		// Try to get from cache with timeout
		cctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()

		if cached, err := m.cache.Get(cctx, key); err == nil {
			m.serveFromCache(ctx, cached)
			return
		}

		// Cache miss - execute the handler
		m.serveMiss(ctx)
		next(ctx)

		// Cache the response if appropriate
		m.cacheResponse(ctx, key)
	}
}

// WithKeyGenerator sets a custom key generator
func (m *HTTPCacheMiddleware[T]) WithKeyGenerator(fn func(*fasthttp.RequestCtx) string) *HTTPCacheMiddleware[T] {
	if fn != nil {
		m.keyGen = fn
	}
	return m
}

// WithSkipCondition sets a custom skip condition
func (m *HTTPCacheMiddleware[T]) WithSkipCondition(fn func(*fasthttp.RequestCtx) bool) *HTTPCacheMiddleware[T] {
	if fn != nil {
		m.shouldSkip = fn
	}
	return m
}

// WithSerializer sets a custom serializer
func (m *HTTPCacheMiddleware[T]) WithSerializer(serializer config.Serializer[T]) *HTTPCacheMiddleware[T] {
	if serializer != nil {
		m.serializer = serializer
	}
	return m
}

// WithTimeout sets a custom timeout
func (m *HTTPCacheMiddleware[T]) WithTimeout(timeout time.Duration) *HTTPCacheMiddleware[T] {
	if timeout > 0 {
		m.timeout = timeout
	}
	return m
}

// ========== PRIVATE METHODS ==========

// serveFromCache serves a cached response
func (m *HTTPCacheMiddleware[T]) serveFromCache(ctx *fasthttp.RequestCtx, cached T) {
	body, err := m.serializer.Encode(cached)
	if err != nil {
		// If serialization fails, treat as cache miss and continue
		m.serveMiss(ctx)
		return
	}

	ctx.Response.Header.Set("X-Cache", "HIT")
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
	ctx.Response.SetBody(body)

	// Set appropriate content type
	ctx.Response.Header.SetContentType("application/json; charset=utf-8")
}

// serveMiss indicates a cache miss
func (m *HTTPCacheMiddleware[T]) serveMiss(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("X-Cache", "MISS")
}

// cacheResponse caches the response if appropriate
func (m *HTTPCacheMiddleware[T]) cacheResponse(ctx *fasthttp.RequestCtx, key string) {
	// Only cache successful responses
	if !m.isCacheableStatus(ctx.Response.StatusCode()) {
		return
	}
	// Don't cache if response indicates no-cache
	cc := ctx.Response.Header.Peek("Cache-Control")
	if len(cc) > 0 {
		if bytes.Contains(cc, []byte("no-cache")) ||
			bytes.Contains(cc, []byte("no-store")) {
			return
		}
	}

	var response T
	data, err := m.serializer.Decode(ctx.Response.Body())
	if err != nil {
		return
	}
	response = data

	// Cache the response asynchronously to not block the request
	go safeExec("http_cache_set", func() error {
		return m.cache.Set(context.Background(), key, response, m.ttl)
	})
}

// isCacheableStatus checks if a status code is cacheable
func (m *HTTPCacheMiddleware[T]) isCacheableStatus(statusCode int) bool {
	for _, code := range m.opts.CacheableStatuses {
		if code == statusCode {
			return true
		}
	}
	return false
}

// defaultKeyGenerator creates the default cache key generator
func defaultKeyGenerator(varyHeaders []string) func(*fasthttp.RequestCtx) string {
	return func(ctx *fasthttp.RequestCtx) string {
		key := string(ctx.Method()) + ":" + string(ctx.Path())

		// Include query string
		if query := ctx.QueryArgs().QueryString(); len(query) > 0 {
			key += "?" + string(query)
		}

		// Include vary headers
		for _, header := range varyHeaders {
			if value := ctx.Request.Header.Peek(header); len(value) > 0 {
				key += "|" + header + ":" + string(value)
			}
		}

		return key
	}
}

// defaultSkipChecker creates the default skip condition checker
func defaultSkipChecker(skipMethods map[string]bool, cacheableStatuses map[int]bool, bypassHeader string) func(*fasthttp.RequestCtx) bool {
	return func(ctx *fasthttp.RequestCtx) bool {
		// Skip non-cacheable methods
		if skipMethods[string(ctx.Method())] { // Fixed: Inline string conversion
			return true
		}
		// Skip if bypass header is present
		if bypassHeader != "" && ctx.Request.Header.Peek(bypassHeader) != nil {
			return true
		}

		// If response status is not cacheable → skip caching
		status := ctx.Response.StatusCode()
		if !cacheableStatuses[status] {
			return true
		}

		// Skip if explicitly no-store
		if cc := ctx.Request.Header.Peek("Cache-Control"); string(cc) == "no-store" {
			return true
		}

		return false
	}
}

// CacheControl sets Cache-Control headers
func CacheControl(maxAge time.Duration, public bool) func(fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			next(ctx)

			var directive string
			if public {
				directive = "public"
			} else {
				directive = "private"
			}

			ctx.Response.Header.Set("Cache-Control",
				directive+", max-age="+string(rune(maxAge.Seconds())))
		}
	}
}

// ETag generates and validates ETags
func ETag(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		next(ctx)

		// Generate ETag from response body
		body := ctx.Response.Body()
		if len(body) > 0 {
			etag := fasthttp.AppendQuotedArg(nil, body)
			ctx.Response.Header.SetBytesV("ETag", etag)

			// Check If-None-Match
			if match := ctx.Request.Header.Peek("If-None-Match"); len(match) > 0 {
				if bytes.Equal(match, etag) {
					ctx.SetStatusCode(fasthttp.StatusNotModified)
					ctx.SetBody(nil)
				}
			}
		}
	}
}
