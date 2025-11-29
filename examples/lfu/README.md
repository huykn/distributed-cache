# LFU Cache Example

This example demonstrates the LFU (Least Frequently Used) cache implementation using the Ristretto library, which is the default local cache strategy in distributed-cache.

## What This Example Demonstrates

- **LFU cache behavior** with frequency-based eviction
- **TinyLFU algorithm** via Ristretto library
- **Frequency tracking** for cache items
- **Smart eviction** based on access patterns
- **Configuration parameters** for LFU cache
- **Performance characteristics** and use cases

## Key Concepts

### LFU (Least Frequently Used)

- **Eviction Strategy**: Removes items accessed least frequently
- **Algorithm**: TinyLFU (via Ristretto)
- **Best For**: Workloads with varying access patterns
- **Advantage**: Keeps "hot" items in cache longer
- **Default**: This is the default cache implementation

### TinyLFU Algorithm

- **Frequency Tracking**: Uses probabilistic counters
- **Memory Efficient**: Compact frequency estimation
- **High Performance**: Excellent hit ratios
- **Admission Policy**: Smart decisions on what to cache

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
cd examples/lfu
go run main.go
```

## Expected Output

```
========================================
LFU Cache (Ristretto) Example
========================================

LFU Cache Overview:
  Algorithm: TinyLFU (via Ristretto)
  Eviction: Least Frequently Used items
  Best for: Varying access patterns

LFU Configuration:
  NumCounters: 10000000
  MaxCost: 1073741824 bytes (1.00 GB)
  BufferItems: 64

Initializing cache with LFU implementation...
✓ Cache initialized successfully

Demonstrating LFU behavior...
========================================

Step 1: Adding 5 users to cache...
✓ 5 users added

Step 2: Accessing users with different frequencies...
  - User 1: 10 accesses (high frequency)
  - User 2: 5 accesses (medium frequency)
  - User 3: 2 accesses (low frequency)
  - User 4: 1 access (very low frequency)
  - User 5: 1 access (very low frequency)
✓ Access pattern established

Step 3: Cache Statistics...
  Local Hits: 17
  Local Misses: 5
  Hit Ratio: 77.27%

========================================

LFU Cache Behavior:
  - User 1 (10 accesses) will be kept longest
  - User 2 (5 accesses) will be kept moderately
  - Users 4 & 5 (1 access) will be evicted first
  - Frequency tracking ensures hot items stay cached

When to Use LFU:
  ✓ Access patterns vary significantly
  ✓ Some items are much more popular than others
  ✓ You want to maximize cache hit ratio
  ✓ Memory is limited and eviction must be smart

Configuration Tips:
  - NumCounters: 10x the expected number of items
  - MaxCost: Set based on available memory
  - BufferItems: 64 is a good default
  - Monitor metrics to tune parameters

========================================
Example completed successfully!
========================================
```

## Configuration Parameters

### NumCounters

- **Purpose**: Number of counters for frequency tracking
- **Recommendation**: 10x the expected number of unique items
- **Example**: For 1M items, use 10M counters
- **Impact**: Higher values improve accuracy but use more memory

```go
cfg.LocalCacheConfig.NumCounters = 1e7 // 10 million
```

### MaxCost

- **Purpose**: Maximum memory cost for the cache
- **Recommendation**: Based on available memory
- **Example**: 1GB = `1 << 30`, 2GB = `2 << 30`
- **Impact**: Determines how much data can be cached

```go
cfg.LocalCacheConfig.MaxCost = 1 << 30 // 1GB
```

### BufferItems

- **Purpose**: Size of the buffer for async operations
- **Recommendation**: 64 is good for most workloads
- **Example**: Increase to 128 for high-throughput scenarios
- **Impact**: Higher values improve throughput but use more memory

```go
cfg.LocalCacheConfig.BufferItems = 64
```

### IgnoreInternalCost

- **Purpose**: Whether to ignore internal memory overhead
- **Recommendation**: `false` for accurate memory tracking
- **Example**: Set to `true` if you want to ignore metadata overhead
- **Impact**: Affects how MaxCost is calculated

```go
cfg.LocalCacheConfig.IgnoreInternalCost = false
```

## LFU Behavior Demonstration

The example shows how LFU keeps frequently accessed items:

```
Items Added:
  User 1, User 2, User 3, User 4, User 5

