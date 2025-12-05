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

	cfg, _ := config.NewBuilder(config.TypeMemory).WithMemoryConfig(100, time.Minute).Build()

	// cfg := config.Config{
	// 	Type:            config.TypeMemory,
	// 	TTL:             5 * time.Second,
	// 	Prefix:          "demo:",
	// 	MaxSize:         100,
	// 	CleanupInterval: 10 * time.Second,
	// }

	c, err := cache.NewAdvanced[string](cfg)
	if err != nil {
		panic(err)
	}

	// Set value
	if err := c.Set(ctx, "greeting", "Hello Memory Cache!", 0); err != nil {
		panic(err)
	}

	// Get value
	value, err := c.Get(ctx, "greeting")
	if err != nil {
		panic(err)
	}
	fmt.Println("GET greeting =>", value)

	// Check exists
	ok, _ := c.Exists(ctx, "greeting")
	fmt.Println("Exists(greeting) =>", ok)

	// Delete
	if err := c.Delete(ctx, "greeting"); err != nil {
		panic(err)
	}

	_, err = c.Get(ctx, "greeting")
	fmt.Println("After delete =>", err)
}
