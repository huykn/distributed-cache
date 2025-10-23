# Getting Started with Distributed Cache Library

## 5-Minute Quick Start

### 1. Install the Library

```bash
go get github.com/huykn/distributed-cache
```

### 2. Create a Cache Instance

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/huykn/distributed-cache/cache"
)

func main() {
    // Create cache with default options
    opts := cache.DefaultOptions()
    opts.PodID = "my-app-pod"
    opts.RedisAddr = "localhost:6379"
    
    c, err := cache.New(opts)
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    // Use the cache
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Set a value
    c.Set(ctx, "greeting", "Hello, World!")
    
    // Get the value
    value, found := c.Get(ctx, "greeting")
    if found {
        log.Printf("Got: %v", value)
    }
}
```

### 3. Run It

```bash
# Make sure Redis is running
redis-server

# Run your application
go run main.go
```

## Prerequisites

### Required
- Go 1.24 or later
- Redis server (local or remote)

### Optional
- Docker (for running Redis in container)
- Kubernetes (for production deployment)

## Installation Steps

### Step 1: Add to Your Project

```bash
cd your-project
go get github.com/huykn/distributed-cache
```

### Step 2: Import the Package

```go
import "github.com/huykn/distributed-cache/cache"
```

### Step 3: Initialize Cache

```go
opts := cache.DefaultOptions()
opts.PodID = "my-pod"
opts.RedisAddr = "redis.example.com:6379"

c, err := cache.New(opts)
if err != nil {
    log.Fatal(err)
}
defer c.Close()
```

## Common Use Cases

### Use Case 1: Simple Key-Value Store

```go
// Set
c.Set(ctx, "user:123", User{ID: 123, Name: "John"})

// Get
user, found := c.Get(ctx, "user:123")
if found {
    fmt.Println(user)
}
```

### Use Case 2: Cache with Fallback

```go
// Try cache first
value, found := c.Get(ctx, "expensive-key")
if !found {
    // Compute or fetch from database
    value = computeExpensiveValue()
    // Store in cache
    c.Set(ctx, "expensive-key", value)
}
return value
```

### Use Case 3: Distributed Cache Invalidation

```go
// Pod A updates data
c.Set(ctx, "config:app", newConfig)

// Pod B automatically gets invalidation event
// and removes from local cache
// Next Get will fetch from Redis
```

### Use Case 4: Cache Statistics

```go
stats := c.Stats()
fmt.Printf("Hit Rate: %.2f%%\n", 
    float64(stats.LocalHits) / 
    float64(stats.LocalHits + stats.LocalMisses) * 100)
```

## Configuration

### Minimal Configuration

```go
opts := cache.DefaultOptions()
opts.PodID = "pod-1"
opts.RedisAddr = "redis:6379"
```

### Full Configuration

```go
opts := cache.Options{
    PodID:               "pod-1",
    RedisAddr:           "redis.default.svc.cluster.local:6379",
    RedisPassword:       "secret",
    RedisDB:             0,
    InvalidationChannel: "cache:invalidate",
    SerializationFormat: "json",
    ContextTimeout:      5 * time.Second,
    EnableMetrics:       true,
    LocalCacheConfig: cache.LocalCacheConfig{
        NumCounters:        1e7,
        MaxCost:            1 << 30,
        BufferItems:        64,
        IgnoreInternalCost: false,
    },
}
```

### Environment Variables

```bash
export CACHE_POD_ID=pod-1
export CACHE_REDIS_ADDR=redis:6379
export CACHE_REDIS_PASSWORD=secret
export CACHE_CONTEXT_TIMEOUT=5s
```

## Testing Your Setup

### Test 1: Redis Connection

```go
opts := cache.DefaultOptions()
opts.RedisAddr = "localhost:6379"

c, err := cache.New(opts)
if err != nil {
    log.Fatal("Redis connection failed:", err)
}
defer c.Close()
log.Println("✓ Redis connection successful")
```

### Test 2: Basic Operations

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Set
err := c.Set(ctx, "test", "value")
if err != nil {
    log.Fatal("Set failed:", err)
}
log.Println("✓ Set successful")

// Get
value, found := c.Get(ctx, "test")
if !found {
    log.Fatal("Get failed: value not found")
}
log.Println("✓ Get successful:", value)

// Delete
err = c.Delete(ctx, "test")
if err != nil {
    log.Fatal("Delete failed:", err)
}
log.Println("✓ Delete successful")
```

### Test 3: Multi-Pod Synchronization

```go
// Pod 1
c1, _ := cache.New(opts1)
c1.Set(ctx, "key", "value1")

// Pod 2
c2, _ := cache.New(opts2)
value, _ := c2.Get(ctx, "key")  // Gets from Redis
log.Println("✓ Multi-pod sync works:", value)
```

## Troubleshooting

### Issue: "connection refused"
**Cause**: Redis not running
**Solution**: Start Redis
```bash
redis-server
# or with Docker
docker run -d -p 6379:6379 redis:latest
```