Access Pattern:
  User 1: ████████████████████ (10 accesses)
  User 2: ██████████ (5 accesses)
  User 3: ████ (2 accesses)
  User 4: ██ (1 access)
  User 5: ██ (1 access)

Eviction Priority (when cache is full):
  1. User 4 & User 5 (least frequent)
  2. User 3 (low frequency)
  3. User 2 (medium frequency)
  4. User 1 (high frequency) - kept longest
```

## Use Cases for LFU

### 1. E-commerce Product Catalog

```go
// Popular products accessed frequently
cache.Set(ctx, "product:bestseller", product)
// Long-tail products accessed rarely
cache.Set(ctx, "product:niche", product)
// LFU keeps bestsellers in cache
```

### 2. Content Delivery Network

```go
// Viral content gets many hits
cache.Set(ctx, "content:viral", content)
// Old content rarely accessed
cache.Set(ctx, "content:archive", content)
// LFU prioritizes viral content
```

### 3. API Rate Limiting

```go
// Track frequent API callers
cache.Set(ctx, "ratelimit:heavy-user", limits)
// Infrequent users evicted first
cache.Set(ctx, "ratelimit:light-user", limits)
// LFU optimizes for heavy users
```

### 4. Database Query Cache

```go
// Frequently run queries
cache.Set(ctx, "query:popular", results)
// Rarely run queries
cache.Set(ctx, "query:rare", results)
// LFU keeps popular queries cached
```

## Performance Characteristics

- **Local Cache Hit**: ~100ns (in-process)
- **Remote Cache Hit**: ~1-5ms (Redis round-trip)
- **Eviction Overhead**: Minimal (TinyLFU is efficient)
- **Memory Overhead**: ~10% for frequency counters
- **Hit Ratio**: Excellent for varying access patterns

## Comparison with LRU

| Aspect | LFU | LRU |
|--------|-----|-----|
| Eviction | Least frequently used | Least recently used |
| Tracking | Access frequency | Access recency |
| Best For | Varying patterns | Sequential patterns |
| Complexity | Higher | Lower |
| Memory | Higher (counters) | Lower |
| Hit Ratio | Better for varying | Better for sequential |

## What You'll Learn

1. **How LFU cache works** with frequency-based eviction
2. **How to configure LFU parameters** for optimal performance
3. **When to use LFU** vs other cache strategies
4. **How TinyLFU algorithm** provides efficient frequency tracking
5. **How to monitor cache effectiveness** using statistics

## Next Steps

After understanding LFU cache, explore:

- **[LRU Example](../lru/)** - Learn about Least Recently Used cache
- **[Comparison Example](../comparison/)** - Compare LFU vs LRU
- **[Custom Local Cache](../custom-local-cache/)** - Implement your own cache
- **[Custom Config](../custom-config/)** - Advanced configuration tuning

## Tuning Tips

### For High Hit Ratio

```go
cfg.LocalCacheConfig.NumCounters = 1e8  // More counters
cfg.LocalCacheConfig.MaxCost = 4 << 30  // Larger cache
```

### For Memory Efficiency

```go
cfg.LocalCacheConfig.NumCounters = 1e6   // Fewer counters
cfg.LocalCacheConfig.MaxCost = 256 << 20 // Smaller cache
```

### For High Throughput

```go
cfg.LocalCacheConfig.BufferItems = 128   // Larger buffer
cfg.LocalCacheConfig.MaxCost = 2 << 30   // Adequate size
```

## Troubleshooting

### Low Hit Ratio

**Cause**: Cache too small or items evicted too quickly

**Solution**: Increase MaxCost
```go
cfg.LocalCacheConfig.MaxCost = 2 << 30 // Increase to 2GB
```

### High Memory Usage

**Cause**: MaxCost or NumCounters set too high

**Solution**: Reduce cache size
```go
cfg.LocalCacheConfig.MaxCost = 512 << 20 // Reduce to 512MB
cfg.LocalCacheConfig.NumCounters = 1e6   // Reduce counters
```

## Related Documentation

- [Main README](../../README.md) - Complete library documentation
- [Ristretto Documentation](https://github.com/dgraph-io/ristretto) - LFU implementation details
- [Comparison Example](../comparison/) - LFU vs LRU comparison
