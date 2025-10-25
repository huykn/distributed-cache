package distributedcache

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	cfg := Config{
		PodID:               "test-pod",
		RedisAddr:           "localhost:6379",
		RedisDB:             0,
		InvalidationChannel: "cache:invalidate",
		SerializationFormat: "json",
		ContextTimeout:      5 * time.Second,
		EnableMetrics:       true,
		LocalCacheConfig:    DefaultLocalCacheConfig(),
	}

	cache, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}
}

func TestNewWithDefaults(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PodID = "test-pod-defaults"

	cache, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache with defaults: %v", err)
	}
	defer cache.Close()

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PodID != "default-pod" {
		t.Errorf("Expected PodID 'default-pod', got %s", cfg.PodID)
	}

	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("Expected RedisAddr 'localhost:6379', got %s", cfg.RedisAddr)
	}

	if cfg.RedisDB != 0 {
		t.Errorf("Expected RedisDB 0, got %d", cfg.RedisDB)
	}

	if cfg.InvalidationChannel != "cache:invalidate" {
		t.Errorf("Expected InvalidationChannel 'cache:invalidate', got %s", cfg.InvalidationChannel)
	}

	if cfg.SerializationFormat != "json" {
		t.Errorf("Expected SerializationFormat 'json', got %s", cfg.SerializationFormat)
	}

	if cfg.ContextTimeout != 5*time.Second {
		t.Errorf("Expected ContextTimeout 5s, got %v", cfg.ContextTimeout)
	}

	if !cfg.EnableMetrics {
		t.Error("Expected EnableMetrics to be true")
	}

	if cfg.DebugMode {
		t.Error("Expected DebugMode to be false")
	}

	if cfg.LocalCacheFactory != nil {
		t.Error("Expected LocalCacheFactory to be nil (will default to Ristretto)")
	}

	if cfg.Marshaller != nil {
		t.Error("Expected Marshaller to be nil (will default to JSON)")
	}

	if cfg.Logger != nil {
		t.Error("Expected Logger to be nil (will default to no-op)")
	}
}

func TestNewWithCustomLogger(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PodID = "test-pod-logger"
	cfg.Logger = &testLogger{}

	cache, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache with custom logger: %v", err)
	}
	defer cache.Close()

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}
}

func TestNewWithCustomMarshaller(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PodID = "test-pod-marshaller"
	cfg.Marshaller = &testMarshaller{}

	cache, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache with custom marshaller: %v", err)
	}
	defer cache.Close()

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}
}

func TestNewWithDebugMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PodID = "test-pod-debug"
	cfg.DebugMode = true

	cache, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache with debug mode: %v", err)
	}
	defer cache.Close()

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}
}

func TestNewCacheOperations(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PodID = "test-pod-ops"

	cache, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test Set
	err = cache.Set(ctx, "test:key", "test:value")
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Test Get
	value, found := cache.Get(ctx, "test:key")
	if !found {
		t.Fatal("Value should be found")
	}

	if value != "test:value" {
		t.Fatalf("Expected 'test:value', got %v", value)
	}

	// Test Delete
	err = cache.Delete(ctx, "test:key")
	if err != nil {
		t.Fatalf("Failed to delete value: %v", err)
	}

	// Verify deletion
	_, found = cache.Get(ctx, "test:key")
	if found {
		t.Fatal("Value should not be found after deletion")
	}
}

// testLogger is a simple logger implementation for testing
type testLogger struct{}

func (l *testLogger) Debug(msg string, args ...any) {}
func (l *testLogger) Info(msg string, args ...any)  {}
func (l *testLogger) Warn(msg string, args ...any)  {}
func (l *testLogger) Error(msg string, args ...any) {}

// testMarshaller is a simple marshaller implementation for testing
type testMarshaller struct{}

func (m *testMarshaller) Marshal(v any) ([]byte, error) {
	return []byte("test"), nil
}

func (m *testMarshaller) Unmarshal(data []byte, v any) error {
	return nil
}
