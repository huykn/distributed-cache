package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
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

// SimpleMapCache is a custom local cache implementation using a simple map[string]any.
// This demonstrates how to implement the LocalCache interface with a basic in-memory store.
type SimpleMapCache struct {
	mu        sync.RWMutex
	data      map[string]any
	hits      int64
	misses    int64
	evictions int64
	maxSize   int
}

// NewSimpleMapCache creates a new simple map-based cache.
func NewSimpleMapCache(maxSize int) *SimpleMapCache {
	return &SimpleMapCache{
		data:    make(map[string]any),
		maxSize: maxSize,
	}
}

// Get retrieves a value from the local cache.
func (c *SimpleMapCache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, found := c.data[key]
	if found {
		atomic.AddInt64(&c.hits, 1)
	} else {
		atomic.AddInt64(&c.misses, 1)
	}
	return value, found
}

// Set stores a value in the local cache.
// If the cache is full, it evicts a random entry (simple eviction strategy).
func (c *SimpleMapCache) Set(key string, value any, cost int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to evict an entry
	if _, exists := c.data[key]; !exists && len(c.data) >= c.maxSize {
		// Simple eviction: remove a random entry
		// In a real implementation, you might use LRU, LFU, or another strategy
		for k := range c.data {
			delete(c.data, k)
			atomic.AddInt64(&c.evictions, 1)
			break
		}
	}

	c.data[key] = value
	return true
}

// Delete removes a value from the local cache.
func (c *SimpleMapCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// Clear removes all values from the local cache.
func (c *SimpleMapCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]any)
}

// Close closes the local cache.
func (c *SimpleMapCache) Close() {
	c.Clear()
}

// Metrics returns cache metrics.
func (c *SimpleMapCache) Metrics() cache.LocalCacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return cache.LocalCacheMetrics{
		Hits:      atomic.LoadInt64(&c.hits),
		Misses:    atomic.LoadInt64(&c.misses),
		Evictions: atomic.LoadInt64(&c.evictions),
		Size:      int64(len(c.data)),
	}
}

// SimpleMapCacheFactory creates SimpleMapCache instances.
type SimpleMapCacheFactory struct {
	maxSize int
}

// NewSimpleMapCacheFactory creates a new factory for simple map-based caches.
func NewSimpleMapCacheFactory(maxSize int) cache.LocalCacheFactory {
	return &SimpleMapCacheFactory{maxSize: maxSize}
}

// Create creates a new SimpleMapCache instance.
func (f *SimpleMapCacheFactory) Create() (cache.LocalCache, error) {
	return NewSimpleMapCache(f.maxSize), nil
}

