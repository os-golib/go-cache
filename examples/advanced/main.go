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
		URL:             "redis://localhost:6379/0",
		TTL:             10 * time.Second,
		Prefix:          "adv:",
		PoolSize:        10,
		MinIdleConn:     2,
		MaxRetries:      2,
		RefreshTTLOnHit: true,
	}

	adv, err := cache.NewAdvanced[string](cfg, config.Options[string]{})
	if err != nil {
		panic(err)
	}

	// -----------------------------
	// GetOrSet (cached value builder)
	// -----------------------------
	val, err := adv.GetOrSet(ctx, "computed_value", 30*time.Second, func() (string, error) {
		fmt.Println("Computing value...")
		time.Sleep(1 * time.Second)
		return "Expensive Result", nil
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("GetOrSet =>", val)

	// -----------------------------
	// GetOrSetLocked (distributed lock)
	// -----------------------------
	val2, err := adv.GetOrSetLocked(ctx, "locked_value", 30*time.Second, func() (string, error) {
		fmt.Println("Computed under distributed lock")
		return time.Now().Format(time.RFC3339), nil
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("GetOrSetLocked =>", val2)

	// -----------------------------
	// Bulk Set & Get with pipeline
	// -----------------------------
	items := map[string]string{
		"user:1": "Alice",
		"user:2": "Bob",
		"user:3": "Charlie",
	}

	if err := adv.SetManyPipeline(ctx, items, 20*time.Second); err != nil {
		panic(err)
	}

	result, err := adv.GetManyPipeline(ctx, []string{"user:1", "user:2", "user:3"})
	if err != nil {
		panic(err)
	}
	fmt.Println("Pipeline GET results =>", result)

	// -----------------------------
	// Delete by prefix
	// -----------------------------
	n, _ := adv.DeleteByPrefix(ctx, "user:")
	fmt.Println("Deleted count:", n)

	// -----------------------------
	// Stats
	// -----------------------------
	stats := adv.Stats(ctx)
	fmt.Printf("Stats: %+v\n", stats)
}
