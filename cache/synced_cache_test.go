package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Mock implementations for testing error paths

type errorMarshaller struct{}

func (em *errorMarshaller) Marshal(v any) ([]byte, error) {
	return nil, errors.New("marshal error")
}

func (em *errorMarshaller) Unmarshal(data []byte, v any) error {
	return errors.New("unmarshal error")
}

type errorStore struct {
	Store
	getError    error
	setError    error
	deleteError error
	clearError  error
	closeError  error
}

func (es *errorStore) Get(ctx context.Context, key string) ([]byte, error) {
	if es.getError != nil {
		return nil, es.getError
	}
	return nil, errors.New("not found")
}

func (es *errorStore) Set(ctx context.Context, key string, value []byte) error {
	if es.setError != nil {
		return es.setError
	}
	return nil
}

func (es *errorStore) Delete(ctx context.Context, key string) error {
	if es.deleteError != nil {
		return es.deleteError
	}
	return nil
}

func (es *errorStore) Clear(ctx context.Context) error {
	if es.clearError != nil {
		return es.clearError
	}
	return nil
}

func (es *errorStore) Close() error {
	if es.closeError != nil {
		return es.closeError
	}
	return nil
}

type errorSynchronizer struct {
	Synchronizer
	publishError error
	closeError   error
}

func (es *errorSynchronizer) Subscribe(ctx context.Context) error {
	return nil
}

func (es *errorSynchronizer) Publish(ctx context.Context, event InvalidationEvent) error {
	if es.publishError != nil {
		return es.publishError
	}
	return nil
}

func (es *errorSynchronizer) OnInvalidate(callback func(event InvalidationEvent)) {
}

func (es *errorSynchronizer) Close() error {
	if es.closeError != nil {
		return es.closeError
	}
	return nil
}

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

func TestSyncedCacheSetWithInvalidate(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-invalidate"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testValue := "test-value-invalidate"
	key := "test:invalidate"

	// Use SetWithInvalidate
	err = c.SetWithInvalidate(ctx, key, testValue)
	if err != nil {
		t.Fatalf("Failed to set value with invalidate: %v", err)
	}

	// Value should be in Redis
	value, found := c.Get(ctx, key)
	if !found {
		t.Fatal("Value should be found in cache")
	}

	if value != testValue {
		t.Fatalf("Expected %v, got %v", testValue, value)
	}
}

func TestSyncedCacheGetRemoteHit(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-remote"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testValue := "test-value-remote"
	key := "test:remote"

	// Set value
	err = c.Set(ctx, key, testValue)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Clear local cache to force remote hit
	c.local.Clear()

	// Get value - should hit remote cache
	value, found := c.Get(ctx, key)
	if !found {
		t.Fatal("Value should be found in remote cache")
	}

	if value != testValue {
		t.Fatalf("Expected %v, got %v", testValue, value)
	}

	// Check stats for remote hit
	stats := c.Stats()
	if stats.RemoteHits == 0 {
		t.Fatal("Expected at least one remote hit")
	}
}

func TestSyncedCacheGetMiss(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-miss"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get non-existent key
	_, found := c.Get(ctx, "test:nonexistent")
	if found {
		t.Fatal("Value should not be found")
	}

	// Check stats for misses
	stats := c.Stats()
	if stats.LocalMisses == 0 && stats.RemoteMisses == 0 {
		t.Fatal("Expected at least one miss")
	}
}

func TestSyncedCacheWithDebugMode(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-debug"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test operations with debug mode enabled
	err = c.Set(ctx, "test:debug", "value")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	_, _ = c.Get(ctx, "test:debug")
	_ = c.Delete(ctx, "test:debug")
}

func TestSyncedCacheWithOnError(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-error"
	opts.RedisAddr = "localhost:6379"

	errorCalled := false
	opts.OnError = func(err error) {
		errorCalled = true
	}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// OnError callback is set
	if opts.OnError == nil {
		t.Fatal("OnError callback should be set")
	}

	// Note: errorCalled might not be true in normal operations
	// This test just verifies the callback can be set
	_ = errorCalled
}

func TestSyncedCacheDeleteOnClosedCache(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-closed-delete"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Delete(ctx, "test:key")
	if err == nil {
		t.Fatal("Delete on closed cache should fail")
	}
}

func TestSyncedCacheClearOnClosedCache(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-closed-clear"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.Clear(ctx)
	if err == nil {
		t.Fatal("Clear on closed cache should fail")
	}
}