func main() {
	fmt.Println("========================================")
	fmt.Println("Custom Local Cache Example")
	fmt.Println("========================================")
	fmt.Println()

	// This example demonstrates how to implement a custom local cache
	// using a simple map[string]any as the underlying storage mechanism.
	//
	// Custom Cache Implementation:
	// - Uses sync.RWMutex for thread-safe access
	// - Implements all required LocalCache interface methods
	// - Tracks metrics (hits, misses, evictions)
	// - Simple eviction strategy (random when full)
	//
	// The LocalCache interface requires:
	// - Get(key string) (any, bool)
	// - Set(key string, value any, cost int64) bool
	// - Delete(key string)
	// - Clear()
	// - Close()
	// - Metrics() LocalCacheMetrics

	fmt.Println("Custom Cache Overview:")
	fmt.Println("  Implementation: Simple map[string]any")
	fmt.Println("  Thread-safety: sync.RWMutex")
	fmt.Println("  Eviction: Random (when full)")
	fmt.Println("  Metrics: Hits, Misses, Evictions, Size")
	fmt.Println()

	// Create configuration with custom cache
	cfg := dc.DefaultConfig()
	cfg.PodID = "custom-cache-pod"
	cfg.RedisAddr = "localhost:6379"

	// Configure custom cache with maximum size
	maxSize := 100 // Maximum number of items in cache

	// Use custom cache factory
	cfg.LocalCacheFactory = NewSimpleMapCacheFactory(maxSize)

	fmt.Println("Custom Cache Configuration:")
	fmt.Printf("  MaxSize: %d items\n", maxSize)
	fmt.Printf("  Storage: map[string]any\n")
	fmt.Printf("  Eviction: Random entry when full\n")
	fmt.Println()

	// Initialize cache
	fmt.Println("Initializing cache with custom implementation...")
	cacheInstance, err := dc.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer cacheInstance.Close()

	fmt.Println("✓ Cache initialized successfully")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Demonstrate custom cache behavior
	fmt.Println("Demonstrating custom cache behavior...")
	fmt.Println("========================================")

	// Example 1: Basic Set and Get operations
	fmt.Println("\nExample 1: Basic Set and Get")
	fmt.Println("----------------------------")

	user1 := User{
		ID:    1,
		Name:  "Alice Johnson",
		Email: "alice@example.com",
	}

	fmt.Printf("Setting user:1 = %+v\n", user1)
	if err := cacheInstance.Set(ctx, "user:1", user1); err != nil {
		log.Printf("Error setting user:1: %v", err)
	}

	// Get from cache
	value, found := cacheInstance.Get(ctx, "user:1")
	if found {
		fmt.Printf("✓ Got user:1 = %+v\n", value)
	} else {
		fmt.Println("✗ user:1 not found")
	}

	// Example 2: Multiple operations
	fmt.Println("\nExample 2: Multiple Set Operations")
	fmt.Println("-----------------------------------")

	fmt.Println("Adding 10 users to cache...")
	for i := 2; i <= 11; i++ {
		user := User{
			ID:    i,
			Name:  fmt.Sprintf("User%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
		}
		if err := cacheInstance.Set(ctx, fmt.Sprintf("user:%d", i), user); err != nil {
			log.Printf("Error setting user:%d: %v", i, err)
		}
	}
	fmt.Println("✓ 10 users added")

	// Example 3: Cache hits and misses
	fmt.Println("\nExample 3: Cache Hits and Misses")
	fmt.Println("---------------------------------")

	// Access existing keys (hits)
	fmt.Println("Accessing existing keys...")
	for i := 1; i <= 5; i++ {
		_, found := cacheInstance.Get(ctx, fmt.Sprintf("user:%d", i))
		if found {
			fmt.Printf("  ✓ user:%d found (hit)\n", i)
		} else {
			fmt.Printf("  ✗ user:%d not found (miss)\n", i)
		}
	}

	// Access non-existing keys (misses)
	fmt.Println("\nAccessing non-existing keys...")
	for i := 100; i <= 102; i++ {
		_, found := cacheInstance.Get(ctx, fmt.Sprintf("user:%d", i))
		if found {
			fmt.Printf("  ✓ user:%d found (hit)\n", i)
		} else {
			fmt.Printf("  ✗ user:%d not found (miss)\n", i)
		}
	}

	// Example 4: Delete operation
	fmt.Println("\nExample 4: Delete Operation")
	fmt.Println("---------------------------")

	fmt.Println("Deleting user:5...")
	if err := cacheInstance.Delete(ctx, "user:5"); err != nil {
		log.Printf("Error deleting user:5: %v", err)
	}

	_, found = cacheInstance.Get(ctx, "user:5")
	if !found {
		fmt.Println("✓ user:5 successfully deleted")
	}

	// Example 5: Cache statistics
	fmt.Println("\nExample 5: Cache Statistics")
	fmt.Println("---------------------------")

	stats := cacheInstance.Stats()
	fmt.Printf("Local Hits:      %d\n", stats.LocalHits)
	fmt.Printf("Local Misses:    %d\n", stats.LocalMisses)
	fmt.Printf("Remote Hits:     %d\n", stats.RemoteHits)
	fmt.Printf("Remote Misses:   %d\n", stats.RemoteMisses)
	fmt.Printf("Invalidations:   %d\n", stats.Invalidations)

	if stats.LocalHits+stats.LocalMisses > 0 {
		hitRatio := float64(stats.LocalHits) / float64(stats.LocalHits+stats.LocalMisses) * 100
		fmt.Printf("Local Hit Ratio: %.2f%%\n", hitRatio)
	}

	// Example 6: Eviction behavior
	fmt.Println("\nExample 6: Eviction Behavior")
	fmt.Println("----------------------------")

	fmt.Printf("Current cache max size: %d items\n", maxSize)
	fmt.Println("Adding items beyond max size to trigger eviction...")

	// Add more items to trigger eviction
	for i := 50; i <= 60; i++ {
		user := User{
			ID:    i,
			Name:  fmt.Sprintf("User%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
		}
		if err := cacheInstance.Set(ctx, fmt.Sprintf("user:%d", i), user); err != nil {
			log.Printf("Error setting user:%d: %v", i, err)
		}
	}

	fmt.Println("✓ Items added (some older items may have been evicted)")

	fmt.Println("\n========================================")
	fmt.Println()
	fmt.Println("Custom Cache Implementation Summary:")
	fmt.Println("  ✓ Implements LocalCache interface")
	fmt.Println("  ✓ Thread-safe with sync.RWMutex")
	fmt.Println("  ✓ Tracks metrics (hits, misses, evictions)")
	fmt.Println("  ✓ Simple eviction strategy")
	fmt.Println("  ✓ Integrates with distributed-cache library")
	fmt.Println()
	fmt.Println("Key Implementation Points:")
	fmt.Println("  1. Implement all LocalCache interface methods")
	fmt.Println("  2. Ensure thread-safety for concurrent access")
	fmt.Println("  3. Track metrics using atomic operations")
	fmt.Println("  4. Create a factory that implements LocalCacheFactory")
	fmt.Println("  5. Pass factory to cache configuration")
	fmt.Println()
	fmt.Println("When to Use Custom Cache:")
	fmt.Println("  ✓ Need specific eviction strategy")
	fmt.Println("  ✓ Have unique performance requirements")
	fmt.Println("  ✓ Want to integrate existing cache implementation")
	fmt.Println("  ✓ Need custom metrics or monitoring")
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Example completed successfully!")
	fmt.Println("========================================")
}
