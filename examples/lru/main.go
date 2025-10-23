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
	fmt.Println("LRU Cache Example")
	fmt.Println("========================================")
	fmt.Println()

	// This example demonstrates the LRU (Least Recently Used) cache
	// implementation using the hashicorp/golang-lru library.
	//
	// LRU Cache Characteristics:
	// - Tracks recency of access for each item
	// - Evicts items that haven't been accessed recently
	// - Better for sequential access patterns
	// - Simpler and more predictable than LFU
	// - Uses doubly-linked list for O(1) operations
	//
	// Use LRU when:
	// - Access patterns are sequential or temporal
	// - Recent items are more likely to be accessed again
	// - You want predictable eviction behavior
	// - Simplicity is preferred over complex frequency tracking

	fmt.Println("LRU Cache Overview:")
	fmt.Println("  Algorithm: Least Recently Used")
	fmt.Println("  Eviction: Items not accessed recently")
	fmt.Println("  Best for: Sequential/temporal access patterns")
	fmt.Println()

	// Create configuration with LRU cache
	cfg := dc.DefaultConfig()
	cfg.PodID = "lru-cache-pod"
	cfg.RedisAddr = "localhost:6379"

	// Configure LRU cache with maximum size
	// Note: LRU uses MaxSize parameter, not MaxCost
	maxSize := 10000 // Maximum number of items in cache

	// Use LRU cache factory
	cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)

	fmt.Println("LRU Configuration:")
	fmt.Printf("  MaxSize: %d items\n", maxSize)
	fmt.Printf("  Eviction: Least recently used\n")
	fmt.Println()

	// Initialize cache
	fmt.Println("Initializing cache with LRU implementation...")
	cache, err := dc.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	fmt.Println("✓ Cache initialized successfully")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Demonstrate LRU behavior
	fmt.Println("Demonstrating LRU behavior...")
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
	fmt.Println("  Order: User1 → User2 → User3 → User4 → User5")

	// Access users in a specific order to demonstrate LRU
	fmt.Println("\nStep 2: Accessing users in specific order...")
	fmt.Println("  Accessing: User5 → User3 → User1")

	// Access User 5 (most recent)
	time.Sleep(100 * time.Millisecond)
	cache.Get(ctx, "user:5")
	fmt.Println("  ✓ Accessed User5 (now most recent)")

	// Access User 3
	time.Sleep(100 * time.Millisecond)
	cache.Get(ctx, "user:3")
	fmt.Println("  ✓ Accessed User3")

	// Access User 1
	time.Sleep(100 * time.Millisecond)
	cache.Get(ctx, "user:1")
	fmt.Println("  ✓ Accessed User1")

	fmt.Println("\nCurrent LRU order (most → least recent):")
	fmt.Println("  User1 → User3 → User5 → User4 → User2")
	fmt.Println("  (User2 and User4 are least recently used)")

	// Add more users to demonstrate eviction
	fmt.Println("\nStep 3: Adding more users to trigger eviction...")
	for i := 6; i <= 8; i++ {
		user := User{
			ID:    i,
			Name:  fmt.Sprintf("User%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
		}
		if err := cache.Set(ctx, fmt.Sprintf("user:%d", i), user); err != nil {
			log.Printf("Error setting user %d: %v", i, err)
		}
	}
	fmt.Println("✓ Added User6, User7, User8")

	// Show cache statistics
	fmt.Println("\nStep 4: Cache Statistics...")
	stats := cache.Stats()
	fmt.Printf("  Local Hits: %d\n", stats.LocalHits)
	fmt.Printf("  Local Misses: %d\n", stats.LocalMisses)
	if stats.LocalHits+stats.LocalMisses > 0 {
		fmt.Printf("  Hit Ratio: %.2f%%\n", float64(stats.LocalHits)/float64(stats.LocalHits+stats.LocalMisses)*100)
	}

	fmt.Println("\n========================================")
	fmt.Println()
	fmt.Println("LRU Cache Behavior:")
	fmt.Println("  - Recently accessed items stay in cache")
	fmt.Println("  - Least recently used items are evicted first")
	fmt.Println("  - Access order determines eviction priority")
	fmt.Println("  - Simple and predictable eviction policy")
	fmt.Println()
	fmt.Println("When to Use LRU:")
	fmt.Println("  ✓ Sequential access patterns")
	fmt.Println("  ✓ Temporal locality (recent items accessed again)")
	fmt.Println("  ✓ Predictable eviction behavior needed")
	fmt.Println("  ✓ Simpler implementation preferred")
	fmt.Println()
	fmt.Println("LRU vs LFU:")
	fmt.Println("  LRU: Tracks recency (when last accessed)")
	fmt.Println("  LFU: Tracks frequency (how often accessed)")
	fmt.Println("  LRU: Better for sequential patterns")
	fmt.Println("  LFU: Better for varying access patterns")
	fmt.Println()
	fmt.Println("Configuration Tips:")
	fmt.Println("  - Set MaxSize based on expected working set")
	fmt.Println("  - Monitor hit ratio to validate size")
	fmt.Println("  - Consider memory per item when sizing")
	fmt.Println("  - Use metrics to tune cache size")
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Example completed successfully!")
	fmt.Println("========================================")
}
