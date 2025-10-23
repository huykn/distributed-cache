package cache

import (
	"testing"
	"time"
)

func TestLFUCacheNew(t *testing.T) {
	config := DefaultLocalCacheConfig()
	cache, err := NewLFUCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}
}

func TestLFUCacheSet(t *testing.T) {
	config := DefaultLocalCacheConfig()
	cache, err := NewLFUCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	ok := cache.Set("key1", "value1", 1)
	if !ok {
		t.Fatal("Set should succeed")
	}
}

func TestLFUCacheGet(t *testing.T) {
	config := DefaultLocalCacheConfig()
	cache, err := NewLFUCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	cache.Set("key1", "value1", 1)
	time.Sleep(10 * time.Millisecond) // Wait for async processing

	value, found := cache.Get("key1")
	if !found {
		t.Fatal("Value should be found")
	}

	if value != "value1" {
		t.Fatalf("Expected 'value1', got %v", value)
	}
}

func TestLFUCacheDelete(t *testing.T) {
	config := DefaultLocalCacheConfig()
	cache, err := NewLFUCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	cache.Set("key1", "value1", 1)
	cache.Delete("key1")

	_, found := cache.Get("key1")
	if found {
		t.Fatal("Value should not be found after deletion")
	}
}

func TestLFUCacheClear(t *testing.T) {
	config := DefaultLocalCacheConfig()
	cache, err := NewLFUCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	cache.Set("key1", "value1", 1)
	cache.Set("key2", "value2", 1)
	cache.Clear()

	_, found1 := cache.Get("key1")
	_, found2 := cache.Get("key2")

	if found1 || found2 {
		t.Fatal("Cache should be empty after clear")
	}
}

func TestLFUCacheMetrics(t *testing.T) {
	config := DefaultLocalCacheConfig()
	cache, err := NewLFUCache(config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	cache.Set("key1", "value1", 1)
	time.Sleep(10 * time.Millisecond) // Wait for async processing
	cache.Get("key1")                 // Hit
	cache.Get("key2")                 // Miss

	metrics := cache.Metrics()
	if metrics.Hits != 1 {
		t.Fatalf("Expected 1 hit, got %d", metrics.Hits)
	}

	if metrics.Misses != 1 {
		t.Fatalf("Expected 1 miss, got %d", metrics.Misses)
	}
}
