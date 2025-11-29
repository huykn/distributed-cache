# Custom Local Cache Example

This example demonstrates how to implement a custom local cache using a simple `map[string]any` as the underlying storage mechanism, showing how to integrate your own cache implementation with the distributed-cache library.

## What This Example Demonstrates

- **Custom LocalCache interface implementation** using a simple map
- **Thread-safe cache operations** with sync.RWMutex
- **Metrics tracking** (hits, misses, evictions, size)
- **Simple eviction strategy** (random eviction when full)
- **LocalCacheFactory implementation** for creating cache instances
- **Integration with distributed-cache** library

## Key Concepts

### LocalCache Interface

The `LocalCache` interface requires implementing these methods:

```go
type LocalCache interface {
    Get(key string) (any, bool)
    Set(key string, value any, cost int64) bool
    Delete(key string)
    Clear()
    Close()
    Metrics() LocalCacheMetrics
}
```

### SimpleMapCache Implementation

The example provides a basic implementation:

```go
type SimpleMapCache struct {
    mu        sync.RWMutex
    data      map[string]any
    hits      int64
    misses    int64
    evictions int64
    maxSize   int
}
```

## Prerequisites

- Go 1.25 or later
- Redis server running on `localhost:6379`

## How to Run

### 1. Start Redis

```bash
# Using Redis directly
redis-server

# Or using Docker
docker run -d -p 6379:6379 redis:latest
```

### 2. Run the Example

```bash
cd examples/custom-local-cache
go run main.go
```

## Expected Output

```
========================================
Custom Local Cache Example
========================================

Custom Cache Overview:
  Implementation: Simple map[string]any
  Thread-safety: sync.RWMutex
  Eviction: Random (when full)
  Metrics: Hits, Misses, Evictions, Size

Custom Cache Configuration:
  MaxSize: 100 items
  Storage: map[string]any
  Eviction: Random entry when full

Creating cache with custom implementation...
✓ Cache initialized with custom local cache

Testing custom cache operations...

Example 1: Basic Set and Get
✓ Set user:1 successfully
✓ Retrieved user:1 from cache

Example 2: Cache Metrics
Custom Cache Metrics:
  Hits: 1
  Misses: 0
  Evictions: 0
  Size: 1 items

Example 3: Filling Cache
✓ Added 10 users to cache

Example 4: Cache Full - Eviction
Current cache size: 10 items
Adding more items to trigger eviction...
✓ Eviction triggered when cache is full

Final Metrics:
  Hits: 11
  Misses: 0
  Evictions: 5
  Size: 10 items (at max capacity)

========================================
Custom Cache Implementation Summary:
  ✓ Implements LocalCache interface
  ✓ Thread-safe with sync.RWMutex
  ✓ Tracks metrics (hits, misses, evictions)
  ✓ Simple eviction strategy
  ✓ Integrates with distributed-cache library
========================================
```

## Implementation Details

### Thread Safety

The implementation uses `sync.RWMutex` for thread-safe operations:

```go
// Read operations use RLock
func (c *SimpleMapCache) Get(key string) (any, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    value, exists := c.data[key]
    return value, exists
}

// Write operations use Lock
func (c *SimpleMapCache) Set(key string, value any, cost int64) bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.data[key] = value
    return true
}
```

### Metrics Tracking

Metrics are tracked using atomic operations:

```go
// Increment hits atomically
atomic.AddInt64(&c.hits, 1)

// Increment misses atomically
atomic.AddInt64(&c.misses, 1)

// Increment evictions atomically
atomic.AddInt64(&c.evictions, 1)
```

### Eviction Strategy

Simple random eviction when cache is full:

```go
if len(c.data) >= c.maxSize {
    // Remove a random entry
    for k := range c.data {
        delete(c.data, k)
        atomic.AddInt64(&c.evictions, 1)
        break
    }
}
```

### Factory Pattern

The factory creates new cache instances:

```go
type SimpleMapCacheFactory struct {
    maxSize int
}

func (f *SimpleMapCacheFactory) Create() (cache.LocalCache, error) {
    return NewSimpleMapCache(f.maxSize), nil
}
```

## Custom Cache Use Cases

### 1. Simple In-Memory Cache

For basic caching needs without complex eviction:

```go
cfg.LocalCacheFactory = NewSimpleMapCacheFactory(1000)
```

### 2. Custom Eviction Strategy

Implement your own eviction logic (LRU, LFU, TTL, etc.):

```go
// Example: Implement LRU eviction
type LRUMapCache struct {
    data      map[string]*list.Element
    evictList *list.List
    maxSize   int
}
```

### 3. Specialized Storage

Use specialized data structures for specific needs:

```go
// Example: Time-based eviction
type TTLCache struct {
    data      map[string]cacheEntry
    expiryMap map[string]time.Time
}
```

## Advanced Customization

### Adding TTL Support

```go
type TTLMapCache struct {
    mu      sync.RWMutex
    data    map[string]cacheEntry
    maxSize int
}

type cacheEntry struct {
    value  any
    expiry time.Time
}

func (c *TTLMapCache) Get(key string) (any, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    entry, exists := c.data[key]
    if !exists {
        return nil, false
    }
    
    // Check if expired
    if time.Now().After(entry.expiry) {
        return nil, false
    }
    
    return entry.value, true
}
```

### Adding Size-Based Eviction

```go
type SizeBasedCache struct {
    mu          sync.RWMutex
    data        map[string]any
    sizes       map[string]int64
    currentSize int64
    maxSize     int64
}

func (c *SizeBasedCache) Set(key string, value any, cost int64) bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // Evict until we have space
    for c.currentSize+cost > c.maxSize && len(c.data) > 0 {
        c.evictOne()
    }
    
    c.data[key] = value
    c.sizes[key] = cost
    c.currentSize += cost
    return true
}
```

## What You'll Learn

1. **How to implement the LocalCache interface** with custom logic
2. **How to ensure thread safety** using sync.RWMutex
3. **How to track metrics** using atomic operations
4. **How to implement eviction strategies** for cache management
5. **How to create a factory** for cache instantiation
6. **How to integrate custom cache** with distributed-cache library

## Next Steps

After understanding custom local cache implementation, explore:

- **[LFU Example](../lfu/)** - Learn about the default LFU implementation
- **[LRU Example](../lru/)** - Learn about the LRU implementation
- **[Comparison Example](../comparison/)** - Compare different cache strategies
- **[Basic Example](../basic/)** - Learn basic cache operations

## Best Practices

1. **Always use locks** for thread-safe operations
2. **Use atomic operations** for metrics tracking
3. **Implement proper eviction** to prevent memory leaks
4. **Track metrics** for monitoring and debugging
5. **Test thoroughly** with concurrent access
6. **Document behavior** clearly for users

## Related Documentation

- [Main README](../../README.md) - Complete library documentation
- [LocalCache Interface](../../cache/interfaces.go) - Interface definition
- [Getting Started Guide](../../GETTING_STARTED.md) - Basic setup
