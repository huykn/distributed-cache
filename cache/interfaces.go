package cache

import (
	"context"

	"github.com/huykn/distributed-cache/types"
)

// Logger defines the interface for logging in the distributed cache.
type Logger interface {
	// Debug logs a debug message.
	Debug(msg string, args ...any)

	// Info logs an info message.
	Info(msg string, args ...any)

	// Warn logs a warning message.
	Warn(msg string, args ...any)

	// Error logs an error message.
	Error(msg string, args ...any)
}

// Marshaller defines the interface for JSON marshalling/unmarshalling.
type Marshaller interface {
	// Marshal serializes a value to bytes.
	Marshal(v any) ([]byte, error)

	// Unmarshal deserializes a value from bytes.
	Unmarshal(data []byte, v any) error
}

// LocalCache defines the interface for local in-process caching.
type LocalCache interface {
	// Get retrieves a value from the local cache.
	Get(key string) (any, bool)

	// Set stores a value in the local cache.
	Set(key string, value any, cost int64) bool

	// Delete removes a value from the local cache.
	Delete(key string)

	// Clear removes all values from the local cache.
	Clear()

	// Close closes the local cache.
	Close()

	// Metrics returns cache metrics.
	Metrics() LocalCacheMetrics
}

// LocalCacheMetrics represents local cache metrics.
type LocalCacheMetrics struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int64
}

// LocalCacheFactory defines the interface for creating local cache implementations.
type LocalCacheFactory interface {
	// Create creates a new local cache instance.
	Create() (LocalCache, error)
}

// Cache defines the interface for a distributed cache with local and remote storage.
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns the value and true if found, nil and false otherwise.
	Get(ctx context.Context, key string) (any, bool)

	// Set stores a value in the cache and propagates it to other pods.
	// The value is stored in both local and remote storage, and other pods
	// receive the value directly to update their local caches.
	Set(ctx context.Context, key string, value any) error

	// SetWithInvalidate stores a value in the cache and invalidates it on other pods.
	// The value is stored in both local and remote storage, but other pods
	// only receive an invalidation event and must fetch from Redis if needed.
	SetWithInvalidate(ctx context.Context, key string, value any) error

	// Delete removes a value from the cache.
	// The value is removed from both local and remote storage.
	Delete(ctx context.Context, key string) error

	// Clear removes all values from the cache.
	Clear(ctx context.Context) error

	// Close closes the cache and releases all resources.
	Close() error

	// Stats returns cache statistics.
	Stats() Stats
}

// Store defines the interface for remote storage backends (e.g., Redis).
type Store interface {
	// Get retrieves a value from the store.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the store.
	Set(ctx context.Context, key string, value []byte) error

	// Delete removes a value from the store.
	Delete(ctx context.Context, key string) error

	// Clear removes all values from the store.
	Clear(ctx context.Context) error

	// Close closes the store connection.
	Close() error
}

// Synchronizer defines the interface for cache synchronization across nodes.
type Synchronizer interface {
	// Subscribe starts listening for invalidation events.
	Subscribe(ctx context.Context) error

	// Publish publishes an invalidation event.
	Publish(ctx context.Context, event types.InvalidationEvent) error

	// OnInvalidate registers a callback for invalidation events.
	OnInvalidate(callback func(event types.InvalidationEvent))

	// Close closes the synchronizer.
	Close() error
}

// InvalidationEvent is an alias for types.InvalidationEvent for backward compatibility
type InvalidationEvent = types.InvalidationEvent

// Action is an alias for types.Action for backward compatibility
type Action = types.Action

// Action constants for cache operations
const (
	ActionSet        = types.Set
	ActionInvalidate = types.Invalidate
	ActionDelete     = types.Delete
	ActionClear      = types.Clear
)

// Stats represents cache statistics.
type Stats struct {
	LocalHits     int64
	LocalMisses   int64
	RemoteHits    int64
	RemoteMisses  int64
	LocalSize     int64
	RemoteSize    int64
	Invalidations int64
}
