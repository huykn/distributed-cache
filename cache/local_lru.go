package cache

import (
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
)

// LRUCacheFactory creates LRU cache instances.
type LRUCacheFactory struct {
	maxSize int
}

// NewLRUCacheFactory creates a new LRU cache factory.
func NewLRUCacheFactory(maxSize int) LocalCacheFactory {
	return &LRUCacheFactory{maxSize: maxSize}
}

// Create creates a new LRU cache instance.
func (lcf *LRUCacheFactory) Create() (LocalCache, error) {
	return NewLRUCache(lcf.maxSize)
}

// LRUCache is a local LRU cache implementation using golang-lru.
type LRUCache struct {
	cache     *lru.Cache[string, any]
	hits      int64
	misses    int64
	evictions int64
	maxSize   int64
}

// NewLRUCache creates a new LRU-based local cache.
func NewLRUCache(maxSize int) (*LRUCache, error) {
	cache, err := lru.New[string, any](maxSize)
	if err != nil {
		return nil, err
	}

	return &LRUCache{
		cache:   cache,
		maxSize: int64(maxSize),
	}, nil
}

// Get retrieves a value from the local cache.
func (lc *LRUCache) Get(key string) (any, bool) {
	value, found := lc.cache.Get(key)
	if found {
		atomic.AddInt64(&lc.hits, 1)
	} else {
		atomic.AddInt64(&lc.misses, 1)
	}
	return value, found
}

// Set stores a value in the local cache.
func (lc *LRUCache) Set(key string, value any, cost int64) bool {
	lc.cache.Add(key, value)
	return true
}

// Delete removes a value from the local cache.
func (lc *LRUCache) Delete(key string) {
	lc.cache.Remove(key)
}

// Clear removes all values from the local cache.
func (lc *LRUCache) Clear() {
	lc.cache.Purge()
}

// Close closes the local cache.
func (lc *LRUCache) Close() {
	lc.cache.Purge()
}

// Metrics returns cache metrics.
func (lc *LRUCache) Metrics() LocalCacheMetrics {
	return LocalCacheMetrics{
		Hits:      atomic.LoadInt64(&lc.hits),
		Misses:    atomic.LoadInt64(&lc.misses),
		Evictions: atomic.LoadInt64(&lc.evictions),
		Size:      lc.maxSize,
	}
}
