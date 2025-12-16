package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/os-golib/go-cache"
)

func main() {
	// Create a memory cache
	// memCache, err := cache.NewMemory[string]()
	memCache, err := cache.NewRedis[string]("redis://localhost:6379/0")
	if err != nil {
		log.Fatal(err)
	}
	defer memCache.Close()

	// Example 1: Basic operations with timeout
	ctx := context.Background()

	// Set with 2-second timeout
	setCtx, cancelSet := cache.WithContext(ctx, 2*time.Second)
	defer cancelSet()

	err = memCache.Set(setCtx, "user:1", "John Doe", 10*time.Minute)
	if err != nil {
		log.Printf("Failed to set: %v", err)
	}

	// Get with 1-second timeout
	getCtx, cancelGet := cache.WithContext(ctx, time.Second)
	defer cancelGet()

	value, err := memCache.Get(getCtx, "user:1")
	if err != nil {
		log.Printf("Failed to get: %v", err)
	} else {
		fmt.Printf("Retrieved: %s\n", value)
	}

	keyBuilderExample()

	ecommerceCacheExample()

	cacheWithRetry()

	batchOperationsExample()

	hierarchicalKeysExample()

	dynamicKeyBuilding()
}

func keyBuilderExample() {
	// Create key builders for different entities
	userKey := cache.NewKeyBuilder("app").Add("users")
	productKey := cache.NewKeyBuilder("app").Add("products")
	sessionKey := cache.NewKeyBuilder("app").Add("sessions")

	// Build consistent keys
	user1Key := userKey.Add("1").Build()            // "app:users:1"
	product42Key := productKey.Add("42").Build()    // "app:products:42"
	session1Key := sessionKey.Add("abc123").Build() // "app:sessions:abc123"

	fmt.Printf("User key: %s\n", user1Key)
	fmt.Printf("Product key: %s\n", product42Key)
	fmt.Printf("Session key: %s\n", session1Key)
}

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

type Cart struct {
	ID     int        `json:"id"`
	UserID int        `json:"user_id"`
	Items  []CartItem `json:"items"`
	Total  float64    `json:"total"`
}

type CartItem struct {
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

func ecommerceCacheExample() {
	// // Create cache instances for different entities
	// userCache, _ := cache.NewMemoryAdvanced[User]()
	// productCache, _ := cache.NewMemoryAdvanced[Product]()
	// cartCache, _ := cache.NewMemoryAdvanced[Cart]()

	userCache, _ := cache.NewAdvancedRedis[User]("redis://localhost:6379/0")
	productCache, _ := cache.NewAdvancedRedis[Product]("redis://localhost:6379/0")
	// cartCache, _ := cache.NewAdvancedRedis[Cart]("redis://localhost:6379/0")

	defer userCache.Close()
	defer productCache.Close()
	// defer cartCache.Close()

	ctx := context.Background()

	// Key builders for different entities
	userKeyBuilder := cache.NewKeyBuilder("ecom").Add("users")
	productKeyBuilder := cache.NewKeyBuilder("ecom").Add("products")
	// cartKeyBuilder := cache.NewKeyBuilder("ecom").Add("carts")

	// Cache user data with timeout
	userCtx, cancel := cache.WithContext(ctx, 3*time.Second)
	defer cancel()

	user := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
	userKey := userKeyBuilder.Add(fmt.Sprintf("%d", user.ID)).Build()

	err := userCache.Set(userCtx, userKey, user, 30*time.Minute)
	if err != nil {
		log.Printf("Failed to cache user: %v", err)
	}

	// Cache multiple products
	products := []Product{
		{ID: 1, Name: "Laptop", Price: 999.99},
		{ID: 2, Name: "Mouse", Price: 29.99},
		{ID: 3, Name: "Keyboard", Price: 79.99},
	}

	productCtx, cancel := cache.WithContext(ctx, 5*time.Second)
	defer cancel()

	for _, product := range products {
		key := productKeyBuilder.Add(fmt.Sprintf("%d", product.ID)).Build()
		err := productCache.Set(productCtx, key, product, time.Hour)
		if err != nil {
			log.Printf("Failed to cache product %d: %v", product.ID, err)
		}
	}

	// Retrieve user with timeout
	getUserCtx, cancel := cache.WithContext(ctx, 2*time.Second)
	defer cancel()

	cachedUser, err := userCache.Get(getUserCtx, userKey)
	if err != nil {
		log.Printf("Failed to get user: %v", err)
	} else {
		fmt.Printf("Retrieved user: %+v\n", cachedUser)
	}
}

func cacheWithRetry() {
	// acache, err := cache.NewMemoryAdvanced[string]()
	acache, err := cache.NewAdvancedRedis[string]("redis://localhost:6379/0")
	if err != nil {
		log.Fatal(err)
	}
	defer acache.Close()

	ctx := context.Background()
	key := cache.NewKeyBuilder("retry").Add("example").Build()

	// Retry logic for cache operations
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		// Use shorter timeout for retries
		timeout := time.Duration(i+1) * time.Second
		opCtx, cancel := cache.WithContext(ctx, timeout)

		err := acache.Set(opCtx, key, fmt.Sprintf("value-attempt-%d", i+1), 5*time.Minute)
		cancel()

		if err == nil {
			fmt.Printf("Success on attempt %d\n", i+1)
			break
		}

		lastErr = err
		fmt.Printf("Attempt %d failed: %v\n", i+1, err)

		if i < maxRetries-1 {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond) // Exponential backoff
		}
	}

	if lastErr != nil {
		log.Printf("All retry attempts failed: %v", lastErr)
	}
}

