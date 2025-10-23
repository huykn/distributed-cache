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
