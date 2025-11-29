# LRU Cache Example

This example demonstrates the LRU (Least Recently Used) cache implementation using the hashicorp/golang-lru library, providing a simpler alternative to the default LFU cache.

## What This Example Demonstrates

- **LRU cache behavior** with recency-based eviction
- **Doubly-linked list implementation** for O(1) operations
- **Recency tracking** for cache items
- **Predictable eviction** based on access order
- **Configuration parameters** for LRU cache
- **Use cases** for temporal locality

## Key Concepts

### LRU (Least Recently Used)

- **Eviction Strategy**: Removes items not accessed recently
- **Algorithm**: Doubly-linked list + hash map
- **Best For**: Sequential or temporal access patterns
- **Advantage**: Simple and predictable behavior
- **Alternative**: To the default LFU implementation

### How LRU Works

- **Access Order**: Maintains order of item access
- **Most Recent**: Moved to front on access
- **Least Recent**: Evicted when cache is full
- **O(1) Operations**: Get, Set, Delete all constant time

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
cd examples/lru
go run main.go
```

## Expected Output

```
========================================
LRU Cache Example
========================================

LRU Cache Overview:
  Algorithm: Least Recently Used
  Eviction: Items not accessed recently
  Best for: Sequential/temporal access patterns

LRU Configuration:
  MaxSize: 10000 items
  Eviction: Least recently used

Initializing cache with LRU implementation...
✓ Cache initialized successfully

Demonstrating LRU behavior...
========================================

Step 1: Adding 5 users to cache...
✓ 5 users added
  Order: User1 → User2 → User3 → User4 → User5

Step 2: Accessing users in specific order...
  Accessing: User5 → User3 → User1
  ✓ Accessed User5 (now most recent)
  ✓ Accessed User3
  ✓ Accessed User1

Current LRU order (most → least recent):
  User1 → User3 → User5 → User4 → User2
  (User2 and User4 are least recently used)

Step 3: Adding more users to trigger eviction...
✓ Added User6, User7, User8

Step 4: Cache Statistics...
  Local Hits: 3
  Local Misses: 8
  Hit Ratio: 27.27%

========================================

LRU Cache Behavior:
  - Recently accessed items stay in cache
  - Least recently used items are evicted first
  - Access order determines eviction priority
  - Simple and predictable eviction policy

When to Use LRU:
  ✓ Sequential access patterns
  ✓ Temporal locality (recent items accessed again)
  ✓ Predictable eviction behavior needed
  ✓ Simpler implementation preferred

LRU vs LFU:
  LRU: Tracks recency (when last accessed)
  LFU: Tracks frequency (how often accessed)
  LRU: Better for sequential patterns
  LFU: Better for varying access patterns

Configuration Tips:
  - Set MaxSize based on expected working set
  - Monitor hit ratio to validate size
  - Consider memory per item when sizing
  - Use metrics to tune cache size

========================================
Example completed successfully!
========================================
```

## Configuration

### MaxSize Parameter

Unlike LFU which uses `MaxCost` (bytes), LRU uses `MaxSize` (number of items):

```go
maxSize := 10000 // Maximum number of items in cache
cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)
```

### Choosing MaxSize

```go
// Small cache for limited memory
maxSize := 1000
cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)

// Medium cache for typical workloads
maxSize := 10000
cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)

// Large cache for high-throughput scenarios
maxSize := 100000
cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)
```

## LRU Behavior Demonstration

The example shows how LRU maintains access order:

```
Initial State (after adding 5 users):
  Most Recent → Least Recent
  User5 → User4 → User3 → User2 → User1

After Accessing User5, User3, User1:
  Most Recent → Least Recent
  User1 → User3 → User5 → User4 → User2

When Cache is Full:
  User2 evicted first (least recently used)
  User4 evicted next
  User5, User3, User1 remain (most recently used)
