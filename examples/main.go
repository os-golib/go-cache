package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/os-golib/go-cache"
	"github.com/os-golib/go-cache/config"
	"github.com/os-golib/go-cache/integration"
)

// =============================================================================
// Data Structures
// =============================================================================

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func main() {
	fmt.Println("🚀 Go-Cache Comprehensive Example")
	fmt.Println(strings.Repeat("=", 50))

	// 1. Memory Cache Example
	exampleMemoryCache()

	// 2. Redis Cache Example
	exampleRedisCache()

	// 3. Advanced Features
	exampleAdvancedFeatures()

	// 4. GORM Integration
	exampleGORMIntegration()

	// 5. Builder Pattern
	exampleBuilderPattern()

	// 6. Cache Strategies
	exampleCacheStrategies()

	// 7. YAML Configuration
	exampleYAMLConfig()

	fmt.Println("\n✅ All examples completed successfully!")
}

// 1. Memory Cache Example
func exampleMemoryCache() {
	fmt.Println("\n📦 1. Memory Cache Example")
	ctx := context.Background()

	// Create memory cache configuration
	cfg, err := cache.NewBuilder().
		WithMemory().
		WithTTL(5 * time.Minute). // Default 5 minute TTL
		Build()
	if err != nil {
		log.Fatal(err)
	}

	// Create advanced cache instance
	ac, err := cache.NewAdvanced[string](cfg)
	if err != nil {
		log.Fatal(err)
	}
	// defer ac.Close()

	// Basic operations
	err = ac.Set(ctx, "greeting", "Hello Memory Cache!", 0) // 0
	//
	//   default TTL
	if err != nil {
		log.Fatal(err)
	}

	val, err := ac.Get(ctx, "greeting")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  Get value: %s\n", val)

	// Check existence
	exists, _ := ac.Exists(ctx, "greeting")
	fmt.Printf("  Key exists: %v\n", exists)

	// Delete
	ac.Delete(ctx, "greeting")
	fmt.Println("  Key deleted")

}

// 2. Redis Cache Example
func exampleRedisCache() {
	fmt.Println("\n🔴 2. Redis Cache Example")
	ctx := context.Background()

	// Create Redis cache configuration
	cfg, err := cache.NewBuilder().
		WithRedis("redis://localhost:6379/0"). // Redis connection string
		WithTTL(10 * time.Minute).
		Build()
	if err != nil {
		log.Printf("Note: Redis not available, skipping Redis example: %v", err)
		return
	}

	ac, err := cache.NewAdvanced[User](cfg)
	if err != nil {
		log.Fatal(err)
	}
	// defer ac.Close()

	// Store user object
	user := User{ID: 1, Name: "John Doe", Email: "john@example.com"}
	err = ac.Set(ctx, "user:1", user, 0)
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve user
	retrievedUser, err := ac.Get(ctx, "user:1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  Retrieved user: %+v\n", retrievedUser)

	// Get statistics
	stats := ac.Stats(ctx)
	fmt.Printf("  Cache stats: %+v\n", stats)
}