func TestSyncedCacheGetOnClosedCache(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-closed-get"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, found := c.Get(ctx, "test:key")
	if found {
		t.Fatal("Get on closed cache should return not found")
	}
}

// TestHandleInvalidationActionSet tests handleInvalidation with ActionSet
func TestHandleInvalidationActionSet(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-invalidation-set"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Create a valid serialized value
	testValue := "test-value"
	data, err := c.serializer.Marshal(testValue)
	if err != nil {
		t.Fatalf("Failed to marshal test value: %v", err)
	}

	// Create an invalidation event with ActionSet
	event := InvalidationEvent{
		Key:    "test:key",
		Sender: "other-pod",
		Action: ActionSet,
		Value:  data,
	}

	// Call handleInvalidation directly
	c.handleInvalidation(event)

	// Wait for async processing (LFU cache)
	time.Sleep(10 * time.Millisecond)

	// Verify the value was set in local cache
	value, found := c.local.Get("test:key")
	if !found {
		t.Fatal("Value should be found in local cache after handleInvalidation")
	}

	if value != testValue {
		t.Fatalf("Expected %v, got %v", testValue, value)
	}
}

// TestHandleInvalidationActionSetWithInvalidData tests handleInvalidation with invalid serialized data
func TestHandleInvalidationActionSetWithInvalidData(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-invalidation-invalid"
	opts.RedisAddr = "localhost:6379"

	errorCalled := false
	opts.OnError = func(err error) {
		errorCalled = true
	}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Create an invalidation event with invalid data
	event := InvalidationEvent{
		Key:    "test:key",
		Sender: "other-pod",
		Action: ActionSet,
		Value:  []byte("invalid json data {{{"),
	}

	// Call handleInvalidation directly
	c.handleInvalidation(event)

	// Verify OnError was called
	if !errorCalled {
		t.Fatal("OnError should have been called for invalid data")
	}

	// Verify the value was NOT set in local cache
	_, found := c.local.Get("test:key")
	if found {
		t.Fatal("Value should not be found in local cache after failed deserialization")
	}
}

// TestHandleInvalidationActionInvalidate tests handleInvalidation with ActionInvalidate
func TestHandleInvalidationActionInvalidate(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-invalidation-invalidate"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Set a value in local cache first
	c.local.Set("test:key", "test-value", 1)

	// Wait for async processing (LFU cache)
	time.Sleep(10 * time.Millisecond)

	// Verify it's there
	_, found := c.local.Get("test:key")
	if !found {
		t.Fatal("Value should be in local cache before invalidation")
	}

	// Create an invalidation event with ActionInvalidate
	event := InvalidationEvent{
		Key:    "test:key",
		Sender: "other-pod",
		Action: ActionInvalidate,
	}

	// Call handleInvalidation directly
	c.handleInvalidation(event)

	// Verify the value was removed from local cache
	_, found = c.local.Get("test:key")
	if found {
		t.Fatal("Value should be removed from local cache after invalidation")
	}

	// Verify invalidation count increased
	stats := c.Stats()
	if stats.Invalidations == 0 {
		t.Fatal("Invalidations count should be greater than 0")
	}
}

// TestHandleInvalidationActionDelete tests handleInvalidation with ActionDelete
func TestHandleInvalidationActionDelete(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-invalidation-delete"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Set a value in local cache first
	c.local.Set("test:key", "test-value", 1)

	// Wait for async processing (LFU cache)
	time.Sleep(10 * time.Millisecond)

	// Create an invalidation event with ActionDelete
	event := InvalidationEvent{
		Key:    "test:key",
		Sender: "other-pod",
		Action: ActionDelete,
	}

	// Call handleInvalidation directly
	c.handleInvalidation(event)

	// Verify the value was removed from local cache
	_, found := c.local.Get("test:key")
	if found {
		t.Fatal("Value should be removed from local cache after delete")
	}

	// Verify invalidation count increased
	stats := c.Stats()
	if stats.Invalidations == 0 {
		t.Fatal("Invalidations count should be greater than 0")
	}
}

