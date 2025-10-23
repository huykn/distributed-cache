package cache

import (
	"sync/atomic"

	lfu "github.com/dgraph-io/ristretto"
)

// LFUCacheFactory creates Ristretto cache instances.
type LFUCacheFactory struct {
	config LocalCacheConfig
}

// NewLFUCacheFactory creates a new Ristretto cache factory.
func NewLFUCacheFactory(config LocalCacheConfig) LocalCacheFactory {
	return &LFUCacheFactory{config: config}
}

// Create creates a new Ristretto cache instance.
func (rcf *LFUCacheFactory) Create() (LocalCache, error) {
	return NewLFUCache(rcf.config)
}

// NewLFUCache creates a new Ristretto-based local cache.
func NewLFUCache(config LocalCacheConfig) (*LFUCache, error) {
	cache, err := lfu.NewCache(&lfu.Config{
		NumCounters:        config.NumCounters,
		MaxCost:            config.MaxCost,
		BufferItems:        config.BufferItems,
		IgnoreInternalCost: config.IgnoreInternalCost,
		OnEvict: func(item *lfu.Item) {
			// Track evictions
		},
	})
	if err != nil {
		return nil, err
	}

	return &LFUCache{
		cache: cache,
	}, nil
}

// LFUCache is a local LFU cache implementation using lfu.
type LFUCache struct {
	cache     *lfu.Cache
	hits      int64
	misses    int64
	evictions int64
}

// Get retrieves a value from the local cache.
func (rc *LFUCache) Get(key string) (any, bool) {
	value, found := rc.cache.Get(key)
	if found {
		atomic.AddInt64(&rc.hits, 1)
	} else {
		atomic.AddInt64(&rc.misses, 1)
	}
	return value, found
}

// Set stores a value in the local cache.
func (rc *LFUCache) Set(key string, value any, cost int64) bool {
	return rc.cache.Set(key, value, cost)
}

// Delete removes a value from the local cache.
func (rc *LFUCache) Delete(key string) {
	rc.cache.Del(key)
}

// Clear removes all values from the local cache.
func (rc *LFUCache) Clear() {
	rc.cache.Clear()
}

// Close closes the local cache.
func (rc *LFUCache) Close() {
	rc.cache.Close()
}

// Metrics returns cache metrics.
func (rc *LFUCache) Metrics() LocalCacheMetrics {
	return LocalCacheMetrics{
		Hits:      atomic.LoadInt64(&rc.hits),
		Misses:    atomic.LoadInt64(&rc.misses),
		Evictions: atomic.LoadInt64(&rc.evictions),
		Size:      int64(rc.cache.MaxCost()),
	}
}
