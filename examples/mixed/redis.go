package main

import (
	"context"
	"fmt"
	"time"

	"github.com/os-golib/go-cache"
	"github.com/os-golib/go-cache/config"
)

func redis() {
	fmt.Println("===== Redis example starting =====")
	cfg := config.Defaults()
	cfg.Type = config.TypeRedis
	cfg.URL = "redis://localhost:6379"

	c, err := cache.NewAdvanced[int](cfg, config.Options[int]{})
	if err != nil {
		panic(err)
	}
	defer c.Close()

	ctx := context.Background()

	// Set
	c.Set(ctx, "counter", 42, 10*time.Second)

	// Get
	val, err := c.Get(ctx, "counter")
	if err == nil {
		fmt.Println("Counter:", val) // Output: Counter: 42
	}

	// Len (approximate, scans keys)
	length, _ := c.Len(ctx)
	fmt.Println("Items in cache:", length)

	// Clear
	c.Clear(ctx)
}
