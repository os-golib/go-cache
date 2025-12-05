package main

import (
	"fmt"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/os-golib/go-cache"
	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/integration"
)

func myfasthttp() {
	fmt.Println("===== FastHTTP example starting =====")
	cfg := config.Defaults()
	cfg.Type = config.TypeRedis
	cfg.RedisURL = "redis://localhost:6379"

	ac, err := cache.NewAdvanced[[]byte](cfg)
	if err != nil {
		panic(err)
	}
	defer ac.Close()

	mw := integration.NewHTTPCache[[]byte](ac, 30*time.Second)
	mw.
		WithSerializer(config.BinarySerializer{}).
		WithKeyGenerator(func(ctx *fasthttp.RequestCtx) string {
			q := ctx.QueryArgs().String()
			if q == "" {
				return fmt.Sprintf("%s:%s", ctx.Method(), ctx.Path())
			}
			return fmt.Sprintf("%s:%s?%s", ctx.Method(), ctx.Path(), q)
		}).
		WithSkipCondition(func(ctx *fasthttp.RequestCtx) bool {
			return ctx.IsPost()
		})

	handler := func(ctx *fasthttp.RequestCtx) {
		ctx.SetContentType("text/plain")
		ctx.SetStatusCode(200)
		ctx.Response.SetBodyString("Hello, cached world!")
	}

	cachedHandler := mw.Handler(handler)

	fmt.Println("Server running on :8080")
	fasthttp.ListenAndServe(":8080", cachedHandler)
}