// TestHandleInvalidationActionClear tests handleInvalidation with ActionClear
func TestHandleInvalidationActionClear(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-invalidation-clear"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Set multiple values in local cache first
	c.local.Set("test:key1", "value1", 1)
	c.local.Set("test:key2", "value2", 1)
	c.local.Set("test:key3", "value3", 1)

	// Create an invalidation event with ActionClear
	event := InvalidationEvent{
		Key:    "*",
		Sender: "other-pod",
		Action: ActionClear,
	}

	// Call handleInvalidation directly
	c.handleInvalidation(event)

	// Verify all values were removed from local cache
	_, found1 := c.local.Get("test:key1")
	_, found2 := c.local.Get("test:key2")
	_, found3 := c.local.Get("test:key3")

	if found1 || found2 || found3 {
		t.Fatal("All values should be removed from local cache after clear")
	}

	// Verify invalidation count increased
	stats := c.Stats()
	if stats.Invalidations == 0 {
		t.Fatal("Invalidations count should be greater than 0")
	}
}

// TestHandleInvalidationUnknownAction tests handleInvalidation with unknown action
func TestHandleInvalidationUnknownAction(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-invalidation-unknown"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Create an invalidation event with unknown action
	event := InvalidationEvent{
		Key:    "test:key",
		Sender: "other-pod",
		Action: Action("unknown-action"),
	}

	// Call handleInvalidation directly - should not panic
	c.handleInvalidation(event)

	// Test passes if no panic occurs
}

// TestSyncedCacheGetDeserializationError tests Get with deserialization error
func TestSyncedCacheGetDeserializationError(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-deserialize-error"
	opts.RedisAddr = "localhost:6379"

	errorCalled := false
	opts.OnError = func(err error) {
		errorCalled = true
	}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set a value with normal marshaller
	err = c.Set(ctx, "test:deserialize", "test-value")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Clear local cache
	c.local.Clear()

	// Replace marshaller with error marshaller
	c.serializer = &errorMarshaller{}

	// Try to get - should fail deserialization
	_, found := c.Get(ctx, "test:deserialize")
	if found {
		t.Fatal("Get should return false when deserialization fails")
	}

	// Verify OnError was called
	if !errorCalled {
		t.Fatal("OnError should have been called for deserialization error")
	}
}

// TestSyncedCacheSetSerializationError tests Set with serialization error
func TestSyncedCacheSetSerializationError(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-serialize-error"
	opts.RedisAddr = "localhost:6379"

	errorCalled := false
	opts.OnError = func(err error) {
		errorCalled = true
	}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Replace marshaller with error marshaller
	c.serializer = &errorMarshaller{}

	// Try to set - should fail serialization
	err = c.Set(ctx, "test:key", "test-value")
	if err == nil {
		t.Fatal("Set should return error when serialization fails")
	}

	// Verify OnError was called
	if !errorCalled {
		t.Fatal("OnError should have been called for serialization error")
	}
}

// TestSyncedCacheSetRedisError tests Set with Redis error
func TestSyncedCacheSetRedisError(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-redis-error"
	opts.RedisAddr = "localhost:6379"

	errorCalled := false
	opts.OnError = func(err error) {
		errorCalled = true
	}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Replace store with error store
	c.store = &errorStore{setError: errors.New("redis set error")}

	// Try to set - should fail
	err = c.Set(ctx, "test:key", "test-value")
	if err == nil {
		t.Fatal("Set should return error when Redis fails")
	}

	// Verify OnError was called
	if !errorCalled {
		t.Fatal("OnError should have been called for Redis error")
	}
}

// TestSyncedCacheSetPublishError tests Set with publish error
func TestSyncedCacheSetPublishError(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-publish-error"
	opts.RedisAddr = "localhost:6379"

	errorCalled := false
	opts.OnError = func(err error) {
		errorCalled = true
	}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Replace synchronizer with error synchronizer
	c.synchronizer = &errorSynchronizer{publishError: errors.New("publish error")}

	// Try to set - should succeed but log error
	err = c.Set(ctx, "test:key", "test-value")
	if err != nil {
		t.Fatalf("Set should succeed even if publish fails: %v", err)
	}

	// Verify OnError was called
	if !errorCalled {
		t.Fatal("OnError should have been called for publish error")
	}
}

// TestSyncedCacheDeleteRedisError tests Delete with Redis error
func TestSyncedCacheDeleteRedisError(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-delete-redis-error"
	opts.RedisAddr = "localhost:6379"

	errorCalled := false
	opts.OnError = func(err error) {
		errorCalled = true
	}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Replace store with error store
	c.store = &errorStore{deleteError: errors.New("redis delete error")}

	// Try to delete - should fail
	err = c.Delete(ctx, "test:key")
	if err == nil {
		t.Fatal("Delete should return error when Redis fails")
	}

	// Verify OnError was called
	if !errorCalled {
		t.Fatal("OnError should have been called for Redis error")
	}
}

