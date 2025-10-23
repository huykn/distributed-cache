package cache

import (
	"time"
)

// LocalCacheConfig configures the local cache.
type LocalCacheConfig struct {
	// NumCounters is the number of counters for the cache (Ristretto only).
	// Recommended: 10 * MaxItems
	NumCounters int64

	// MaxCost is the maximum cost of items in the cache (Ristretto only).
	// Recommended: 1GB = 1 << 30
	MaxCost int64

	// BufferItems is the number of items to buffer before eviction (Ristretto only).
	// Recommended: 64
	BufferItems int64

	// IgnoreInternalCost ignores the internal cost of items (Ristretto only).
	IgnoreInternalCost bool

	// MaxSize is the maximum number of items in the cache (LRU only).
	MaxSize int
}

// Options configures a SyncedCache instance.
type Options struct {
	// PodID is the unique identifier for this pod/instance.
	// Used to avoid self-invalidation in pub/sub.
	PodID string

	// LocalCacheConfig configures the local Ristretto cache.
	LocalCacheConfig LocalCacheConfig

	// LocalCacheFactory is the factory for creating local cache instances.
	// If nil, defaults to Ristretto factory.
	LocalCacheFactory LocalCacheFactory

	// RedisAddr is the Redis server address (e.g., "localhost:6379").
	RedisAddr string

	// RedisPassword is the optional Redis password.
	RedisPassword string

	// RedisDB is the Redis database number.
	RedisDB int

	// InvalidationChannel is the Redis pub/sub channel for cache invalidation.
	InvalidationChannel string

	// SerializationFormat specifies how values are serialized ("json" or "msgpack").
	SerializationFormat string

	// Marshaller is the marshaller for serialization.
	// If nil, defaults to JSON marshaller.
	Marshaller Marshaller

	// Logger is the logger for debug logging.
	// If nil, defaults to no-op logger.
	Logger Logger

	// DebugMode enables debug logging.
	DebugMode bool

	// ContextTimeout is the default timeout for cache operations.
	ContextTimeout time.Duration

	// EnableMetrics enables metrics collection.
	EnableMetrics bool

	// OnError is called when an error occurs in background operations.
	OnError func(error)
}

// DefaultOptions returns default cache options.
func DefaultOptions() Options {
	return Options{
		PodID:               "default-pod",
		RedisAddr:           "localhost:6379",
		RedisDB:             0,
		InvalidationChannel: "cache:invalidate",
		SerializationFormat: "json",
		ContextTimeout:      5 * time.Second,
		EnableMetrics:       true,
		LocalCacheConfig:    DefaultLocalCacheConfig(),
		LocalCacheFactory:   nil, // Will default to Ristretto in New()
		Marshaller:          nil, // Will default to JSON in New()
		Logger:              nil, // Will default to no-op in New()
		DebugMode:           false,
	}
}

// DefaultLocalCacheConfig returns default local cache configuration.
func DefaultLocalCacheConfig() LocalCacheConfig {
	return LocalCacheConfig{
		NumCounters:        1e7,     // 10 million
		MaxCost:            1 << 30, // 1GB
		BufferItems:        64,
		IgnoreInternalCost: false,
		MaxSize:            10000,
	}
}

// Validate validates the options.
func (o *Options) Validate() error {
	if o.PodID == "" {
		return ErrInvalidConfig
	}
	if o.RedisAddr == "" {
		return ErrInvalidConfig
	}
	if o.InvalidationChannel == "" {
		return ErrInvalidConfig
	}
	if o.SerializationFormat != "json" && o.SerializationFormat != "msgpack" {
		return ErrInvalidConfig
	}
	if o.LocalCacheConfig.NumCounters <= 0 {
		return ErrInvalidConfig
	}
	if o.LocalCacheConfig.MaxCost <= 0 {
		return ErrInvalidConfig
	}
	return nil
}

// ErrInvalidConfig is returned when options are invalid.
var ErrInvalidConfig = NewError("invalid cache configuration")

// NewError creates a new error with the given message.
func NewError(msg string) error {
	return &cacheError{msg: msg}
}

type cacheError struct {
	msg string
}

func (e *cacheError) Error() string {
	return e.msg
}
