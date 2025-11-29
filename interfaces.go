package distributedcache

import "github.com/huykn/distributed-cache/cache"

// Logger is an alias for cache.Logger.
type Logger = cache.Logger

// Marshaller is an alias for cache.Marshaller.
type Marshaller = cache.Marshaller

// LocalCache is an alias for cache.LocalCache.
type LocalCache = cache.LocalCache

// LocalCacheMetrics is an alias for cache.LocalCacheMetrics.
type LocalCacheMetrics = cache.LocalCacheMetrics

// LocalCacheFactory is an alias for cache.LocalCacheFactory.
type LocalCacheFactory = cache.LocalCacheFactory

// LocalCacheConfig is an alias for cache.LocalCacheConfig.
type LocalCacheConfig = cache.LocalCacheConfig

// InvalidationEvent is an alias for cache.InvalidationEvent.
type InvalidationEvent = cache.InvalidationEvent

// DefaultLocalCacheConfig returns default local cache configuration for Ristretto.
func DefaultLocalCacheConfig() LocalCacheConfig {
	return cache.DefaultLocalCacheConfig()
}
