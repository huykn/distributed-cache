package main

import (
	"context"
	"fmt"
	"log"
	"strings"
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
	fmt.Println("LFU vs LRU Cache Comparison")
	fmt.Println("========================================")
	fmt.Println()

	// This example compares LFU (Least Frequently Used) and LRU (Least Recently Used)
	// cache implementations side-by-side to help you choose the right one for your use case.

	fmt.Println("Overview:")
	fmt.Println("  LFU: Tracks frequency of access (how often)")
	fmt.Println("  LRU: Tracks recency of access (when last)")
	fmt.Println()

	// Comparison Table
	fmt.Println("========================================")
	fmt.Println("Feature Comparison")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Printf("%-25s | %-30s | %-30s\n", "Feature", "LFU (Ristretto)", "LRU (golang-lru)")
	fmt.Println(strings.Repeat("-", 90))
	fmt.Printf("%-25s | %-30s | %-30s\n", "Eviction Strategy", "Least frequently used", "Least recently used")
	fmt.Printf("%-25s | %-30s | %-30s\n", "Tracking", "Access frequency", "Access recency")
	fmt.Printf("%-25s | %-30s | %-30s\n", "Algorithm", "TinyLFU", "Doubly-linked list")
	fmt.Printf("%-25s | %-30s | %-30s\n", "Best For", "Varying access patterns", "Sequential patterns")
	fmt.Printf("%-25s | %-30s | %-30s\n", "Complexity", "Higher (frequency tracking)", "Lower (simple ordering)")
	fmt.Printf("%-25s | %-30s | %-30s\n", "Memory Overhead", "Moderate (counters)", "Low (linked list)")
	fmt.Printf("%-25s | %-30s | %-30s\n", "Hit Ratio", "Better for hot items", "Better for recent items")
	fmt.Printf("%-25s | %-30s | %-30s\n", "Predictability", "Less predictable", "More predictable")
	fmt.Printf("%-25s | %-30s | %-30s\n", "Default", "Yes", "No")
	fmt.Println()

	// Scenario 1: LFU Advantage - Varying Access Patterns
	fmt.Println("========================================")
	fmt.Println("Scenario 1: Varying Access Patterns")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Use case: Some items are accessed much more frequently than others")
	fmt.Println("Example: Popular user profiles, trending products, hot data")
	fmt.Println()
	fmt.Println("LFU Behavior:")
	fmt.Println("  - Item A: 100 accesses → Stays in cache (high frequency)")
	fmt.Println("  - Item B: 50 accesses → Stays in cache (medium frequency)")
	fmt.Println("  - Item C: 2 accesses → Evicted first (low frequency)")
	fmt.Println("  ✓ LFU keeps frequently accessed items regardless of when they were accessed")
	fmt.Println()
	fmt.Println("LRU Behavior:")
	fmt.Println("  - Item A: Last accessed 1 hour ago → May be evicted")
	fmt.Println("  - Item C: Last accessed 1 minute ago → Stays in cache")
	fmt.Println("  ✗ LRU may evict frequently used items if not accessed recently")
	fmt.Println()
	fmt.Println("Winner: LFU ✓")
	fmt.Println()

	// Scenario 2: LRU Advantage - Sequential Access Patterns
	fmt.Println("========================================")
	fmt.Println("Scenario 2: Sequential Access Patterns")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Use case: Data accessed in sequence or with temporal locality")
	fmt.Println("Example: Pagination, time-series data, log processing")
	fmt.Println()
	fmt.Println("LRU Behavior:")
	fmt.Println("  - Page 1 → Page 2 → Page 3 → Page 4")
	fmt.Println("  - Page 1 evicted when Page 5 is accessed")
	fmt.Println("  ✓ LRU efficiently handles sequential access")
	fmt.Println()
	fmt.Println("LFU Behavior:")
	fmt.Println("  - All pages have frequency = 1")
	fmt.Println("  - Eviction is less predictable")
	fmt.Println("  ✗ LFU doesn't provide clear advantage for sequential access")
	fmt.Println()
	fmt.Println("Winner: LRU ✓")
	fmt.Println()

	// Practical Demonstration
	fmt.Println("========================================")
	fmt.Println("Practical Demonstration")
	fmt.Println("========================================")
	fmt.Println()

	demonstrateLFU()
	fmt.Println()
	demonstrateLRU()

	// Decision Guide
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Decision Guide: Which Cache to Use?")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Choose LFU (Ristretto) when:")
	fmt.Println("  ✓ Access patterns vary significantly")
	fmt.Println("  ✓ Some items are much more popular than others")
	fmt.Println("  ✓ You want to maximize cache hit ratio")
	fmt.Println("  ✓ Memory is limited and eviction must be smart")
	fmt.Println("  ✓ You have 'hot' data that should stay cached")
	fmt.Println()
	fmt.Println("Choose LRU (golang-lru) when:")
	fmt.Println("  ✓ Access patterns are sequential or temporal")
	fmt.Println("  ✓ Recent items are more likely to be accessed again")
	fmt.Println("  ✓ You want predictable eviction behavior")
	fmt.Println("  ✓ Simplicity is preferred over complex tracking")
	fmt.Println("  ✓ You process data in batches or pages")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println()
	fmt.Println("LFU (Default):")
	fmt.Println("  cfg.LocalCacheFactory = cache.NewLFUCacheFactory(cfg.LocalCacheConfig)")
	fmt.Println("  cfg.LocalCacheConfig.NumCounters = 1e7")
	fmt.Println("  cfg.LocalCacheConfig.MaxCost = 1 << 30  // 1GB")
	fmt.Println()
	fmt.Println("LRU:")
	fmt.Println("  cfg.LocalCacheFactory = cache.NewLRUCacheFactory(10000)")
	fmt.Println("  // 10000 = maximum number of items")
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Example completed successfully!")
	fmt.Println("========================================")
}

