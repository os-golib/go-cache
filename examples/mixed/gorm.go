package main

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/os-golib/go-cache"
	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/integration"
)

func mygorm() {
	fmt.Println("===== GORM example starting =====")
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(&User{})

	// Seed data
	db.Create(&User{ID: 1, Name: "Alice", Email: "alice@example.com"})

	cfg := config.Defaults()
	cfg.Type = config.TypeMemory

	advancedCache, err := cache.NewAdvanced[User](cfg)
	if err != nil {
		panic(err)
	}

	gc := integration.NewGORMCache[User](advancedCache, db)

	ctx := context.Background()

	// GetByID: Loads from DB if miss, caches it
	user, err := gc.GetByID(ctx, 1, 10*time.Second)
	if err == nil {
		fmt.Println("User:", user.Name) // Output: User: Alice
	}

	// Invalidate
	gc.Invalidate(ctx, 1)

	// Miss after invalidate
	_, err = gc.GetByID(ctx, 1, 10*time.Second) // Will reload from DB
	fmt.Println("Reloaded user:", user.Name)
}
