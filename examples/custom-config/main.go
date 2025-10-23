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
	fmt.Println("Custom LocalCacheConfig Example")
	fmt.Println("========================================")
	fmt.Println()

	// This example demonstrates how to customize LocalCacheConfig parameters
	// to optimize cache performance for your specific use case.
	//
	// LocalCacheConfig parameters:
	// - NumCounters: Number of counters for frequency tracking (LFU)
	// - MaxCost: Maximum memory cost in bytes (LFU)
	// - BufferItems: Number of items in the buffer for async operations (LFU)
	// - IgnoreInternalCost: Whether to ignore internal memory overhead (LFU)
	// - MaxSize: Maximum number of items in cache (LRU)
	//
	// Tuning guidelines:
	// - NumCounters: 10x the expected number of unique items
	// - MaxCost: Based on available memory (e.g., 1GB = 1 << 30)
	// - BufferItems: 64 is a good default, increase for high throughput
	// - MaxSize (LRU): Based on expected working set size

	// Example 1: Default Configuration
	fmt.Println("=== Example 1: Default Configuration ===")
	defaultConfig := dc.DefaultLocalCacheConfig()
	fmt.Printf("NumCounters: %d\n", defaultConfig.NumCounters)
	fmt.Printf("MaxCost: %d bytes (%.2f GB)\n", defaultConfig.MaxCost, float64(defaultConfig.MaxCost)/(1<<30))
	fmt.Printf("BufferItems: %d\n", defaultConfig.BufferItems)
	fmt.Printf("IgnoreInternalCost: %v\n", defaultConfig.IgnoreInternalCost)
	fmt.Printf("MaxSize: %d\n", defaultConfig.MaxSize)
	fmt.Println()

	// Example 2: Small Cache Configuration (for development/testing)
	fmt.Println("=== Example 2: Small Cache (Development/Testing) ===")
	smallConfig := dc.LocalCacheConfig{
		NumCounters:        1e6,     // 1 million counters
		MaxCost:            1 << 20, // 1MB
		BufferItems:        32,      // Smaller buffer
		IgnoreInternalCost: false,
		MaxSize:            1000, // 1000 items for LRU
	}
	fmt.Printf("NumCounters: %d\n", smallConfig.NumCounters)
	fmt.Printf("MaxCost: %d bytes (%.2f MB)\n", smallConfig.MaxCost, float64(smallConfig.MaxCost)/(1<<20))
	fmt.Printf("BufferItems: %d\n", smallConfig.BufferItems)
	fmt.Printf("MaxSize: %d items\n", smallConfig.MaxSize)
	fmt.Println()

	// Example 3: Large Cache Configuration (for production)
	fmt.Println("=== Example 3: Large Cache (Production) ===")
	largeConfig := dc.LocalCacheConfig{
		NumCounters:        1e8,     // 100 million counters
		MaxCost:            4 << 30, // 4GB
		BufferItems:        128,     // Larger buffer for high throughput
		IgnoreInternalCost: false,
		MaxSize:            100000, // 100k items for LRU
	}
	fmt.Printf("NumCounters: %d\n", largeConfig.NumCounters)
	fmt.Printf("MaxCost: %d bytes (%.2f GB)\n", largeConfig.MaxCost, float64(largeConfig.MaxCost)/(1<<30))
	fmt.Printf("BufferItems: %d\n", largeConfig.BufferItems)
	fmt.Printf("MaxSize: %d items\n", largeConfig.MaxSize)
	fmt.Println()

	// Example 4: Using Custom Configuration with LFU Cache
	fmt.Println("=== Example 4: Custom Config with LFU Cache ===")
	cfg := dc.DefaultConfig()
	cfg.PodID = "custom-config-pod"
	cfg.RedisAddr = "localhost:6379"

	// Use custom configuration
	cfg.LocalCacheConfig = dc.LocalCacheConfig{
		NumCounters:        5e6,       // 5 million counters
		MaxCost:            512 << 20, // 512MB
		BufferItems:        64,
		IgnoreInternalCost: false,
		MaxSize:            10000,
	}

	// Use LFU cache factory with custom config
	cfg.LocalCacheFactory = cache.NewLFUCacheFactory(cfg.LocalCacheConfig)

	fmt.Println("Creating cache with custom configuration...")
	cache, err := dc.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	fmt.Println("✓ Cache initialized with custom config")
	fmt.Println()

	// Test the cache
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println("Testing cache operations...")
	for i := 1; i <= 10; i++ {
		user := User{
			ID:    i,
			Name:  fmt.Sprintf("User%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
		}
		if err := cache.Set(ctx, fmt.Sprintf("user:%d", i), user); err != nil {
			log.Printf("Error: %v", err)
		}
	}
	fmt.Println("✓ Added 10 users to cache")

	// Retrieve and verify
	if value, found := cache.Get(ctx, "user:5"); found {
		if user, ok := value.(User); ok {
			fmt.Printf("✓ Retrieved: %+v\n", user)
		}
	} else {
		log.Println("User not found")
	}

	// Show statistics
	stats := cache.Stats()
	fmt.Printf("\nCache Statistics:\n")
	fmt.Printf("  Local Hits: %d\n", stats.LocalHits)
	fmt.Printf("  Local Misses: %d\n", stats.LocalMisses)

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Configuration Guidelines:")
	fmt.Println()
	fmt.Println("NumCounters (LFU):")
	fmt.Println("  - Rule of thumb: 10x expected unique items")
	fmt.Println("  - Too low: Poor frequency tracking")
	fmt.Println("  - Too high: Wasted memory")
	fmt.Println("  - Example: 1M items → 10M counters")
	fmt.Println()
	fmt.Println("MaxCost (LFU):")
	fmt.Println("  - Set based on available memory")
	fmt.Println("  - Consider: 1GB = 1 << 30 bytes")
	fmt.Println("  - Monitor actual usage with metrics")
	fmt.Println("  - Leave headroom for other processes")
	fmt.Println()
	fmt.Println("BufferItems (LFU):")
	fmt.Println("  - Default: 64 items")
	fmt.Println("  - Increase for high-throughput workloads")
	fmt.Println("  - Decrease for low-latency requirements")
	fmt.Println("  - Powers of 2 work best (32, 64, 128)")
	fmt.Println()
	fmt.Println("MaxSize (LRU):")
	fmt.Println("  - Maximum number of items in cache")
	fmt.Println("  - Based on expected working set size")
	fmt.Println("  - Monitor hit ratio to validate")
	fmt.Println("  - Consider memory per item")
	fmt.Println()
	fmt.Println("Environment-Specific Recommendations:")
	fmt.Println()
	fmt.Println("Development/Testing:")
	fmt.Println("  NumCounters: 1e6, MaxCost: 1MB-10MB, BufferItems: 32")
	fmt.Println()
	fmt.Println("Staging:")
	fmt.Println("  NumCounters: 1e7, MaxCost: 100MB-500MB, BufferItems: 64")
	fmt.Println()
	fmt.Println("Production:")
	fmt.Println("  NumCounters: 1e8+, MaxCost: 1GB-4GB, BufferItems: 64-128")
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Example completed successfully!")
	fmt.Println("========================================")
}
