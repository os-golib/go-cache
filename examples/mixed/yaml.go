package main

import (
	"context"
	"fmt"
	"os"

	"github.com/os-golib/go-cache"
	"github.com/os-golib/go-cache/config"
)

// Domain model
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func yaml() {
	fmt.Println("===== YAML example starting =====")
	yamlBytes, err := os.ReadFile("redis.yaml")
	if err != nil {
		panic(err)
	}

	cfg, err := config.Load(yamlBytes)
	if err != nil {
		panic(err)
	}

	// fmt.Printf("Loaded cache config: %+v\n", cfg)

	ac, err := cache.NewAdvanced[User](cfg, config.Options[User]{})
	if err != nil {
		panic(err)
	}
	defer ac.Close()

	ctx := context.Background()

	// Set some prefixed keys
	ac.Set(ctx, "user:1", User{ID: 1, Name: "Alice", Email: "alice@example.com"}, 0)
	ac.Set(ctx, "user:2", User{ID: 2, Name: "Bob", Email: "bob@example.com"}, 0)

	// Delete by prefix
	deleted, err := ac.DeleteByPrefix(ctx, "user:")
	if err == nil {
		fmt.Println("Deleted keys:", deleted) // Output: Deleted keys: 2
	}
}
