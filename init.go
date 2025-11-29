package distributedcache

import (
	"time"

	"github.com/huykn/distributed-cache/cache"
)

// Config configures a distributed cache instance.
type Config struct {
	// PodID is the unique identifier for this pod/instance.
	// Used to avoid self-invalidation in pub/sub.
	PodID string

	// LocalCacheConfig configures the local cache.
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

	// ReaderCanSetToRedis controls whether reader nodes are allowed to write data to Redis.
	// When false (default), reader nodes will only update local cache but NOT write to Redis.
	ReaderCanSetToRedis bool

	// OnSetLocalCache is a callback for custom processing of data before storing in local cache.
	// This callback is invoked when an invalidation event with action "set" is received.
	// When nil (default), the default behavior is used: unmarshal the value and store in local cache.
	OnSetLocalCache func(event InvalidationEvent) any
}

// New creates a new distributed cache instance.
// This is the root-level initialization function that allows users to import from the root package.
func New(cfg Config) (Cache, error) {
	// Convert root Config to cache.Options
	opts := cache.Options{
		PodID:               cfg.PodID,
		LocalCacheConfig:    cfg.LocalCacheConfig,
		LocalCacheFactory:   cfg.LocalCacheFactory,
		RedisAddr:           cfg.RedisAddr,
		RedisPassword:       cfg.RedisPassword,
		RedisDB:             cfg.RedisDB,
		InvalidationChannel: cfg.InvalidationChannel,
		SerializationFormat: cfg.SerializationFormat,
		Marshaller:          cfg.Marshaller,
		Logger:              cfg.Logger,
		DebugMode:           cfg.DebugMode,
		ContextTimeout:      cfg.ContextTimeout,
		EnableMetrics:       cfg.EnableMetrics,
		OnError:             cfg.OnError,
		ReaderCanSetToRedis: cfg.ReaderCanSetToRedis,
		OnSetLocalCache:     cfg.OnSetLocalCache,
	}

	return cache.New(opts)
}

// DefaultConfig returns default cache configuration.
func DefaultConfig() Config {
	return Config{
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

// Cache is an alias for cache.Cache interface.
type Cache = cache.Cache

// Stats is an alias for cache.Stats.
type Stats = cache.Stats
