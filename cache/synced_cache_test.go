package cache

import (
	"context"
	"testing"
	"time"
)

func TestNewSyncedCache(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	if c == nil {
		t.Fatal("Cache should not be nil")
	}
}

func TestSyncedCacheSet(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testData := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	err = c.Set(ctx, "test:key", testData)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}
}

func TestSyncedCacheGet(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testValue := "test-value"
	key := "test:get"

	// Set value
	err = c.Set(ctx, key, testValue)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Get value
	value, found := c.Get(ctx, key)
	if !found {
		t.Fatal("Value should be found")
	}

	if value != testValue {
		t.Fatalf("Expected %v, got %v", testValue, value)
	}
}

func TestSyncedCacheDelete(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := "test:delete"
	testValue := "test-value"

	// Set value
	err = c.Set(ctx, key, testValue)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Delete value
	err = c.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Failed to delete value: %v", err)
	}

	// Verify deletion
	_, found := c.Get(ctx, key)
	if found {
		t.Fatal("Value should not be found after deletion")
	}
}

func TestSyncedCacheClear(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set multiple values
	for i := 0; i < 5; i++ {
		key := "test:clear:" + string(rune(i))
		err = c.Set(ctx, key, i)
		if err != nil {
			t.Fatalf("Failed to set value: %v", err)
		}
	}

	// Clear cache
	err = c.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}
}

func TestSyncedCacheStats(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	stats := c.Stats()
	if stats.LocalHits < 0 {
		t.Fatal("Stats should be valid")
	}
}

func TestSyncedCacheClose(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	err = c.Close()
	if err != nil {
		t.Fatalf("Failed to close cache: %v", err)
	}

	// Operations on closed cache should fail
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Set(ctx, "test", "value")
	if err == nil {
		t.Fatal("Set on closed cache should fail")
	}
}
