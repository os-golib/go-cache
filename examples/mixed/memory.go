package main

import (
	"context"
	"fmt"
	"time"

	"github.com/os-golib/go-cache"
	"github.com/os-golib/go-cache/config"
)

func memory() {
	fmt.Println("===== Memory example starting =====")
	cfg := config.Defaults()
	cfg.Type = config.TypeMemory
	cfg.MaxSize = 100 // Limit to 100 items

	c, err := cache.NewAdvanced[string](cfg)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	ctx := context.Background()

	// Set a value
	err = c.Set(ctx, "key1", "value1", 5*time.Second)
	if err != nil {
		fmt.Println("Set error:", err)
	}

	// Get the value
	val, err := c.Get(ctx, "key1")
	if err == nil {
		fmt.Println("Got:", val) // Output: Got: value1
	} else {
		fmt.Println("Get error:", err)
	}

	// Check existence
	exists, _ := c.Exists(ctx, "key1")
	fmt.Println("Exists:", exists) // Output: Exists: true

	// Delete
	c.Delete(ctx, "key1")
	exists, _ = c.Exists(ctx, "key1")
	fmt.Println("Exists after delete:", exists) // Output: Exists after delete: false

	// Stats (if using AdvancedCache)
	// Note: For basic Cache, stats are limited; use Advanced for full metrics.
}
