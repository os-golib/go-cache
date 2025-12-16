package integration

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/os-golib/go-cache/internal/base"
	"github.com/os-golib/go-cache/internal/interfaces"
)

/* ------------------ Defaults ------------------ */

const DefaultTimeout = 500 * time.Millisecond

/* ------------------ Options ------------------ */

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

/* ------------------ Middleware ------------------ */

type HTTPCacheMiddleware[T any] struct {
	cache      interfaces.AdvancedCache[T]
	serializer base.Serializer[T]
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
		serializer: &base.JsonSerializer[T]{},
	}

	m.keyGen = m.defaultKeyGenerator()
	m.shouldSkip = m.defaultSkipChecker()

	return m
}

/* ------------------ Fluent Config ------------------ */

func (m *HTTPCacheMiddleware[T]) WithKeyGenerator(
	fn func(*fasthttp.RequestCtx) string,
) *HTTPCacheMiddleware[T] {
	if fn != nil {
		m.keyGen = fn
	}
	return m
}

func (m *HTTPCacheMiddleware[T]) WithSkipCondition(
	fn func(*fasthttp.RequestCtx) bool,
) *HTTPCacheMiddleware[T] {
	if fn != nil {
		m.shouldSkip = fn
	}
	return m
}

func (m *HTTPCacheMiddleware[T]) WithSerializer(
	serializer base.Serializer[T],
) *HTTPCacheMiddleware[T] {
	if serializer != nil {
		m.serializer = serializer
	}
	return m
}

func (m *HTTPCacheMiddleware[T]) WithTimeout(
	timeout time.Duration,
) *HTTPCacheMiddleware[T] {
	if timeout > 0 {
		m.opts.Timeout = timeout
	}
	return m
}

/* ------------------ Handler ------------------ */

func (m *HTTPCacheMiddleware[T]) Handler(
	next fasthttp.RequestHandler,
) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		if m.shouldSkip(ctx) {
			next(ctx)
			return
		}

		key := m.keyGen(ctx)
		if key == "" {
			next(ctx)
			return
		}

		// Cache lookup context
		cctx, cancel := context.WithTimeout(context.Background(), m.opts.Timeout)
		defer cancel()

		cached, err := m.cache.Get(cctx, key)
		if err == nil {
			m.serveFromCache(ctx, cached)
			return
		}
		if !base.IsCacheMiss(err) {
			// Cache error → fail open
			next(ctx)
			return
		}

		// Cache miss
		ctx.Response.Header.Set("X-Cache", "MISS")
		next(ctx)

		// Async cache write
		if m.isCacheableResponse(ctx) {
			body := append([]byte(nil), ctx.Response.Body()...)
			go func() {
				_ = m.cacheResponse(key, body)
			}()
		}
	}
}

/* ------------------ Cache Helpers ------------------ */

func (m *HTTPCacheMiddleware[T]) serveFromCache(
	ctx *fasthttp.RequestCtx,
	cached T,
) {
	body, err := m.serializer.Encode(cached)
	if err != nil {
		ctx.Response.Header.Set("X-Cache", "MISS")
		return
	}

	ctx.Response.ResetBody()
	ctx.Response.SetBody(body)
	ctx.Response.Header.Set("X-Cache", "HIT")
	ctx.Response.Header.SetContentType("application/json; charset=utf-8")
}

func (m *HTTPCacheMiddleware[T]) cacheResponse(
	key string,
	body []byte,
) error {
	resp, err := m.serializer.Decode(body)
	if err != nil {
		return err
	}

	return m.cache.Set(context.Background(), key, resp, m.ttl)
}

/* ------------------ Cacheability ------------------ */

func (m *HTTPCacheMiddleware[T]) isCacheableResponse(
	ctx *fasthttp.RequestCtx,
) bool {
	if !m.isCacheableStatus(ctx.Response.StatusCode()) {
		return false
	}

	cc := ctx.Response.Header.Peek("Cache-Control")
	if bytes.Contains(cc, []byte("no-cache")) ||
		bytes.Contains(cc, []byte("no-store")) {
		return false
	}

	return true
}

func (m *HTTPCacheMiddleware[T]) isCacheableStatus(
	status int,
) bool {
	for _, code := range m.opts.CacheableStatuses {
		if code == status {
			return true
		}
	}
	return false
}

/* ------------------ Key / Skip Logic ------------------ */

func (m *HTTPCacheMiddleware[T]) defaultKeyGenerator() func(*fasthttp.RequestCtx) string {
	return func(ctx *fasthttp.RequestCtx) string {
		var key strings.Builder

		key.Write(ctx.Method())
		key.WriteByte(':')
		key.Write(ctx.Path())

		if q := ctx.QueryArgs().QueryString(); len(q) > 0 {
			key.WriteByte('?')
			key.Write(q)
		}

		for _, h := range m.opts.VaryHeaders {
			if v := ctx.Request.Header.Peek(h); len(v) > 0 {
				key.WriteByte('|')
				key.WriteString(h)
				key.WriteByte('=')
				key.Write(v)
			}
		}

		return key.String()
	}
}

func (m *HTTPCacheMiddleware[T]) defaultSkipChecker() func(*fasthttp.RequestCtx) bool {
	skip := make(map[string]struct{}, len(m.opts.SkipMethods))
	for _, m := range m.opts.SkipMethods {
		skip[m] = struct{}{}
	}

	return func(ctx *fasthttp.RequestCtx) bool {
		if _, ok := skip[string(ctx.Method())]; ok {
			return true
		}

		if m.opts.BypassHeader != "" &&
			ctx.Request.Header.Peek(m.opts.BypassHeader) != nil {
			return true
		}

		if cc := ctx.Request.Header.Peek("Cache-Control"); bytes.Contains(cc, []byte("no-store")) {
			return true
		}

		return false
	}
}
