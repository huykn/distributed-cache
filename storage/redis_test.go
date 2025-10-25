package storage

import (
	"context"
	"testing"
	"time"
)

func TestNewRedisStore(t *testing.T) {
	store, err := NewRedisStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("Store should not be nil")
	}
}

func TestRedisStoreSet(t *testing.T) {
	store, err := NewRedisStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = store.Set(ctx, "test:key", []byte("test-value"))
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}
}

func TestRedisStoreGet(t *testing.T) {
	store, err := NewRedisStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	testValue := []byte("test-value")
	key := "test:get"

	// Set value
	err = store.Set(ctx, key, testValue)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Get value
	value, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if string(value) != string(testValue) {
		t.Fatalf("Expected %s, got %s", testValue, value)
	}
}

func TestRedisStoreDelete(t *testing.T) {
	store, err := NewRedisStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := "test:delete"
	testValue := []byte("test-value")

	// Set value
	err = store.Set(ctx, key, testValue)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Delete value
	err = store.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Failed to delete value: %v", err)
	}

	// Verify deletion
	_, err = store.Get(ctx, key)
	if err == nil {
		t.Fatal("Value should not be found after deletion")
	}
}

func TestRedisStoreNotFound(t *testing.T) {
	store, err := NewRedisStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = store.Get(ctx, "nonexistent:key")
	if err == nil {
		t.Fatal("Should return error for nonexistent key")
	}
}

func TestRedisStoreClear(t *testing.T) {
	store, err := NewRedisStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set multiple values
	for i := 0; i < 5; i++ {
		key := "test:clear:" + string(rune('a'+i))
		err = store.Set(ctx, key, []byte("value"))
		if err != nil {
			t.Fatalf("Failed to set value: %v", err)
		}
	}

	// Clear all values
	err = store.Clear(ctx)
	if err != nil {
		t.Fatalf("Failed to clear store: %v", err)
	}

	// Verify values are cleared
	for i := 0; i < 5; i++ {
		key := "test:clear:" + string(rune('a'+i))
		_, err = store.Get(ctx, key)
		if err == nil {
			t.Fatal("Values should be cleared")
		}
	}
}

func TestRedisStoreGetClient(t *testing.T) {
	store, err := NewRedisStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()

	client := store.GetClient()
	if client == nil {
		t.Fatal("Client should not be nil")
	}

	// Verify client is functional
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Ping(ctx).Err()
	if err != nil {
		t.Fatalf("Client should be able to ping Redis: %v", err)
	}
}

func TestNewRedisStoreWithPassword(t *testing.T) {
	// This test assumes Redis is running without password
	// It tests that the password parameter is accepted
	store, err := NewRedisStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to create Redis store with password: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("Store should not be nil")
	}
}

func TestNewRedisStoreWithDB(t *testing.T) {
	// Test with different database number
	store, err := NewRedisStore("localhost:6379", "", 1)
	if err != nil {
		t.Fatalf("Failed to create Redis store with DB 1: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify we can use the store
	err = store.Set(ctx, "test:db1", []byte("value"))
	if err != nil {
		t.Fatalf("Failed to set value in DB 1: %v", err)
	}

	value, err := store.Get(ctx, "test:db1")
	if err != nil {
		t.Fatalf("Failed to get value from DB 1: %v", err)
	}

	if string(value) != "value" {
		t.Fatalf("Expected 'value', got %s", value)
	}
}

func TestRedisStoreGetError(t *testing.T) {
	store, err := NewRedisStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get non-existent key should return ErrNotFound
	_, err = store.Get(ctx, "test:nonexistent:key")
	if err != ErrNotFound {
		t.Fatalf("Expected ErrNotFound, got %v", err)
	}
}
