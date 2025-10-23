package cache

import (
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.PodID == "" {
		t.Fatal("PodID should not be empty")
	}

	if opts.RedisAddr == "" {
		t.Fatal("RedisAddr should not be empty")
	}

	if opts.InvalidationChannel == "" {
		t.Fatal("InvalidationChannel should not be empty")
	}

	if opts.SerializationFormat == "" {
		t.Fatal("SerializationFormat should not be empty")
	}

	if opts.ContextTimeout == 0 {
		t.Fatal("ContextTimeout should not be zero")
	}
}

func TestDefaultLocalCacheConfig(t *testing.T) {
	config := DefaultLocalCacheConfig()

	if config.NumCounters <= 0 {
		t.Fatal("NumCounters should be positive")
	}

	if config.MaxCost <= 0 {
		t.Fatal("MaxCost should be positive")
	}

	if config.BufferItems <= 0 {
		t.Fatal("BufferItems should be positive")
	}
}

func TestOptionsValidate(t *testing.T) {
	tests := []struct {
		name  string
		opts  Options
		valid bool
	}{
		{
			name:  "Valid options",
			opts:  DefaultOptions(),
			valid: true,
		},
		{
			name: "Empty PodID",
			opts: Options{
				PodID:               "",
				RedisAddr:           "localhost:6379",
				InvalidationChannel: "cache:invalidate",
				SerializationFormat: "json",
				LocalCacheConfig:    DefaultLocalCacheConfig(),
			},
			valid: false,
		},
		{
			name: "Empty RedisAddr",
			opts: Options{
				PodID:               "pod-1",
				RedisAddr:           "",
				InvalidationChannel: "cache:invalidate",
				SerializationFormat: "json",
				LocalCacheConfig:    DefaultLocalCacheConfig(),
			},
			valid: false,
		},
		{
			name: "Invalid SerializationFormat",
			opts: Options{
				PodID:               "pod-1",
				RedisAddr:           "localhost:6379",
				InvalidationChannel: "cache:invalidate",
				SerializationFormat: "invalid",
				LocalCacheConfig:    DefaultLocalCacheConfig(),
			},
			valid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.opts.Validate()
			if test.valid && err != nil {
				t.Fatalf("Expected valid options, got error: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("Expected invalid options, got no error")
			}
		})
	}
}

func TestOptionsContextTimeout(t *testing.T) {
	opts := DefaultOptions()
	opts.ContextTimeout = 10 * time.Second

	if opts.ContextTimeout != 10*time.Second {
		t.Fatalf("Expected 10s timeout, got %v", opts.ContextTimeout)
	}
}

func TestLocalCacheConfigValidation(t *testing.T) {
	config := DefaultLocalCacheConfig()

	if config.NumCounters != 1e7 {
		t.Fatalf("Expected NumCounters to be 1e7, got %d", config.NumCounters)
	}

	if config.MaxCost != 1<<30 {
		t.Fatalf("Expected MaxCost to be 1GB, got %d", config.MaxCost)
	}

	if config.BufferItems != 64 {
		t.Fatalf("Expected BufferItems to be 64, got %d", config.BufferItems)
	}
}