### Issue: "context deadline exceeded"
**Cause**: Timeout too short or Redis slow
**Solution**: Increase timeout
```go
opts.ContextTimeout = 10 * time.Second
```

### Issue: "serialization failed"
**Cause**: Value not JSON-serializable
**Solution**: Use JSON-compatible types
```go
// Good
c.Set(ctx, "key", map[string]any{"name": "John"})

// Bad - functions not serializable
c.Set(ctx, "key", func() {})
```

## Next Steps

1. **Read Documentation**
   - See README.md for full API
   - See QUICK_REFERENCE.md for common patterns

2. **Review Examples**
   - Start with [examples/basic](examples/basic/) for simple usage
   - Explore cache strategies: [LFU](examples/lfu/), [LRU](examples/lru/), [Comparison](examples/comparison/)
   - Learn customization: [Custom Logger](examples/custom-logger/), [Custom Marshaller](examples/custom-marshaller/), [Custom Local Cache](examples/custom-local-cache/)
   - Advanced topics: [Custom Config](examples/custom-config/), [Debug Mode](examples/debug-mode/)
   - Production deployment: [Kubernetes](examples/kubernetes/)

## Examples Overview

The library includes comprehensive examples for different use cases:

### Getting Started Examples

| Example | Description | Best For |
|---------|-------------|----------|
| [Basic](examples/basic/) | Fundamental cache operations and multi-pod sync | First-time users |
| [Debug Mode](examples/debug-mode/) | Detailed logging and troubleshooting | Development & debugging |

### Cache Strategy Examples

| Example | Description | Best For |
|---------|-------------|----------|
| [LFU](examples/lfu/) | Least Frequently Used cache (default) | Varying access patterns |
| [LRU](examples/lru/) | Least Recently Used cache | Sequential access patterns |
| [Comparison](examples/comparison/) | Side-by-side LFU vs LRU comparison | Choosing cache strategy |

### Customization Examples

| Example | Description | Best For |
|---------|-------------|----------|
| [Custom Logger](examples/custom-logger/) | Implement custom logging | Integration with logging systems |
| [Custom Marshaller](examples/custom-marshaller/) | Custom serialization formats | MessagePack, Protobuf, compression |
| [Custom Local Cache](examples/custom-local-cache/) | Custom cache implementation | Specialized eviction strategies |
| [Custom Config](examples/custom-config/) | Advanced configuration tuning | Production optimization |

### Production Examples

| Example | Description | Best For |
|---------|-------------|----------|
| [Kubernetes](examples/kubernetes/) | Multi-pod deployment with HTTP API | Production deployment |

Each example includes a detailed README with:
- What the example demonstrates
- How to run it
- Expected output
- Configuration options
- Use cases and best practices

## Learning Resources

| Resource | Purpose |
|----------|---------|
| README.md | Complete API documentation |
| QUICK_REFERENCE.md | Quick lookup guide |
| MIGRATION_GUIDE.md | Integration instructions |
| [examples/basic](examples/basic/) | Simple example to get started |
| [examples/kubernetes](examples/kubernetes/) | Production deployment example |
| CONTRIBUTING.md | How to contribute |

## Getting Help

1. **Check Documentation**: Most questions answered in README.md
2. **Review Examples**: See examples/ directory
3. **Check Tests**: See *_test.go files for usage patterns
4. **Report Issues**: Use GitHub Issues

## Success Checklist

- [ ] Go 1.24+ installed
- [ ] Redis running locally or accessible
- [ ] Library installed: `go get github.com/huykn/distributed-cache`
- [ ] Basic example runs successfully
- [ ] Can set and get values
- [ ] Can delete values
- [ ] Statistics working
- [ ] Ready to integrate into your app

## What's Next?

Once you've completed the quick start:

1. **Explore Examples**
   - Run [examples/basic](examples/basic/) to see core functionality
   - Try [examples/debug-mode](examples/debug-mode/) to understand cache behavior
   - Compare [LFU vs LRU](examples/comparison/) to choose the right strategy
   - Learn [customization options](examples/custom-logger/) for your needs

2. **Integrate into Your Application**
   - Replace your existing cache implementation
   - Update all cache operations to use new API
   - Test thoroughly with your workload

3. **Configure for Your Environment**
   - Set environment variables
   - Configure Redis connection
   - Tune cache size parameters (see [Custom Config](examples/custom-config/))

4. **Deploy to Production**
   - Follow [Kubernetes example](examples/kubernetes/) for deployment patterns
   - Set up Redis in your infrastructure
   - Deploy application with cache
   - Monitor performance

5. **Optimize**
   - Track cache hit rates
   - Adjust configuration as needed
   - Monitor memory usage
   - Review examples for optimization tips

---

**Ready to get started?** Follow the 5-minute quick start above!

**Want to learn more?** Explore the [examples directory](examples/) for detailed guides

**Need help?** See CONTRIBUTING.md for support options
