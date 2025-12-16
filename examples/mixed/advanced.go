package main

import (
	"context"
	"fmt"
	"time"

	"github.com/os-golib/go-cache"
)

func advanced() {
	fmt.Println("===== Advanced example starting =====")

	cfg, _ := cache.NewBuilder().WithMemory().Build()

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
	if err != nil {
		panic(err)
	}
	fmt.Println("Computed value:", val) // Output: Computed value: expensive computation

	// SetMany
	items := map[string]string{
		"fruit:apple":  "red",
		"fruit:banana": "yellow",
		"fruit:orange": "orange",
	}
	ac.SetManyPipeline(ctx, items, 0) // Use default TTL

	// GetMany
	keys := []string{"fruit:apple", "fruit:banana", "fruit:orange"}

	results, _ := ac.GetManyPipeline(ctx, keys)
	fmt.Printf("Results: %+v\n", results) // Output: Results: map[a:1 b:2]

	// Metrics
	metrics := ac.Metrics().Snapshot()
	for op, stats := range metrics {
		fmt.Printf("Op %s: Hits=%d, Misses=%d, Duration=%v\n", op, stats.Hits, stats.Misses, stats.AvgDuration)
	}
}

func main() {
	advanced()
	myfasthttp()
}