func batchOperationsExample() {
	// acache, err := cache.NewMemoryAdvanced[string]()
	acache, err := cache.NewAdvancedRedis[string]("redis://localhost:6379/0")
	if err != nil {
		log.Fatal(err)
	}
	defer acache.Close()

	ctx := context.Background()
	userKeyBuilder := cache.NewKeyBuilder("batch").Add("users")

	// Prepare batch data
	users := map[int]string{
		1: "Alice",
		2: "Bob",
		3: "Charlie",
		4: "Diana",
	}

	// Create batch operation context with longer timeout
	batchCtx, cancel := cache.WithContext(ctx, 10*time.Second)
	defer cancel()

	// Convert to cache items using KeyBuilder
	items := make(map[string]string)
	for id, name := range users {
		key := userKeyBuilder.Add(fmt.Sprintf("%d", id)).Build()
		items[key] = name
	}

	// Set multiple items at once
	err = acache.SetManyPipeline(batchCtx, items, 30*time.Minute)
	if err != nil {
		log.Printf("Batch set failed: %v", err)
	}

	// Build keys for batch get
	keys := make([]string, 0, len(users))
	for id := range users {
		key := userKeyBuilder.Add(fmt.Sprintf("%d", id)).Build()
		keys = append(keys, key)
	}

	// Get multiple items
	getBatchCtx, cancel := cache.WithContext(ctx, 5*time.Second)
	defer cancel()

	results, err := acache.GetManyPipeline(getBatchCtx, keys)
	if err != nil {
		log.Printf("Batch get failed: %v", err)
	} else {
		fmt.Println("Batch get results:")
		for key, value := range results {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
}

func hierarchicalKeysExample() {
	// Multi-level key hierarchy
	geoKeyBuilder := cache.NewKeyBuilder("geo")

	// Country -> State -> City hierarchy
	usKey := geoKeyBuilder.Add("US").Build()                     // "geo:US"
	californiaKey := geoKeyBuilder.Add("US").Add("CA").Build()   // "geo:US:CA"
	laKey := geoKeyBuilder.Add("US").Add("CA").Add("LA").Build() // "geo:US:CA:LA"
	sfKey := geoKeyBuilder.Add("US").Add("CA").Add("SF").Build() // "geo:US:CA:SF"
	nyKey := geoKeyBuilder.Add("US").Add("NY").Build()           // "geo:US:NY"

	fmt.Printf("Country key: %s\n", usKey)
	fmt.Printf("State key: %s\n", californiaKey)
	fmt.Printf("City keys: %s, %s\n", laKey, sfKey)
	fmt.Printf("Another state: %s\n", nyKey)

	// Use in cache operations
	// acache, _ := cache.NewMemoryAdvanced[map[string]interface{}]()
	acache, _ := cache.NewAdvancedRedis[map[string]interface{}]("redis://localhost:6379/0")
	defer acache.Close()

	ctx := context.Background()
	opCtx, cancel := cache.WithContext(ctx, 2*time.Second)
	defer cancel()

	// Cache geographical data
	californiaData := map[string]interface{}{
		"population": 39500000,
		"capital":    "Sacramento",
		"timezone":   "PST",
	}

	err := acache.Set(opCtx, californiaKey, californiaData, time.Hour)
	if err != nil {
		log.Printf("Failed to cache data: %v", err)
	}
}

func dynamicKeyBuilding() {
	// Key builder for API responses with parameters
	apiKeyBuilder := cache.NewKeyBuilder("api")

	// Function to build cache key for API responses
	buildAPIKey := func(method, endpoint string, params map[string]string) string {
		builder := apiKeyBuilder.Add(method).Add(endpoint)

		// Add sorted parameters for consistent keys
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		// sort.Strings(keys) // Uncomment if parameter order matters

		for _, k := range keys {
			builder.Add(k).Add(params[k])
		}

		return builder.Build()
	}

	// Example API calls with different parameters
	userListKey := buildAPIKey("GET", "users", map[string]string{
		"page":  "1",
		"limit": "10",
	}) // "api:GET:users:limit:10:page:1"

	userDetailKey := buildAPIKey("GET", "users", map[string]string{
		"id": "123",
	}) // "api:GET:users:id:123"

	searchKey := buildAPIKey("GET", "search", map[string]string{
		"q":    "golang",
		"sort": "date",
	}) // "api:GET:search:q:golang:sort:date"

	fmt.Printf("User list key: %s\n", userListKey)
	fmt.Printf("User detail key: %s\n", userDetailKey)
	fmt.Printf("Search key: %s\n", searchKey)
}
