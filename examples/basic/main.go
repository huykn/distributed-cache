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
	// Example 1: Using root-level initialization (recommended)
	fmt.Println("=== Using Root-Level Initialization ===")
	rootCfg := dc.DefaultConfig()
	rootCfg.PodID = "example-pod-1"
	rootCfg.RedisAddr = "localhost:6379"
	rootCfg.DebugMode = true // Enable debug logging
	rootCfg.Logger = cache.NewConsoleLogger("pod-1")

	c, err := dc.New(rootCfg)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Example 2: Using cache package directly (backward compatible)
	fmt.Println("\n=== Using Cache Package Directly ===")
	cacheCfg := cache.DefaultOptions()
	cacheCfg.PodID = "example-pod-2"
	cacheCfg.RedisAddr = "localhost:6379"
	cacheCfg.DebugMode = true
	cacheCfg.Logger = cache.NewConsoleLogger("pod-2")

	c2, err := cache.New(cacheCfg)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer c2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Example 1: Set and Get
	fmt.Println("=== Example 1: Set and Get ===")
	user := User{
		ID:    123,
		Name:  "John Doe",
		Email: "john@example.com",
	}

	key := "user:123"
	if err := c.Set(ctx, key, user); err != nil {
		log.Fatalf("Failed to set value: %v", err)
	}
	fmt.Printf("Set %s = %+v\n", key, user)

	// Get from cache (should hit local cache)
	value, found := c.Get(ctx, key)
	if found {
		fmt.Printf("Got %s = %+v\n", key, value)
	}

	// Example 2: Multi-Pod Cache Synchronization with Value Propagation
	fmt.Println("\n=== Example 2: Multi-Pod Cache Synchronization (Value Propagation) ===")
	key2 := "user:456"
	user2 := User{
		ID:    456,
		Name:  "Jane Smith",
		Email: "jane@example.com",
	}

	fmt.Println("Pod-1 sets a value...")
	if err := c.Set(ctx, key2, user2); err != nil {
		log.Fatalf("Failed to set value: %v", err)
	}
	fmt.Printf("  Pod-1: Set %s = %+v\n", key2, user2)

	// Simulate another pod joining (in real scenario, this would be another pod)
	fmt.Println("\nCreating Pod-3 (simulating another pod in the cluster)...")
	cacheCfg2 := cache.DefaultOptions()
	cacheCfg2.PodID = "example-pod-3"
	cacheCfg2.RedisAddr = "localhost:6379"
	cacheCfg2.DebugMode = true
	cacheCfg2.Logger = cache.NewConsoleLogger("pod-3")
	c3, err := cache.New(cacheCfg2)
	if err != nil {
		log.Fatalf("Failed to create second cache: %v", err)
	}
	defer c3.Close()
	fmt.Println("  Pod-3: Initialized and subscribed to synchronization events")

	// Pod-3 fetches from Redis (local cache is empty, so it's a remote hit)
	fmt.Println("\nPod-3 fetches the value (should come from Redis)...")
	value2, found2 := c3.Get(ctx, key2)
	if found2 {
		fmt.Printf("  Pod-3: Got %s from remote = %+v\n", key2, value2)
	}

	// Now Pod-3 updates the value - this should propagate to Pod-1's local cache
	fmt.Println("\nPod-3 updates the value (should propagate to Pod-1's local cache)...")
	user2Updated := User{
		ID:    456,
		Name:  "Jane Smith-Updated",
		Email: "jane.updated@example.com",
	}
	if err := c3.Set(ctx, key2, user2Updated); err != nil {
		log.Fatalf("Failed to set value: %v", err)
	}
	fmt.Printf("  Pod-3: Updated %s = %+v (sent value to other pods)\n", key2, user2Updated)

	// Give time for the synchronization event to propagate
	fmt.Println("\nWaiting for synchronization event to propagate...")
	time.Sleep(100 * time.Millisecond)

	// Pod-1 should now have the updated value in local cache (no Redis fetch needed!)
	fmt.Println("\nPod-1 fetches the value (should be in local cache from propagation)...")
	value3, found3 := c.Get(ctx, key2)
	if found3 {
		fmt.Printf("  Pod-1: Got %s = %+v\n", key2, value3)
		if userVal, ok := value3.(User); ok && userVal.Name == "Jane Smith-Updated" {
			fmt.Println("  ✓ Value propagation successful! Pod-1 got the updated value from local cache.")
		}
	}

	// Example 3: SetWithInvalidate (Invalidate-Only Mode)
	fmt.Println("\n=== Example 3: SetWithInvalidate (Invalidate-Only Mode) ===")
	fmt.Println("This mode is useful for large values or when you want lazy loading.")

	key3 := "user:789"
	user3 := User{
		ID:    789,
		Name:  "Bob Johnson",
		Email: "bob@example.com",
	}

	// Pod-1 sets a value with invalidate-only mode
	fmt.Println("\nPod-1 sets a value with invalidate-only mode...")
	if err := c.SetWithInvalidate(ctx, key3, user3); err != nil {
		log.Fatalf("Failed to set value: %v", err)
	}
	fmt.Printf("  Pod-1: Set %s = %+v (invalidate-only mode)\n", key3, user3)

	// Give time for the invalidation event to propagate
	fmt.Println("\nWaiting for invalidation event to propagate...")
	time.Sleep(100 * time.Millisecond)

	// Pod-3 should NOT have the value in local cache (it was invalidated, not propagated)
	fmt.Println("\nPod-3 fetches the value (should fetch from Redis, not local cache)...")
	value4, found4 := c3.Get(ctx, key3)
	if found4 {
		fmt.Printf("  Pod-3: Got %s = %+v (fetched from Redis)\n", key3, value4)
		fmt.Println("  ✓ Invalidate-only mode works! Pod-3 fetched from Redis instead of local cache.")
	}

	// Example 4: Delete and Synchronization
	fmt.Println("\n=== Example 4: Delete and Synchronization ===")
	fmt.Println("Pod-1 will delete a key, and Pod-3 should receive the delete event...")

	// First, verify that c3 has the key in local cache
	fmt.Println("\nBefore delete:")
	_, found = c3.Get(ctx, key)
	fmt.Printf("  Pod-3: Get %s, found = %v\n", key, found)

	// Pod-1 deletes the key
	if err := c.Delete(ctx, key); err != nil {
		log.Fatalf("Failed to delete value: %v", err)
	}
	fmt.Printf("\n  Pod-1: Deleted %s and published delete event\n", key)

	// Give time for the delete event to propagate via Redis Pub/Sub
	fmt.Println("\nWaiting for delete event to propagate...")
	time.Sleep(100 * time.Millisecond)

	// Now check both caches
	fmt.Println("\nAfter delete:")
	_, found = c.Get(ctx, key)
	fmt.Printf("  Pod-1: Get %s, found = %v (should be false)\n", key, found)

	_, found = c3.Get(ctx, key)
	fmt.Printf("  Pod-3: Get %s, found = %v (should be false - deleted!)\n", key, found)

	// Example 5: Statistics
	fmt.Println("\n=== Example 5: Statistics ===")
	stats := c.Stats()
	fmt.Printf("Cache Statistics:\n")
	fmt.Printf("  Local Hits:      %d\n", stats.LocalHits)
	fmt.Printf("  Local Misses:    %d\n", stats.LocalMisses)
	fmt.Printf("  Remote Hits:     %d\n", stats.RemoteHits)
	fmt.Printf("  Remote Misses:   %d\n", stats.RemoteMisses)
	fmt.Printf("  Invalidations:   %d\n", stats.Invalidations)

	fmt.Println("\n=== Example Complete ===")
	fmt.Println("\nKey Takeaways:")
	fmt.Println("  • Set() propagates values to other pods (better cache hit rates)")
	fmt.Println("  • SetWithInvalidate() only invalidates on other pods (useful for large values)")
	fmt.Println("  • Delete() removes entries from all pods")
	fmt.Println("  • Value propagation reduces Redis/DB queries across the cluster")
}