// 3. Advanced Features
func exampleAdvancedFeatures() {
	fmt.Println("\n⚡ 3. Advanced Features")
	ctx := context.Background()

	cfg, _ := cache.NewBuilder().WithMemory().Build()
	ac, err := cache.NewAdvanced[string](cfg)
	if err != nil {
		log.Fatal(err)
	}
	// defer ac.Close()

	// GetOrSet - computes value only if missing
	fmt.Println("  GetOrSet Example:")
	val, err := ac.GetOrSet(ctx, "computed_value", 30*time.Second, func() (string, error) {
		fmt.Println("    Computing expensive value...")
		time.Sleep(500 * time.Millisecond) // Simulate expensive computation
		return "Computed Result", nil
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("    Result: %s\n", val)

	// GetOrSetLocked - distributed locking for cache computation
	fmt.Println("  GetOrSetLocked Example:")
	val2, err := ac.GetOrSetLocked(ctx, "locked_value", 30*time.Second, func() (string, error) {
		fmt.Println("    Computing under distributed lock...")
		return time.Now().Format(time.RFC3339), nil
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("    Locked result: %s\n", val2)

	// Bulk operations with pipeline
	fmt.Println("  Bulk Operations:")
	items := map[string]string{
		"user:1": "Alice",
		"user:2": "Bob",
		"user:3": "Charlie",
	}

	err = ac.SetManyPipeline(ctx, items, 20*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	results, err := ac.GetManyPipeline(ctx, []string{"user:1", "user:2", "user:3"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("    Pipeline results: %v\n", results)

	// Delete by prefix
	deleted, _ := ac.DeleteByPrefix(ctx, "user:")
	fmt.Printf("    Deleted %d items with prefix 'user:'\n", deleted)
}

// 4. GORM Integration
func exampleGORMIntegration() {
	fmt.Println("\n🗄️  4. GORM Integration Example")

	// Setup SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		log.Printf("  Note: SQLite not available, skipping GORM example: %v", err)
		return
	}

	// Auto migrate
	db.AutoMigrate(&User{})

	// Seed data
	db.Create(&User{ID: 1, Name: "Alice", Email: "alice@example.com"})

	// Create cache
	cfg := config.DefaultConfig()
	cfg.Type = config.TypeMemory

	advancedCache, err := cache.NewAdvanced[User](cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer advancedCache.Close()

	// Create GORM cache wrapper
	gc := integration.NewGORMCache[User](advancedCache, db)

	ctx := context.Background()

	// GetByID - loads from DB if cache miss
	fmt.Println("  Database Query with Caching:")
	user, err := gc.GetByID(ctx, 1, 10*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("    User from DB (cached): %s (%s)\n", user.Name, user.Email)

	// Invalidate cache
	gc.Invalidate(ctx, 1)
	fmt.Println("    Cache invalidated")

	// Will reload from DB
	user, err = gc.GetByID(ctx, 1, 10*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("    User reloaded from DB: %s\n", user.Name)
}

// 5. Builder Pattern
func exampleBuilderPattern() {
	fmt.Println("\n🔨 5. Builder Pattern Examples")

	// Key Builder
	fmt.Println("  Key Builder:")
	keyBuilder := cache.NewKeyBuilder("myapp")

	userKey := keyBuilder.Add("users").Add("1").Build()
	productKey := keyBuilder.Reset().Add("products").Add("42").Build()
	sessionKey := keyBuilder.Reset().Add("sessions").Add("abc123").Build()

	fmt.Printf("    User key: %s\n", userKey)
	fmt.Printf("    Product key: %s\n", productKey)
	fmt.Printf("    Session key: %s\n", sessionKey)

	// Complex key building
	fmt.Println("  Complex Key Building:")
	apiKeyBuilder := cache.NewKeyBuilder("api")

	buildSearchKey := func(query string, page, limit int) string {
		return apiKeyBuilder.
			Add("search").
			Add(query).
			Add("page").
			Add(fmt.Sprintf("%d", page)).
			Add("limit").
			Add(fmt.Sprintf("%d", limit)).
			Build()
	}

	searchKey := buildSearchKey("golang", 1, 10)
	fmt.Printf("    Search API key: %s\n", searchKey)
}

// 6. Cache Strategies
func exampleCacheStrategies() {
	fmt.Println("\n🎯 6. Cache Strategies")

	ctx := context.Background()
	cfg, _ := cache.NewBuilder().WithMemory().Build()
	ac, err := cache.NewAdvanced[string](cfg)
	if err != nil {
		log.Fatal(err)
	}
	// defer ac.Close()

	// Cache-Aside Pattern
	fmt.Println("  Cache-Aside Pattern:")
	cacheAside := func(key string) (string, error) {
		// 1. Try cache first
		if val, err := ac.Get(ctx, key); err == nil {
			fmt.Println("    Cache hit!")
			return val, nil
		}

		// 2. Cache miss - compute/store
		fmt.Println("    Cache miss, computing...")
		computedValue := "Expensive Computation Result"

		// 3. Store in cache
		ac.Set(ctx, key, computedValue, 5*time.Minute)
		return computedValue, nil
	}

	result, _ := cacheAside("expensive_data")
	fmt.Printf("    Result: %s\n", result)

	// Write-Through Pattern simulation
	fmt.Println("  Write-Through Pattern:")
	writeThrough := func(key, value string) error {
		// 1. Write to database (simulated)
		fmt.Printf("    Writing to database: %s = %s\n", key, value)

		// 2. Update cache
		return ac.Set(ctx, key, value, 10*time.Minute)
	}

	writeThrough("config:timeout", "30s")
	fmt.Println("    Write-through completed")

	// Time-based invalidation
	fmt.Println("  Time-based Invalidation:")
	ac.Set(ctx, "temporary_data", "This expires soon", 2*time.Second)
	time.Sleep(3 * time.Second)

	_, err = ac.Get(ctx, "temporary_data")
	if err != nil {
		fmt.Println("    Data correctly expired (cache miss)")
	}

	// Metrics
	fmt.Println("  Cache Metrics:")
	metrics := ac.Metrics().Snapshot()
	for op, stats := range metrics {
		fmt.Printf("    %s: Hits=%d, Misses=%d, ErrorRate=%.2f%%\n",
			op, stats.Hits, stats.Misses, float64(stats.Misses)/float64(stats.Hits+stats.Misses)*100)
	}
}

// 7. YAML Configuration
func exampleYAMLConfig() {
	fmt.Println("\n📄 YAML Configuration Example")

	// Create Memory cache configuration from file
	cfg, err := cache.NewBuilder().WithLoadFromFile("memory.yaml").Build()

	// // Create Redis cache configuration from file
	// cfg, err := cache.NewBuilder().WithLoadFromFile("redis.yaml").Build()

	if err != nil {
		fmt.Printf("  Note: Create cache-config.yaml to use this feature\n")
		return
	}

	ac, err := cache.NewAdvanced[string](cfg)
	if err != nil {
		log.Fatal(err)
	}
	// defer ac.Close()

	ctx := context.Background()
	ac.Set(ctx, "yaml_configured", "Works with YAML config!", 0)

	val, _ := ac.Get(ctx, "yaml_configured")
	fmt.Printf("  Value from YAML-configured cache: %s\n", val)
}
