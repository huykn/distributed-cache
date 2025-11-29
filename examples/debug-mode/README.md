# Debug Mode Example

This example demonstrates how to enable and use debug mode in the distributed-cache library to observe detailed cache operations and troubleshoot issues.

## What This Example Demonstrates

- **Debug mode activation** for detailed logging
- **Cache operation visibility** (Set, Get, Delete, SetWithInvalidate)
- **Local and remote cache interactions** with debug output
- **Synchronization event logging** for multi-pod scenarios
- **Troubleshooting techniques** using debug logs

## Key Features

### Debug Mode Configuration

```go
cfg := dc.DefaultConfig()
cfg.DebugMode = true
cfg.Logger = cache.NewConsoleLogger("DistributedCache")
```

### Operations Logged

- **Set operations**: Local cache storage, Redis storage, pub/sub publishing
- **Get operations**: Local cache checks, Redis fetches, cache hits/misses
- **Delete operations**: Local cache removal, Redis deletion, invalidation events
- **SetWithInvalidate**: Invalidate-only mode for other pods
- **Synchronization events**: Received events, value propagation, cache updates

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
cd examples/debug-mode
go run main.go
```

## Expected Output

```
========================================
Debug Mode Example
========================================

Debug Mode Overview:
  Purpose: Observe cache operations in detail
  Use Cases: Development, troubleshooting, learning
  Output: Detailed logs for every operation

Creating cache with debug mode enabled...
[DEBUG] DistributedCache: Initializing cache [podID debug-pod]
[DEBUG] DistributedCache: Connected to Redis [addr localhost:6379]
[DEBUG] DistributedCache: Subscribed to invalidation channel [channel cache:invalidate]
✓ Cache initialized with debug mode

--- Example 1: Set Operation ---
[DEBUG] DistributedCache: Set: storing in local cache [key user:1]
[DEBUG] DistributedCache: Set: stored in remote cache [key user:1]
[DEBUG] DistributedCache: Set: publishing synchronization event [key user:1 action set]
✓ Set user:1 successfully

--- Example 2: Get Operation (Local Cache Hit) ---
[DEBUG] DistributedCache: Get: checking local cache [key user:1]
[DEBUG] DistributedCache: Get: local cache hit [key user:1]
Retrieved: {ID:1 Name:Alice Email:alice@example.com}

--- Example 2b: SetWithInvalidate Operation (Invalidate-Only) ---
[DEBUG] DistributedCache: Set: storing in local cache [key user:2]
[DEBUG] DistributedCache: Set: stored in remote cache [key user:2]
[DEBUG] DistributedCache: Set: publishing synchronization event [key user:2 action invalidate]

--- Example 3: Get Operation (Cache Miss) ---
[DEBUG] DistributedCache: Get: checking local cache [key user:999]
[DEBUG] DistributedCache: Get: local cache miss [key user:999]
[DEBUG] DistributedCache: Get: checking remote cache [key user:999]
[DEBUG] DistributedCache: Get: remote cache miss [key user:999]
Expected miss: user not found

--- Example 4: Delete Operation ---
[DEBUG] DistributedCache: Delete: removing from local cache [key user:1]
[DEBUG] DistributedCache: Delete: removed from remote cache [key user:1]
[DEBUG] DistributedCache: Delete: publishing invalidation event [key user:1 action delete]
✓ Deleted user:1 successfully

========================================
Debug Mode Benefits:
  ✓ Visibility into cache operations
  ✓ Understand local vs remote hits
  ✓ Troubleshoot synchronization issues
  ✓ Monitor pub/sub events
  ✓ Learn cache behavior