func demonstrateLFU() {
	fmt.Println("--- LFU Cache Demonstration ---")

	cfg := dc.DefaultConfig()
	cfg.PodID = "lfu-comparison-pod"
	cfg.RedisAddr = "localhost:6379"
	cfg.LocalCacheFactory = cache.NewLFUCacheFactory(cfg.LocalCacheConfig)

	cache, err := dc.New(cfg)
	if err != nil {
		log.Printf("Error creating LFU cache: %v", err)
		return
	}
	defer cache.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Add users and access with different frequencies
	for i := 1; i <= 3; i++ {
		user := User{ID: i, Name: fmt.Sprintf("User%d", i), Email: fmt.Sprintf("user%d@example.com", i)}
		err := cache.Set(ctx, fmt.Sprintf("user:%d", i), user)
		if err != nil {
			log.Printf("Error setting user %d: %v", i, err)
		}
	}

	// Access User1 frequently
	for range 10 {
		cache.Get(ctx, "user:1")
	}

	// Access User2 moderately
	for range 3 {
		cache.Get(ctx, "user:2")
	}

	// Access User3 once
	cache.Get(ctx, "user:3")

	stats := cache.Stats()
	fmt.Printf("LFU Stats - Hits: %d, Misses: %d\n", stats.LocalHits, stats.LocalMisses)
	fmt.Println("Result: User1 (10 accesses) will be kept longest")
}

func demonstrateLRU() {
	fmt.Println("--- LRU Cache Demonstration ---")

	cfg := dc.DefaultConfig()
	cfg.PodID = "lru-comparison-pod"
	cfg.RedisAddr = "localhost:6379"
	cfg.LocalCacheFactory = cache.NewLRUCacheFactory(10000)

	cache, err := dc.New(cfg)
	if err != nil {
		log.Printf("Error creating LRU cache: %v", err)
		return
	}
	defer cache.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Add users
	for i := 1; i <= 3; i++ {
		user := User{ID: i, Name: fmt.Sprintf("User%d", i), Email: fmt.Sprintf("user%d@example.com", i)}
		err := cache.Set(ctx, fmt.Sprintf("user:%d", i), user)
		if err != nil {
			log.Printf("Error setting user %d: %v", i, err)
		}
	}

	// Access in specific order
	cache.Get(ctx, "user:3") // Most recent
	time.Sleep(10 * time.Millisecond)
	cache.Get(ctx, "user:2")
	time.Sleep(10 * time.Millisecond)
	cache.Get(ctx, "user:1") // Least recent

	stats := cache.Stats()
	fmt.Printf("LRU Stats - Hits: %d, Misses: %d\n", stats.LocalHits, stats.LocalMisses)
	fmt.Println("Result: User1 (least recent) will be evicted first")
}