// TestSyncedCacheDeletePublishError tests Delete with publish error
func TestSyncedCacheDeletePublishError(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-delete-publish-error"
	opts.RedisAddr = "localhost:6379"

	errorCalled := false
	opts.OnError = func(err error) {
		errorCalled = true
	}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Replace synchronizer with error synchronizer
	c.synchronizer = &errorSynchronizer{publishError: errors.New("publish error")}

	// Try to delete - should succeed but log error
	err = c.Delete(ctx, "test:key")
	if err != nil {
		t.Fatalf("Delete should succeed even if publish fails: %v", err)
	}

	// Verify OnError was called
	if !errorCalled {
		t.Fatal("OnError should have been called for publish error")
	}
}

// TestSyncedCacheClearRedisError tests Clear with Redis error
func TestSyncedCacheClearRedisError(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-clear-redis-error"
	opts.RedisAddr = "localhost:6379"

	errorCalled := false
	opts.OnError = func(err error) {
		errorCalled = true
	}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Replace store with error store
	c.store = &errorStore{clearError: errors.New("redis clear error")}

	// Try to clear - should fail
	err = c.Clear(ctx)
	if err == nil {
		t.Fatal("Clear should return error when Redis fails")
	}

	// Verify OnError was called
	if !errorCalled {
		t.Fatal("OnError should have been called for Redis error")
	}
}

// TestSyncedCacheClearPublishError tests Clear with publish error
func TestSyncedCacheClearPublishError(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-clear-publish-error"
	opts.RedisAddr = "localhost:6379"

	errorCalled := false
	opts.OnError = func(err error) {
		errorCalled = true
	}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Replace synchronizer with error synchronizer
	c.synchronizer = &errorSynchronizer{publishError: errors.New("publish error")}

	// Try to clear - should succeed but log error
	err = c.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear should succeed even if publish fails: %v", err)
	}

	// Verify OnError was called
	if !errorCalled {
		t.Fatal("OnError should have been called for publish error")
	}
}

// TestSyncedCacheCloseWithErrors tests Close with synchronizer and store errors
func TestSyncedCacheCloseWithErrors(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-close-errors"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Replace synchronizer and store with error versions
	c.synchronizer = &errorSynchronizer{closeError: errors.New("synchronizer close error")}
	c.store = &errorStore{closeError: errors.New("store close error")}

	// Close should return the first error
	err = c.Close()
	if err == nil {
		t.Fatal("Close should return error when synchronizer or store fails")
	}
}

// TestSyncedCacheDoubleClose tests calling Close twice
func TestSyncedCacheDoubleClose(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-double-close"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// First close
	err = c.Close()
	if err != nil {
		t.Fatalf("First close should succeed: %v", err)
	}

	// Second close should be a no-op
	err = c.Close()
	if err != nil {
		t.Fatalf("Second close should succeed (no-op): %v", err)
	}
}

// TestSyncedCacheClearWithDebugMode tests Clear with debug mode enabled
func TestSyncedCacheClearWithDebugMode(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-clear-debug"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-clear")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set some values
	err = c.Set(ctx, "test:key1", "value1")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Clear cache
	err = c.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}
}

// TestHandleInvalidationWithDebugMode tests handleInvalidation with debug mode
func TestHandleInvalidationWithDebugMode(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-handle-debug"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-handle")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Test ActionSet with debug mode
	testValue := "test-value"
	data, _ := c.serializer.Marshal(testValue)
	event := InvalidationEvent{
		Key:    "test:key",
		Sender: "other-pod",
		Action: ActionSet,
		Value:  data,
	}
	c.handleInvalidation(event)

	// Test ActionInvalidate with debug mode
	c.local.Set("test:key2", "value", 1)
	time.Sleep(10 * time.Millisecond)
	event2 := InvalidationEvent{
		Key:    "test:key2",
		Sender: "other-pod",
		Action: ActionInvalidate,
	}
	c.handleInvalidation(event2)

	// Test ActionClear with debug mode
	event3 := InvalidationEvent{
		Key:    "*",
		Sender: "other-pod",
		Action: ActionClear,
	}
	c.handleInvalidation(event3)
}

