# Basic Example

This example demonstrates the fundamental usage of the distributed-cache library, including both root-level initialization and direct cache package usage.

## What This Example Demonstrates

- **Root-level initialization** using `dc.New()` (recommended approach)
- **Direct cache package usage** using `cache.New()`
- **Basic cache operations**: Set, Get, Delete
- **Multi-pod cache synchronization** with value propagation
- **Debug logging** to observe cache behavior
- **Local and remote cache hits**

## Key Features Shown

### 1. Two Initialization Methods

The example shows both ways to initialize the cache:

```go
// Method 1: Root-level (recommended)
rootCfg := dc.DefaultConfig()
rootCfg.PodID = "example-pod-1"
rootCfg.RedisAddr = "localhost:6379"
c, err := dc.New(rootCfg)

// Method 2: Direct cache package (backward compatible)
cacheCfg := cache.DefaultOptions()
cacheCfg.PodID = "example-pod-2"
c2, err := cache.New(cacheCfg)
```

### 2. Value Propagation

When one pod sets a value, other pods receive the value directly through pub/sub, eliminating the need to fetch from Redis:

```go
// Pod-1 sets a value
c.Set(ctx, "user:456", user2)

// Pod-3 updates the value - propagates to Pod-1's local cache
c3.Set(ctx, "user:456", updatedUser)

// Pod-1 gets the updated value from local cache (no Redis fetch!)
value, found := c.Get(ctx, "user:456")
```

### 3. Debug Logging

Enable debug mode to see detailed cache operations:

```go
cfg.DebugMode = true
cfg.Logger = cache.NewConsoleLogger("pod-1")
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
cd examples/basic
go run main.go
```

## Expected Output

The example will output:

1. **Root-level initialization** confirmation
2. **Cache package initialization** confirmation
3. **Set and Get operations** with debug logs showing:
   - Local cache storage
   - Remote cache storage
   - Pub/sub event publishing
4. **Multi-pod synchronization** demonstration showing:
   - Pod-1 setting a value
   - Pod-3 fetching from Redis (remote hit)
   - Pod-3 updating the value
   - Value propagation to Pod-1's local cache
   - Pod-1 getting the updated value from local cache (no Redis fetch)
5. **Cache statistics** showing hits, misses, and hit ratios

### Sample Output

```
=== Using Root-Level Initialization ===
✓ Cache initialized

=== Using Cache Package Directly ===
✓ Cache initialized

=== Example 1: Set and Get ===
Set user:123 = {ID:123 Name:John Doe Email:john@example.com}
Got user:123 = {ID:123 Name:John Doe Email:john@example.com}

=== Example 2: Multi-Pod Cache Synchronization (Value Propagation) ===
Pod-1 sets a value...
  Pod-1: Set user:456 = {ID:456 Name:Jane Smith Email:jane@example.com}

Simulating Pod-3 (another instance)...
  Pod-3: Initialized and subscribed to synchronization events

Pod-3 fetches the value (should come from Redis)...
  Pod-3: Got user:456 from remote = {ID:456 Name:Jane Smith Email:jane@example.com}

Pod-3 updates the value (should propagate to Pod-1's local cache)...
  Pod-3: Updated user:456 = {ID:456 Name:Jane Smith-Updated Email:jane.updated@example.com}

Waiting for synchronization event to propagate...

Pod-1 fetches the value (should be in local cache from propagation)...
  Pod-1: Got user:456 = {ID:456 Name:Jane Smith-Updated Email:jane.updated@example.com}
  ✓ Value propagation successful! Pod-1 got the updated value from local cache.
```

## Configuration Options

The example uses these configuration options:

- `PodID`: Unique identifier for each pod instance
- `RedisAddr`: Redis server address
- `DebugMode`: Enable detailed logging
- `Logger`: Custom logger implementation

## What You'll Learn

1. **How to initialize the cache** using both methods
2. **How to perform basic operations** (Set, Get, Delete)
3. **How multi-pod synchronization works** with value propagation
4. **How to enable debug logging** to observe cache behavior
5. **The difference between local and remote cache hits**
6. **How values propagate across pods** without Redis fetches

## Next Steps

After understanding this basic example, explore:

- **[LFU Example](../lfu/)** - Learn about Least Frequently Used cache eviction
- **[LRU Example](../lru/)** - Learn about Least Recently Used cache eviction
- **[Custom Logger](../custom-logger/)** - Implement custom logging
- **[Custom Marshaller](../custom-marshaller/)** - Implement custom serialization
- **[Debug Mode](../debug-mode/)** - Deep dive into debug logging
- **[Kubernetes](../kubernetes/)** - Production deployment example

## Troubleshooting

### "connection refused" Error

**Cause**: Redis is not running

**Solution**: Start Redis server
```bash
redis-server
# or
docker run -d -p 6379:6379 redis:latest
```

### "context deadline exceeded" Error

**Cause**: Redis is slow or unreachable

**Solution**: Check Redis connectivity and increase timeout
```go
cfg.ContextTimeout = 10 * time.Second
```

## Related Documentation

- [Main README](../../README.md) - Complete library documentation
- [Getting Started Guide](../../GETTING_STARTED.md) - Step-by-step setup
- [API Reference](../../README.md#api-reference) - Full API documentation
