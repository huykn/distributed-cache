# Custom Configuration Example

This example demonstrates how to customize the distributed-cache configuration with advanced settings for production environments.

## What This Example Demonstrates

- **Custom local cache configuration** with tuned parameters
- **Redis connection settings** including password and database selection
- **Cache size and performance tuning** for specific workloads
- **Environment-specific configuration** for different deployment scenarios
- **LFU cache factory** with custom parameters

## Key Configuration Options

### Local Cache Configuration

```go
cfg.LocalCacheConfig = dc.LocalCacheConfig{
    NumCounters:        1e7,     // 10 million counters for frequency tracking
    MaxCost:            1 << 30, // 1GB maximum cache size
    BufferItems:        64,      // Buffer size for async operations
    IgnoreInternalCost: false,   // Track actual memory cost
}
```

### Redis Configuration

```go
cfg.RedisAddr = "localhost:6379"
cfg.RedisPassword = ""           // Set if Redis requires authentication
cfg.RedisDB = 0                  // Redis database number (0-15)
cfg.InvalidationChannel = "cache:invalidate"
```

### Performance Configuration

```go
cfg.ContextTimeout = 5 * time.Second
cfg.EnableMetrics = true
cfg.DebugMode = false
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
cd examples/custom-config
go run main.go
```

## Expected Output

```
========================================
Custom Configuration Example
========================================

Configuration Overview:
  PodID: custom-config-pod
  Redis: localhost:6379
  Database: 0
  Channel: cache:invalidate
  Timeout: 5s

Local Cache Configuration:
  NumCounters: 10000000
  MaxCost: 1073741824 bytes (1.00 GB)
  BufferItems: 64
  IgnoreInternalCost: false

Creating cache with custom configuration...
✓ Cache initialized with custom config

Testing cache operations...
✓ Set 10 users successfully

Cache Statistics:
  Local Hits: 10
  Local Misses: 0
  Hit Ratio: 100.00%
  Local Size: 10 items

========================================
Configuration Tips:
  - NumCounters: 10x expected items
  - MaxCost: Based on available memory
  - BufferItems: 64 is good default
  - Monitor metrics to tune parameters
========================================
```

## Configuration Parameters Explained

### NumCounters

- **Purpose**: Number of counters for frequency tracking in LFU cache
- **Recommendation**: 10x the expected number of unique items
- **Example**: For 1M items, use 10M counters
- **Impact**: Higher values improve accuracy but use more memory

### MaxCost

- **Purpose**: Maximum memory cost for the local cache
- **Recommendation**: Based on available memory (e.g., 1GB = 1 << 30)
- **Example**: For 2GB cache, use `2 << 30`
- **Impact**: Determines how much data can be cached locally

### BufferItems

- **Purpose**: Size of the buffer for async operations
- **Recommendation**: 64 is a good default for most workloads
- **Example**: Increase to 128 for high-throughput scenarios
- **Impact**: Higher values improve throughput but use more memory

### IgnoreInternalCost

- **Purpose**: Whether to ignore internal memory overhead
- **Recommendation**: `false` for accurate memory tracking
- **Example**: Set to `true` if you want to ignore metadata overhead
- **Impact**: Affects how MaxCost is calculated

## Environment-Specific Configurations

### Development Environment

```go
cfg := dc.DefaultConfig()
cfg.PodID = "dev-pod"
cfg.RedisAddr = "localhost:6379"
cfg.DebugMode = true
cfg.LocalCacheConfig.MaxCost = 100 << 20 // 100MB
```

### Staging Environment

```go
cfg := dc.DefaultConfig()
cfg.PodID = os.Getenv("POD_NAME")
cfg.RedisAddr = "redis.staging.svc.cluster.local:6379"
cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")
cfg.DebugMode = false
cfg.LocalCacheConfig.MaxCost = 512 << 20 // 512MB
```

### Production Environment

```go
cfg := dc.DefaultConfig()
cfg.PodID = os.Getenv("POD_NAME")
cfg.RedisAddr = os.Getenv("REDIS_ADDR")
cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")
cfg.RedisDB = 0
cfg.DebugMode = false
cfg.EnableMetrics = true
cfg.LocalCacheConfig.MaxCost = 2 << 30 // 2GB
cfg.LocalCacheConfig.NumCounters = 1e8 // 100M counters
```

## Tuning Guidelines

### For High-Throughput Workloads

```go
cfg.LocalCacheConfig.BufferItems = 128
cfg.LocalCacheConfig.MaxCost = 4 << 30 // 4GB
cfg.ContextTimeout = 10 * time.Second
```

### For Memory-Constrained Environments

```go
cfg.LocalCacheConfig.MaxCost = 256 << 20 // 256MB
cfg.LocalCacheConfig.NumCounters = 1e6   // 1M counters
cfg.LocalCacheConfig.BufferItems = 32
```

### For Large Item Sizes

```go
cfg.LocalCacheConfig.MaxCost = 8 << 30 // 8GB
cfg.LocalCacheConfig.IgnoreInternalCost = true
```

## What You'll Learn

1. **How to customize cache configuration** for different environments
2. **How to tune performance parameters** based on workload
3. **How to configure Redis connection** with authentication
4. **How to optimize memory usage** with MaxCost and NumCounters
5. **How to monitor and adjust** configuration based on metrics

## Next Steps

After understanding custom configuration, explore:

- **[Basic Example](../basic/)** - Learn basic cache operations
- **[LFU Example](../lfu/)** - Understand LFU cache behavior
- **[LRU Example](../lru/)** - Understand LRU cache behavior
- **[Kubernetes Example](../kubernetes/)** - Production deployment with environment variables

## Troubleshooting

### High Memory Usage

**Cause**: MaxCost or NumCounters set too high

**Solution**: Reduce cache size
```go
cfg.LocalCacheConfig.MaxCost = 512 << 20 // 512MB instead of 1GB
cfg.LocalCacheConfig.NumCounters = 1e6   // 1M instead of 10M
```

### Low Hit Ratio

**Cause**: Cache size too small for workload

**Solution**: Increase MaxCost
```go
cfg.LocalCacheConfig.MaxCost = 2 << 30 // Increase to 2GB
```

### Slow Performance

**Cause**: BufferItems too small for high throughput

**Solution**: Increase buffer size
```go
cfg.LocalCacheConfig.BufferItems = 128
```

## Related Documentation

- [Main README](../../README.md) - Complete library documentation
- [Configuration Section](../../README.md#configuration) - All configuration options
- [Getting Started Guide](../../GETTING_STARTED.md) - Basic setup
