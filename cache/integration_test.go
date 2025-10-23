//go:build integration
// +build integration

package cache

import (
	"context"
	"testing"
	"time"
)

// TestIntegrationTwoLevelCache tests the two-level caching behavior.
func TestIntegrationTwoLevelCache(t *testing.T) {
	opts1 := DefaultOptions()
	opts1.PodID = "pod-1"
	opts1.RedisAddr = "localhost:6379"

	c1, err := New(opts1)
	if err != nil {
		t.Fatalf("Failed to create cache 1: %v", err)
	}
	defer c1.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set value in pod 1
	testValue := map[string]any{
		"id":   123,
		"name": "Test User",
	}

	key := "user:123"
	err = c1.Set(ctx, key, testValue)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Get from local cache (should hit)
	value, found := c1.Get(ctx, key)
	if !found {
		t.Fatal("Value should be found in local cache")
	}

	if value == nil {
		t.Fatal("Value should not be nil")
	}

	// Create second cache instance (simulating another pod)
	opts2 := DefaultOptions()
	opts2.PodID = "pod-2"
	opts2.RedisAddr = "localhost:6379"

	c2, err := New(opts2)
	if err != nil {
		t.Fatalf("Failed to create cache 2: %v", err)
	}
	defer c2.Close()

	// Get from remote cache (should fetch from Redis)
	value2, found2 := c2.Get(ctx, key)
	if !found2 {
		t.Fatal("Value should be found in remote cache")
	}

	if value2 == nil {
		t.Fatal("Value should not be nil")
	}
}

// TestIntegrationCacheInvalidation tests cache invalidation across pods.
func TestIntegrationCacheInvalidation(t *testing.T) {
	opts1 := DefaultOptions()
	opts1.PodID = "pod-1"
	opts1.RedisAddr = "localhost:6379"

	c1, err := New(opts1)
	if err != nil {
		t.Fatalf("Failed to create cache 1: %v", err)
	}
	defer c1.Close()

	opts2 := DefaultOptions()
	opts2.PodID = "pod-2"
	opts2.RedisAddr = "localhost:6379"

	c2, err := New(opts2)
	if err != nil {
		t.Fatalf("Failed to create cache 2: %v", err)
	}
	defer c2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := "test:invalidation"
	testValue := "test-value"

	// Set value in pod 1
	err = c1.Set(ctx, key, testValue)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Get in pod 2 to populate local cache
	_, found := c2.Get(ctx, key)
	if !found {
		t.Fatal("Value should be found in pod 2")
	}

	// Wait a bit for pub/sub to propagate
	time.Sleep(100 * time.Millisecond)

	// Delete in pod 1
	err = c1.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Failed to delete value: %v", err)
	}

	// Wait for invalidation event
	time.Sleep(100 * time.Millisecond)

	// Value should be gone from pod 2's local cache
	_, found = c2.Get(ctx, key)
	if found {
		t.Fatal("Value should be invalidated in pod 2")
	}
}

// TestIntegrationMultipleOperations tests multiple cache operations.
func TestIntegrationMultipleOperations(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set multiple values
	for i := 0; i < 10; i++ {
		key := "test:" + string(rune(i))
		err = c.Set(ctx, key, i)
		if err != nil {
			t.Fatalf("Failed to set value: %v", err)
		}
	}

	// Get all values
	for i := 0; i < 10; i++ {
		key := "test:" + string(rune(i))
		value, found := c.Get(ctx, key)
		if !found {
			t.Fatalf("Value not found for key %s", key)
		}
		if value != i {
			t.Fatalf("Expected %d, got %v", i, value)
		}
	}

	// Delete some values
	for i := 0; i < 5; i++ {
		key := "test:" + string(rune(i))
		err = c.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Failed to delete value: %v", err)
		}
	}

	// Verify deletions
	for i := 0; i < 5; i++ {
		key := "test:" + string(rune(i))
		_, found := c.Get(ctx, key)
		if found {
			t.Fatalf("Value should not be found for key %s", key)
		}
	}
}
