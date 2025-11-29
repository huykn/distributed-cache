# LFU vs LRU Comparison Example

This example demonstrates the differences between LFU (Least Frequently Used) and LRU (Least Recently Used) cache eviction strategies through side-by-side comparison.

## What This Example Demonstrates

- **LFU cache behavior** with frequency-based eviction
- **LRU cache behavior** with recency-based eviction
- **Performance comparison** between the two strategies
- **Use case recommendations** for each strategy
- **Cache statistics** and metrics comparison

## Key Concepts

### LFU (Least Frequently Used)

- **Eviction Strategy**: Removes items accessed least frequently
- **Best For**: Workloads with varying access patterns
- **Implementation**: TinyLFU algorithm via Ristretto
- **Advantage**: Keeps "hot" items in cache longer

### LRU (Least Recently Used)

- **Eviction Strategy**: Removes items not accessed recently
- **Best For**: Sequential or temporal access patterns
- **Implementation**: Doubly-linked list via hashicorp/golang-lru
- **Advantage**: Simple and predictable behavior

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
cd examples/comparison
go run main.go
```

## Expected Output

The example demonstrates both cache types with the same access patterns:

### LFU Demonstration

```
--- LFU Cache Demonstration ---
Adding users with different access frequencies...
  User 1: 10 accesses (high frequency)
  User 2: 5 accesses (medium frequency)
  User 3: 2 accesses (low frequency)

LFU Behavior:
  - User 1 (10 accesses) will be kept longest
  - User 2 (5 accesses) will be kept moderately
  - User 3 (2 accesses) will be evicted first
  - Frequency tracking ensures hot items stay cached

Cache Statistics:
  Local Hits: 17
  Local Misses: 3
  Hit Ratio: 85.00%
```

### LRU Demonstration

```
--- LRU Cache Demonstration ---
Adding users in sequence...
  Order: User1 → User2 → User3

Accessing in specific order:
  User3 → User1 → User2

LRU Behavior:
  - User2 (most recent) will be kept longest
  - User1 (moderately recent) will be kept moderately
  - User3 (least recent) will be evicted first
  - Recency determines eviction priority

Cache Statistics:
  Local Hits: 15
  Local Misses: 3
  Hit Ratio: 83.33%
```

## Comparison Summary

The example provides a detailed comparison:

```
========================================
LFU vs LRU Comparison Summary
========================================

LFU (Least Frequently Used):
  ✓ Tracks access frequency
  ✓ Better for varying access patterns
  ✓ Keeps frequently accessed items
  ✓ More complex implementation
  ✓ Higher memory overhead

LRU (Least Recently Used):
  ✓ Tracks access recency
  ✓ Better for sequential patterns
  ✓ Keeps recently accessed items
  ✓ Simpler implementation
  ✓ Lower memory overhead

When to Use LFU:
  - Access patterns vary significantly
  - Some items are much more popular
  - You want to maximize hit ratio
  - Memory is limited

When to Use LRU:
  - Sequential access patterns
  - Temporal locality is important
  - Predictable behavior needed
  - Simplicity is preferred
```

## Configuration Examples

### LFU Configuration

```go
cfg := dc.DefaultConfig()
cfg.LocalCacheConfig = dc.LocalCacheConfig{
    NumCounters:        1e7,     // 10M counters for frequency tracking
    MaxCost:            1 << 30, // 1GB max cache size
    BufferItems:        64,      // Buffer for async operations
    IgnoreInternalCost: false,   // Track actual memory cost
}
cfg.LocalCacheFactory = cache.NewLFUCacheFactory(cfg.LocalCacheConfig)
```

### LRU Configuration

```go
cfg := dc.DefaultConfig()
maxSize := 10000 // Maximum number of items
cfg.LocalCacheFactory = cache.NewLRUCacheFactory(maxSize)
```

## Performance Characteristics

### LFU Performance

- **Memory**: Higher (frequency counters + cache data)
- **CPU**: Moderate (frequency tracking overhead)
- **Hit Ratio**: Excellent for varying patterns
- **Eviction**: Smart (frequency-based)

### LRU Performance

- **Memory**: Lower (just cache data + linked list)
- **CPU**: Low (simple recency tracking)
- **Hit Ratio**: Good for sequential patterns
- **Eviction**: Predictable (recency-based)

## Use Case Recommendations

### Choose LFU When:

1. **E-commerce Product Catalog**
   - Popular products accessed frequently
   - Long-tail products accessed rarely
   - Want to keep bestsellers in cache

2. **Content Delivery**
   - Viral content gets many hits
   - Old content rarely accessed
   - Frequency matters more than recency

3. **API Rate Limiting**
   - Track frequent API callers
   - Evict infrequent users first
   - Optimize for heavy users

### Choose LRU When:

1. **Session Management**
   - Recent sessions more likely to be active
   - Old sessions can be evicted
   - Temporal locality is key

2. **Log Processing**
   - Recent logs accessed more often
   - Sequential processing pattern
   - Recency matters most

3. **Database Query Cache**
   - Recent queries likely to repeat
   - Simple eviction policy needed
   - Predictable behavior preferred

## What You'll Learn

1. **How LFU and LRU differ** in eviction behavior
2. **When to use each strategy** based on access patterns
3. **How to configure each cache type** with appropriate parameters
4. **Performance trade-offs** between the two approaches
5. **How to measure cache effectiveness** using statistics

## Next Steps

After understanding the comparison, explore:

- **[LFU Example](../lfu/)** - Deep dive into LFU cache
- **[LRU Example](../lru/)** - Deep dive into LRU cache
- **[Custom Local Cache](../custom-local-cache/)** - Implement your own eviction strategy
- **[Basic Example](../basic/)** - Learn basic cache operations

## Related Documentation

- [Main README](../../README.md) - Complete library documentation
- [Getting Started Guide](../../GETTING_STARTED.md) - Step-by-step setup
- [Performance Characteristics](../../README.md#performance-characteristics) - Detailed performance info
