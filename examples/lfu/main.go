package main

import (
	"context"
	"fmt"
	"log"
	"time"

	dc "github.com/huykn/distributed-cache"
	"github.com/huykn/distributed-cache/cache"
)

// User represents a sample user object.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	fmt.Println("========================================")
	fmt.Println("LFU Cache (Ristretto) Example")
	fmt.Println("========================================")
	fmt.Println()

	// This example demonstrates the LFU (Least Frequently Used) cache
	// implementation using the Ristretto library.
	//
	// LFU Cache Characteristics:
	// - Tracks frequency of access for each item
	// - Evicts items with the lowest access frequency
	// - Better for workloads with varying access patterns
	// - Uses TinyLFU algorithm for efficient frequency tracking
	// - Default implementation in distributed-cache
	//
	// Use LFU when:
	// - Access patterns vary significantly
	// - Some items are accessed much more frequently than others
	// - You want to keep "hot" items in cache longer
	// - Memory is limited and you need smart eviction

	fmt.Println("LFU Cache Overview:")
	fmt.Println("  Algorithm: TinyLFU (via Ristretto)")
	fmt.Println("  Eviction: Least Frequently Used items")
	fmt.Println("  Best for: Varying access patterns")
	fmt.Println()

	// Create configuration with LFU cache
	cfg := dc.DefaultConfig()
	cfg.PodID = "lfu-cache-pod"
	cfg.RedisAddr = "localhost:6379"

	// Configure LFU cache parameters
	cfg.LocalCacheConfig = dc.LocalCacheConfig{
		NumCounters:        1e7,     // 10 million counters for frequency tracking
		MaxCost:            1 << 30, // 1GB max cache size
		BufferItems:        64,      // Buffer for async operations
		IgnoreInternalCost: false,   // Track actual memory cost
	}

	// Use LFU cache factory (this is the default, but shown explicitly)
	cfg.LocalCacheFactory = cache.NewLFUCacheFactory(cfg.LocalCacheConfig)

	fmt.Println("LFU Configuration:")
	fmt.Printf("  NumCounters: %d\n", cfg.LocalCacheConfig.NumCounters)
	fmt.Printf("  MaxCost: %d bytes (%.2f GB)\n", cfg.LocalCacheConfig.MaxCost, float64(cfg.LocalCacheConfig.MaxCost)/(1<<30))
	fmt.Printf("  BufferItems: %d\n", cfg.LocalCacheConfig.BufferItems)
	fmt.Println()

	// Initialize cache
	fmt.Println("Initializing cache with LFU implementation...")
	cache, err := dc.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	fmt.Println("✓ Cache initialized successfully")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Demonstrate LFU behavior
	fmt.Println("Demonstrating LFU behavior...")
	fmt.Println("========================================")

	// Add multiple users
	fmt.Println("\nStep 1: Adding 5 users to cache...")
	for i := 1; i <= 5; i++ {
		user := User{
			ID:    i,
			Name:  fmt.Sprintf("User%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
		}
		if err := cache.Set(ctx, fmt.Sprintf("user:%d", i), user); err != nil {
			log.Printf("Error setting user %d: %v", i, err)
		}
	}
	fmt.Println("✓ 5 users added")

	// Access some users more frequently than others
	fmt.Println("\nStep 2: Accessing users with different frequencies...")
	fmt.Println("  - User 1: 10 accesses (high frequency)")
	fmt.Println("  - User 2: 5 accesses (medium frequency)")
	fmt.Println("  - User 3: 2 accesses (low frequency)")
	fmt.Println("  - User 4: 1 access (very low frequency)")
	fmt.Println("  - User 5: 1 access (very low frequency)")

	// User 1: High frequency (10 accesses)
	for i := 0; i < 10; i++ {
		cache.Get(ctx, "user:1")
	}

	// User 2: Medium frequency (5 accesses)
	for i := 0; i < 5; i++ {
		cache.Get(ctx, "user:2")
	}

	// User 3: Low frequency (2 accesses)
	for i := 0; i < 2; i++ {
		cache.Get(ctx, "user:3")
	}

	// User 4 and 5: Very low frequency (1 access each)
	cache.Get(ctx, "user:4")
	cache.Get(ctx, "user:5")

	fmt.Println("✓ Access pattern established")

	// Show cache statistics
	fmt.Println("\nStep 3: Cache Statistics...")
	stats := cache.Stats()
	fmt.Printf("  Local Hits: %d\n", stats.LocalHits)
	fmt.Printf("  Local Misses: %d\n", stats.LocalMisses)
	fmt.Printf("  Hit Ratio: %.2f%%\n", float64(stats.LocalHits)/float64(stats.LocalHits+stats.LocalMisses)*100)

	fmt.Println("\n========================================")
	fmt.Println()
	fmt.Println("LFU Cache Behavior:")
	fmt.Println("  - User 1 (10 accesses) will be kept longest")
	fmt.Println("  - User 2 (5 accesses) will be kept moderately")
	fmt.Println("  - Users 4 & 5 (1 access) will be evicted first")
	fmt.Println("  - Frequency tracking ensures hot items stay cached")
	fmt.Println()
	fmt.Println("When to Use LFU:")
	fmt.Println("  ✓ Access patterns vary significantly")
	fmt.Println("  ✓ Some items are much more popular than others")
	fmt.Println("  ✓ You want to maximize cache hit ratio")
	fmt.Println("  ✓ Memory is limited and eviction must be smart")
	fmt.Println()
	fmt.Println("Configuration Tips:")
	fmt.Println("  - NumCounters: 10x the expected number of items")
	fmt.Println("  - MaxCost: Set based on available memory")
	fmt.Println("  - BufferItems: 64 is a good default")
	fmt.Println("  - Monitor metrics to tune parameters")
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Example completed successfully!")
	fmt.Println("========================================")
}
