package main

import (
	"context"
	"fmt"
	"time"

	"github.com/os-golib/go-cache"
	"github.com/os-golib/go-cache/config"
)

func main() {
	ctx := context.Background()

	cfg := config.Config{
		Type:            config.TypeRedis,
		RedisURL:        "redis://localhost:6379/0",
		TTL:             5 * time.Second,
		Prefix:          "demo:",
		PoolSize:        20,
		MinIdleConn:     5,
		MaxRetries:      3,
		RefreshTTLOnHit: true,
	}

	// Use ADVANCED cache if you want Stats(), Pipelines, etc.
	adv, err := cache.NewAdvanced[string](cfg)
	if err != nil {
		panic(err)
	}

	// Set value
	if err := adv.Set(ctx, "message", "Hello Redis Cache!", 0); err != nil {
		panic(err)
	}

	// Get value
	msg, err := adv.Get(ctx, "message")
	if err != nil {
		panic(err)
	}
	fmt.Println("GET message =>", msg)

	// Stats available ONLY on advanced cache
	stats := adv.Stats(ctx)
	fmt.Printf("Stats: %+v\n", stats)

	// Clear prefix
	adv.DeleteByPrefix(ctx, "")
}