========================================
```

## Debug Log Messages Explained

### Set Operation Logs

```
[DEBUG] Set: storing in local cache [key user:1]
```
- Cache stores value in local in-memory cache first

```
[DEBUG] Set: stored in remote cache [key user:1]
```
- Cache stores value in Redis for persistence and sharing

```
[DEBUG] Set: publishing synchronization event [key user:1 action set]
```
- Cache publishes event to notify other pods about the new value

### Get Operation Logs (Local Hit)

```
[DEBUG] Get: checking local cache [key user:1]
[DEBUG] Get: local cache hit [key user:1]
```
- Value found in local cache (fastest path, ~100ns)

### Get Operation Logs (Remote Hit)

```
[DEBUG] Get: checking local cache [key user:1]
[DEBUG] Get: local cache miss [key user:1]
[DEBUG] Get: checking remote cache [key user:1]
[DEBUG] Get: remote cache hit [key user:1]
[DEBUG] Get: storing in local cache [key user:1]
```
- Value not in local cache, fetched from Redis (~1-5ms)
- Value stored in local cache for future hits

### Get Operation Logs (Miss)

```
[DEBUG] Get: checking local cache [key user:999]
[DEBUG] Get: local cache miss [key user:999]
[DEBUG] Get: checking remote cache [key user:999]
[DEBUG] Get: remote cache miss [key user:999]
```
- Value not found in either cache

### Delete Operation Logs

```
[DEBUG] Delete: removing from local cache [key user:1]
[DEBUG] Delete: removed from remote cache [key user:1]
[DEBUG] Delete: publishing invalidation event [key user:1 action delete]
```
- Value removed from both caches
- Other pods notified to remove from their local caches

### Synchronization Event Logs

```
[DEBUG] Received invalidation event [key user:1 sender pod-2 action set]
[DEBUG] Updating local cache with propagated value [key user:1]
```
- Pod receives event from another pod
- Local cache updated with new value (value propagation)

```
[DEBUG] Received invalidation event [key user:2 sender pod-3 action invalidate]
[DEBUG] Removing key from local cache [key user:2]
```
- Pod receives invalidate-only event
- Local cache entry removed (will fetch from Redis if needed)

## Use Cases for Debug Mode

### 1. Development

Enable debug mode during development to understand cache behavior:

```go
cfg.DebugMode = true
cfg.Logger = cache.NewConsoleLogger("Dev")
```

### 2. Troubleshooting

Debug synchronization issues between pods:

```go
// Enable on all pods to see event flow
cfg.DebugMode = true
cfg.Logger = NewFileLogger("/var/log/cache-debug.log")
```

### 3. Performance Analysis

Identify local vs remote cache hits:

```go
cfg.DebugMode = true
// Analyze logs to see hit patterns
```

### 4. Learning

Understand how distributed caching works:

```go
cfg.DebugMode = true
// Run examples and observe the logs
```

## Debug Mode vs Production

### Development/Debug Mode

```go
cfg := dc.DefaultConfig()
cfg.DebugMode = true
cfg.Logger = cache.NewConsoleLogger("Debug")
```

### Production Mode

```go
cfg := dc.DefaultConfig()
cfg.DebugMode = false
cfg.Logger = NewProductionLogger() // Structured logging (Zap, Slog)
```

## What You'll Learn

1. **How to enable debug mode** for detailed logging
2. **What happens during each cache operation** (Set, Get, Delete)
3. **How local and remote caches interact** in the two-level architecture
4. **How synchronization events work** across multiple pods
5. **How to troubleshoot cache issues** using debug logs
6. **The difference between Set and SetWithInvalidate** operations

## Next Steps

After understanding debug mode, explore:

- **[Basic Example](../basic/)** - Learn basic cache operations
- **[Custom Logger](../custom-logger/)** - Implement custom logging
- **[Kubernetes Example](../kubernetes/)** - Production deployment

## Best Practices

1. **Enable debug mode in development** for visibility
2. **Disable debug mode in production** to reduce log volume
3. **Use structured logging in production** (Zap, Slog, Logrus)
4. **Log to files or aggregation systems** for persistence
5. **Monitor specific operations** when troubleshooting
6. **Combine with metrics** for comprehensive observability

## Troubleshooting with Debug Mode

### Issue: Values Not Synchronizing

**Debug Logs to Check**:
```
[DEBUG] Set: publishing synchronization event [key user:1 action set]
[DEBUG] Received invalidation event [key user:1 sender pod-2 action set]
```

**Solution**: Ensure all pods are subscribed to the same channel

### Issue: High Cache Misses

**Debug Logs to Check**:
```
[DEBUG] Get: local cache miss [key user:1]
[DEBUG] Get: remote cache miss [key user:1]
```

**Solution**: Check if values are being set correctly and not evicted too quickly

### Issue: Slow Performance

**Debug Logs to Check**:
```
[DEBUG] Get: local cache miss [key user:1]
[DEBUG] Get: checking remote cache [key user:1]
```

**Solution**: Too many remote hits indicate local cache is too small or evicting too aggressively

## Related Documentation

- [Main README](../../README.md) - Complete library documentation
- [Custom Logger Example](../custom-logger/) - Custom logging implementation
- [Getting Started Guide](../../GETTING_STARTED.md) - Basic setup