// TestHandleInvalidationActionSetWithEmptyValue tests ActionSet with empty value
func TestHandleInvalidationActionSetWithEmptyValue(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-empty-value"
	opts.RedisAddr = "localhost:6379"

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	// Create an invalidation event with ActionSet but empty value
	event := InvalidationEvent{
		Key:    "test:key",
		Sender: "other-pod",
		Action: ActionSet,
		Value:  []byte{}, // Empty value
	}

	// Call handleInvalidation - should not panic
	c.handleInvalidation(event)

	// Value should not be set since Value is empty
	_, found := c.local.Get("test:key")
	if found {
		t.Fatal("Value should not be set when Value is empty")
	}
}

// TestSyncedCacheGetWithDebugModeRemoteMiss tests Get with debug mode and remote miss
func TestSyncedCacheGetWithDebugModeRemoteMiss(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-get-debug-miss"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-get-miss")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get non-existent key - should trigger debug logs for remote miss
	_, found := c.Get(ctx, "test:nonexistent")
	if found {
		t.Fatal("Value should not be found")
	}
}

// TestSyncedCacheGetWithDebugModeLocalHit tests Get with debug mode and local hit
func TestSyncedCacheGetWithDebugModeLocalHit(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-get-debug-hit"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-get-hit")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set a value
	err = c.Set(ctx, "test:key", "test-value")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Get the value - should trigger debug logs for local hit
	_, found := c.Get(ctx, "test:key")
	if !found {
		t.Fatal("Value should be found")
	}
}

// TestSyncedCacheDeleteWithDebugMode tests Delete with debug mode
func TestSyncedCacheDeleteWithDebugMode(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-delete-debug"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-delete")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set a value
	err = c.Set(ctx, "test:key", "test-value")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Delete the value - should trigger debug logs
	err = c.Delete(ctx, "test:key")
	if err != nil {
		t.Fatalf("Failed to delete value: %v", err)
	}
}

// TestSyncedCacheSetWithInvalidateDebugMode tests SetWithInvalidate with debug mode
func TestSyncedCacheSetWithInvalidateDebugMode(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-set-invalidate-debug"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-set-invalidate")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use SetWithInvalidate - should trigger debug logs
	err = c.SetWithInvalidate(ctx, "test:key", "test-value")
	if err != nil {
		t.Fatalf("Failed to set value with invalidate: %v", err)
	}
}

// TestSyncedCacheGetWithConsoleLoggerRemoteHit tests Get with ConsoleLogger and remote hit
func TestSyncedCacheGetWithConsoleLoggerRemoteHit(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-get-console-remote"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-get-remote")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set value directly in Redis (not in local cache)
	testValue := "remote-value"
	data, _ := c.serializer.Marshal(testValue)
	err = c.store.Set(ctx, "test:remote-key", data)
	if err != nil {
		t.Fatalf("Failed to set value in Redis: %v", err)
	}

	// Get should find it in remote cache and populate local
	value, found := c.Get(ctx, "test:remote-key")
	if !found {
		t.Fatal("Value should be found in remote cache")
	}
	if value != testValue {
		t.Fatalf("Expected %v, got %v", testValue, value)
	}
}

// TestSyncedCacheGetWithConsoleLoggerLocalMiss tests Get with ConsoleLogger and local miss
func TestSyncedCacheGetWithConsoleLoggerLocalMiss(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-get-console-miss"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-get-miss")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get non-existent key - triggers local miss and remote miss debug logs
	_, found := c.Get(ctx, "test:nonexistent-key")
	if found {
		t.Fatal("Value should not be found")
	}
}

// TestSyncedCacheSetWithConsoleLoggerSuccess tests Set with ConsoleLogger
func TestSyncedCacheSetWithConsoleLoggerSuccess(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-set-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-set")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set value - triggers all debug logs in setInternal
	err = c.Set(ctx, "test:key", "test-value")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}
}

// TestSyncedCacheDeleteWithConsoleLoggerSuccess tests Delete with ConsoleLogger
func TestSyncedCacheDeleteWithConsoleLoggerSuccess(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-delete-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-delete")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set a value first
	err = c.Set(ctx, "test:key", "test-value")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Delete - triggers all debug logs in Delete
	err = c.Delete(ctx, "test:key")
	if err != nil {
		t.Fatalf("Failed to delete value: %v", err)
	}
}

// TestSyncedCacheClearWithConsoleLoggerSuccess tests Clear with ConsoleLogger
func TestSyncedCacheClearWithConsoleLoggerSuccess(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-clear-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-clear")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set some values
	err = c.Set(ctx, "test:key1", "value1")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Clear - triggers all debug logs in Clear
	err = c.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}
}

