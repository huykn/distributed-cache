package cache

import (
	"testing"
)

func TestLRUCacheNew(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}

	if cache.maxSize != 100 {
		t.Fatalf("Expected maxSize 100, got %d", cache.maxSize)
	}
}

func TestLRUCacheNewWithZeroSize(t *testing.T) {
	_, err := NewLRUCache(0)
	if err == nil {
		t.Fatal("Expected error when creating cache with size 0")
	}
}

func TestLRUCacheNewWithNegativeSize(t *testing.T) {
	_, err := NewLRUCache(-1)
	if err == nil {
		t.Fatal("Expected error when creating cache with negative size")
	}
}

func TestLRUCacheSet(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	ok := cache.Set("key1", "value1", 1)
	if !ok {
		t.Fatal("Set should succeed")
	}
}

func TestLRUCacheSetMultiple(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	for i := 0; i < 10; i++ {
		key := "key" + string(rune('0'+i))
		value := "value" + string(rune('0'+i))
		ok := cache.Set(key, value, 1)
		if !ok {
			t.Fatalf("Set should succeed for key %s", key)
		}
	}
}

func TestLRUCacheGet(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	cache.Set("key1", "value1", 1)

	value, found := cache.Get("key1")
	if !found {
		t.Fatal("Value should be found")
	}

	if value != "value1" {
		t.Fatalf("Expected 'value1', got %v", value)
	}
}

func TestLRUCacheGetNotFound(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	_, found := cache.Get("nonexistent")
	if found {
		t.Fatal("Value should not be found")
	}
}

func TestLRUCacheGetAfterUpdate(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	cache.Set("key1", "value1", 1)
	cache.Set("key1", "value2", 1) // Update

	value, found := cache.Get("key1")
	if !found {
		t.Fatal("Value should be found")
	}

	if value != "value2" {
		t.Fatalf("Expected 'value2', got %v", value)
	}
}

func TestLRUCacheDelete(t *testing.T) {
	cache, err := NewLRUCache(100)
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

func TestLRUCacheDeleteNonexistent(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	// Should not panic
	cache.Delete("nonexistent")
}

func TestLRUCacheClear(t *testing.T) {
	cache, err := NewLRUCache(100)
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

func TestLRUCacheMetrics(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	cache.Set("key1", "value1", 1)
	cache.Get("key1") // Hit
	cache.Get("key2") // Miss

	metrics := cache.Metrics()
	if metrics.Hits != 1 {
		t.Fatalf("Expected 1 hit, got %d", metrics.Hits)
	}

	if metrics.Misses != 1 {
		t.Fatalf("Expected 1 miss, got %d", metrics.Misses)
	}

	if metrics.Size != 100 {
		t.Fatalf("Expected size 100, got %d", metrics.Size)
	}
}

func TestLRUCacheMetricsMultipleHits(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	cache.Set("key1", "value1", 1)
	cache.Set("key2", "value2", 1)

	// Multiple hits
	cache.Get("key1")
	cache.Get("key1")
	cache.Get("key2")

	// Multiple misses
	cache.Get("key3")
	cache.Get("key4")

	metrics := cache.Metrics()
	if metrics.Hits != 3 {
		t.Fatalf("Expected 3 hits, got %d", metrics.Hits)
	}

	if metrics.Misses != 2 {
		t.Fatalf("Expected 2 misses, got %d", metrics.Misses)
	}
}

func TestLRUCacheClose(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	cache.Set("key1", "value1", 1)
	cache.Close()

	// After close, cache should be empty
	_, found := cache.Get("key1")
	if found {
		t.Fatal("Cache should be empty after close")
	}
}

func TestLRUCacheFactory(t *testing.T) {
	factory := NewLRUCacheFactory(100)
	if factory == nil {
		t.Fatal("Factory should not be nil")
	}

	cache, err := factory.Create()
	if err != nil {
		t.Fatalf("Failed to create cache from factory: %v", err)
	}
	defer cache.Close()

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}
}

func TestLRUCacheFactoryCreate(t *testing.T) {
	factory := NewLRUCacheFactory(50)
	cache, err := factory.Create()
	if err != nil {
		t.Fatalf("Failed to create cache from factory: %v", err)
	}
	defer cache.Close()

	// Test that the cache works
	cache.Set("test", "value", 1)
	value, found := cache.Get("test")
	if !found {
		t.Fatal("Value should be found")
	}
	if value != "value" {
		t.Fatalf("Expected 'value', got %v", value)
	}
}

func TestLRUCacheSetWithDifferentTypes(t *testing.T) {
	cache, err := NewLRUCache(100)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	// Test with string
	cache.Set("key1", "string value", 1)
	v1, found := cache.Get("key1")
	if !found || v1 != "string value" {
		t.Fatal("String value should be stored and retrieved")
	}

	// Test with int
	cache.Set("key2", 42, 1)
	v2, found := cache.Get("key2")
	if !found || v2 != 42 {
		t.Fatal("Int value should be stored and retrieved")
	}

	// Test with struct
	type testStruct struct {
		Name string
		Age  int
	}
	cache.Set("key3", testStruct{Name: "John", Age: 30}, 1)
	v3, found := cache.Get("key3")
	if !found {
		t.Fatal("Struct value should be stored and retrieved")
	}
	s, ok := v3.(testStruct)
	if !ok || s.Name != "John" || s.Age != 30 {
		t.Fatal("Struct value should match")
	}
}