```

## Use Cases for LRU

### 1. Session Management

```go
// Recent sessions more likely to be active
cache.Set(ctx, "session:abc123", sessionData)
// Old sessions can be evicted
// Temporal locality is key
```

### 2. Log Processing

```go
// Recent logs accessed more often
cache.Set(ctx, "log:2024-01-15", logData)
// Sequential processing pattern
// Recency matters most
```

### 3. Database Query Cache

```go
// Recent queries likely to repeat
cache.Set(ctx, "query:SELECT-users", results)
// Simple eviction policy needed
// Predictable behavior preferred
```

### 4. Web Page Cache

```go
// Recently viewed pages accessed again
cache.Set(ctx, "page:/home", pageData)
// User navigation is sequential
// LRU matches browsing patterns
```

## Performance Characteristics

- **Local Cache Hit**: ~100ns (in-process)
- **Remote Cache Hit**: ~1-5ms (Redis round-trip)
- **Eviction Overhead**: Minimal (O(1) operations)
- **Memory Overhead**: Low (just linked list pointers)
- **Hit Ratio**: Good for sequential/temporal patterns

## LRU vs LFU Comparison

| Aspect | LRU | LFU |
|--------|-----|-----|
| **Eviction** | Least recently used | Least frequently used |
| **Tracking** | Access recency | Access frequency |
| **Best For** | Sequential patterns | Varying patterns |
| **Complexity** | Lower | Higher |
| **Memory** | Lower | Higher (counters) |
| **Predictability** | High | Moderate |
| **Implementation** | Linked list | TinyLFU |

## When to Choose LRU

### Choose LRU When:

1. **Sequential Access Patterns**
   - Users browse pages sequentially
   - Logs processed in order
   - Time-series data access

2. **Temporal Locality**
   - Recent items likely accessed again
   - Session-based workloads
   - User navigation patterns

3. **Predictable Behavior**
   - Need to understand eviction order
   - Debugging cache behavior
   - Simple mental model preferred

4. **Lower Memory Overhead**
   - Memory-constrained environments
   - Don't need frequency tracking
   - Simpler is better

### Choose LFU When:

1. **Varying Access Patterns**
   - Some items much more popular
   - Long-tail distribution
   - Want to keep "hot" items

2. **Maximize Hit Ratio**
   - Frequency matters more than recency
   - Popular items accessed repeatedly
   - Optimize for heavy users

## What You'll Learn

1. **How LRU cache works** with recency-based eviction
2. **How to configure LRU** with MaxSize parameter
3. **When to use LRU** vs LFU cache strategies
4. **How access order** determines eviction priority
5. **Performance trade-offs** between LRU and LFU

## Next Steps

After understanding LRU cache, explore:

- **[LFU Example](../lfu/)** - Learn about Least Frequently Used cache
- **[Comparison Example](../comparison/)** - Compare LFU vs LRU side-by-side
- **[Custom Local Cache](../custom-local-cache/)** - Implement your own cache
- **[Basic Example](../basic/)** - Learn basic cache operations

## Tuning Tips

### For High Hit Ratio

```go
// Increase cache size
maxSize := 50000
cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)
```

### For Memory Efficiency

```go
// Reduce cache size
maxSize := 1000
cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)
```

### Estimating MaxSize

```go
// Calculate based on memory budget
memoryBudget := 1 << 30 // 1GB
avgItemSize := 1024     // 1KB per item
maxSize := memoryBudget / avgItemSize // ~1M items

cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)
```

## Troubleshooting

### Low Hit Ratio

**Cause**: Cache too small for working set

**Solution**: Increase MaxSize
```go
maxSize := 20000 // Double the size
cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)
```

### High Memory Usage

**Cause**: MaxSize set too high

**Solution**: Reduce cache size
```go
maxSize := 5000 // Reduce size
cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)
```

### Frequent Evictions

**Cause**: Working set larger than cache

**Solution**: Increase MaxSize or use LFU
```go
// Option 1: Increase LRU size
maxSize := 50000
cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)

// Option 2: Switch to LFU for better hit ratio
cfg.LocalCacheFactory = cache.NewLFUCacheFactory(cfg.LocalCacheConfig)
```

## Related Documentation

- [Main README](../../README.md) - Complete library documentation
- [LFU Example](../lfu/) - Least Frequently Used cache
- [Comparison Example](../comparison/) - LFU vs LRU comparison
- [golang-lru](https://github.com/hashicorp/golang-lru) - LRU implementation details