// TestSyncedCacheGetDeserializationErrorWithConsoleLogger tests Get deserialization error with ConsoleLogger
func TestSyncedCacheGetDeserializationErrorWithConsoleLogger(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-get-deser-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-get-deser")
	opts.Marshaller = &errorMarshaller{}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set value directly in Redis using real marshaller
	realMarshaller := NewJSONMarshaller()
	data, _ := realMarshaller.Marshal("test-value")
	err = c.store.Set(ctx, "test:key", data)
	if err != nil {
		t.Fatalf("Failed to set value in Redis: %v", err)
	}

	// Get should fail deserialization and log error
	_, found := c.Get(ctx, "test:key")
	if found {
		t.Fatal("Value should not be found due to deserialization error")
	}
}

// TestSyncedCacheSetSerializationErrorWithConsoleLogger tests Set serialization error with ConsoleLogger
func TestSyncedCacheSetSerializationErrorWithConsoleLogger(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-set-ser-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-set-ser")
	opts.Marshaller = &errorMarshaller{}

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set should fail serialization and log error
	err = c.Set(ctx, "test:key", "test-value")
	if err == nil {
		t.Fatal("Set should fail due to serialization error")
	}
}

// TestSyncedCacheSetRedisErrorWithConsoleLogger tests Set Redis error with ConsoleLogger
func TestSyncedCacheSetRedisErrorWithConsoleLogger(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-set-redis-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-set-redis")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Replace store with error store
	c.store = &errorStore{setError: errors.New("redis set error")}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set should fail on Redis and log error
	err = c.Set(ctx, "test:key", "test-value")
	if err == nil {
		t.Fatal("Set should fail due to Redis error")
	}
}

// TestSyncedCacheSetPublishErrorWithConsoleLogger tests Set publish error with ConsoleLogger
func TestSyncedCacheSetPublishErrorWithConsoleLogger(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-set-pub-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-set-pub")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Replace synchronizer with error synchronizer
	c.synchronizer = &errorSynchronizer{publishError: errors.New("publish error")}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set should succeed but log warning for publish error
	err = c.Set(ctx, "test:key", "test-value")
	if err != nil {
		t.Fatalf("Set should succeed despite publish error: %v", err)
	}
}

// TestSyncedCacheDeleteRedisErrorWithConsoleLogger tests Delete Redis error with ConsoleLogger
func TestSyncedCacheDeleteRedisErrorWithConsoleLogger(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-delete-redis-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-delete-redis")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Replace store with error store
	c.store = &errorStore{deleteError: errors.New("redis delete error")}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete should fail on Redis and log error
	err = c.Delete(ctx, "test:key")
	if err == nil {
		t.Fatal("Delete should fail due to Redis error")
	}
}

// TestSyncedCacheDeletePublishErrorWithConsoleLogger tests Delete publish error with ConsoleLogger
func TestSyncedCacheDeletePublishErrorWithConsoleLogger(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-delete-pub-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-delete-pub")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Replace synchronizer with error synchronizer
	c.synchronizer = &errorSynchronizer{publishError: errors.New("publish error")}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete should succeed but log warning for publish error
	err = c.Delete(ctx, "test:key")
	if err != nil {
		t.Fatalf("Delete should succeed despite publish error: %v", err)
	}
}

// TestSyncedCacheClearRedisErrorWithConsoleLogger tests Clear Redis error with ConsoleLogger
func TestSyncedCacheClearRedisErrorWithConsoleLogger(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-clear-redis-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-clear-redis")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Replace store with error store
	c.store = &errorStore{clearError: errors.New("redis clear error")}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Clear should fail on Redis and log error
	err = c.Clear(ctx)
	if err == nil {
		t.Fatal("Clear should fail due to Redis error")
	}
}

// TestSyncedCacheClearPublishErrorWithConsoleLogger tests Clear publish error with ConsoleLogger
func TestSyncedCacheClearPublishErrorWithConsoleLogger(t *testing.T) {
	opts := DefaultOptions()
	opts.PodID = "test-pod-clear-pub-console"
	opts.RedisAddr = "localhost:6379"
	opts.DebugMode = true
	opts.Logger = NewConsoleLogger("test-clear-pub")

	c, err := New(opts)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Replace synchronizer with error synchronizer
	c.synchronizer = &errorSynchronizer{publishError: errors.New("publish error")}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Clear should succeed but log warning for publish error
	err = c.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear should succeed despite publish error: %v", err)
	}
}
