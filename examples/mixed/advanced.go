package main

import (
	"context"
	"fmt"
	"time"

	"github.com/os-golib/go-cache"
	"github.com/os-golib/go-cache/config"
)

func advanced() {
	fmt.Println("===== Advanced example starting =====")

	cfg, _ := config.NewBuilder(config.TypeMemory).WithMemoryConfig(10000, time.Minute).Build()

	ac, err := cache.NewAdvanced[string](cfg)
	if err != nil {
		panic(err)
	}
	defer ac.Close()

	ctx := context.Background()

	// GetOrSet: Compute if missing
	val, err := ac.GetOrSet(ctx, "computed", 5*time.Second, func() (string, error) {
		return "expensive computation", nil
	})
	fmt.Println("Computed value:", val) // Output: Computed value: expensive computation

	// SetMany
	items := map[string]string{
		"fruit:apple":  "red",
		"fruit:banana": "yellow",
		"fruit:orange": "orange",
	}
	ac.SetMany(ctx, items, 0) // Use default TTL

	// GetMany
	keys := []string{"fruit:apple", "fruit:banana", "fruit:orange"}

	results, _ := ac.GetMany(ctx, keys)
	fmt.Printf("Results: %+v\n", results) // Output: Results: map[a:1 b:2]

	// Metrics
	metrics := ac.Metrics().Snapshot()
	for op, stats := range metrics {
		fmt.Printf("Op %s: Hits=%d, Misses=%d, Duration=%v\n", op, stats.Hits, stats.Misses, stats.TotalDuration)
	}
}
