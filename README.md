# Distributed Cache Library

[![build workflow](https://github.com/huykn/distributed-cache/actions/workflows/go.yml/badge.svg)](https://github.com/huykn/distributed-cache/actions)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/huykn/distributed-cache)](https://pkg.go.dev/github.com/huykn/distributed-cache?tab=doc)
[![GitHub License](https://img.shields.io/github/license/huykn/distributed-cache)](https://github.com/huykn/distributed-cache?tab=readme-ov-file#license)
[![GitHub Repo stars](https://img.shields.io/github/stars/huykn/distributed-cache)](https://github.com/huykn/distributed-cache/stargazers)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/m/huykn/distributed-cache)](https://github.com/huykn/distributed-cache/commits/main/)
[![Go Report Card](https://img.shields.io/badge/go%20report-A%2B-brightgreen)](https://goreportcard.com/report/github.com/huykn/distributed-cache)
[![codecov](https://codecov.io/github/huykn/distributed-cache/graph/badge.svg?token=NZBEQ9QGS7)](https://codecov.io/github/huykn/distributed-cache)


A high-performance, distributed cache library for Go that synchronizes local LFU/LRU caches across multiple service instances using Redis as a backing store and pub/sub for set/invalidation events.

**Version**: v1.0.0

## Features

- **Two-Level Caching**: Fast local in-process LFU(Ristretto)/LRU(golang-lru) with Redis backing store
- **Automatic Synchronization**: Redis Pub/Sub for cache set/invalidation across distributed services
- **High Performance**: LFU/LRU provides excellent hit ratios and throughput
- **Kubernetes Ready**: Designed for containerized environments with pod-aware invalidation
- **Simple API**: Easy-to-use interface for Get, Set, Delete, and Clear operations
- **Metrics**: Built-in statistics collection for monitoring cache performance
- **Flexible Configuration**: Environment variable support for easy deployment

## Architecture

The distributed-cache library uses a two-level caching architecture with local in-process caches synchronized via Redis pub/sub:

```mermaid
sequenceDiagram
    participant AppA as Application (Pod A)
    participant CacheA as SyncedCache (Pod A)
    participant LocalA as Local Cache (Pod A)
    participant Serializer as Marshaller
    participant Redis as Redis Store
    participant PubSub as Redis Pub/Sub
    participant CacheB as SyncedCache (Pod B)
    participant LocalB as Local Cache (Pod B)
    participant AppB as Application (Pod B)

    Note over AppA,AppB: 1. Cache Write Flow - Pod A writes & syncs to all pods
    AppA->>CacheA: Set(key, value)
    CacheA->>Serializer: Marshal(value)
    Serializer-->>CacheA: serialized data
    CacheA->>LocalA: Set(key, data)
    CacheA->>Redis: SET key data
    Redis-->>CacheA: OK
    CacheA->>PubSub: PUBLISH cache:sync {key, data}
    CacheA-->>AppA: Success

    Note over PubSub,LocalB: Other pods receive sync message with data
    PubSub->>CacheB: Sync message {key, data}
    CacheB->>LocalB: Set(key, data)
    Note over LocalB: Local cache synced!<br/>Ready to serve

    Note over AppA,AppB: 2. Cache Read from Pod B - Served from local cache
    AppB->>CacheB: Get(key)
    CacheB->>LocalB: Get(key)
    LocalB-->>CacheB: cached data ✓
    CacheB->>Serializer: Unmarshal(data)
    Serializer-->>CacheB: value
    CacheB-->>AppB: value (from local, no Redis/DB query!)

    Note over AppA,AppB: 3. Cache Miss Flow - Pod A fetches & syncs to all pods
    AppA->>CacheA: Get(key)
    CacheA->>LocalA: Get(key)
    LocalA-->>CacheA: nil (miss)
    CacheA->>Redis: GET key
    alt Data exists in Redis
        Redis-->>CacheA: data
    else Data not in Redis (fetch from DB)
        Redis-->>CacheA: nil
        Note over CacheA: Fetch from DB (app logic)
        CacheA->>Serializer: Marshal(value from DB)
        Serializer-->>CacheA: serialized data
        CacheA->>Redis: SET key data
    end
    CacheA->>LocalA: Set(key, data)
    CacheA->>PubSub: PUBLISH cache:sync {key, data}
    Note over PubSub: Broadcast data to all pods
    PubSub->>CacheB: Sync message {key, data}
    CacheB->>LocalB: Set(key, data)
    Note over LocalB: Pod B now has the data!
    CacheA->>Serializer: Unmarshal(data)
    Serializer-->>CacheA: value
    CacheA-->>AppA: value

    Note over AppA,AppB: 4. Next request to Pod B - Instant response
    AppB->>CacheB: Get(key)
    CacheB->>LocalB: Get(key)
    LocalB-->>CacheB: cached data ✓
    CacheB->>Serializer: Unmarshal(data)
    Serializer-->>CacheB: value
    CacheB-->>AppB: value (instant, from local!)

    Note over AppA,AppB: 5. Cache Delete Flow - Invalidate all pods
    AppA->>CacheA: Delete(key)
    CacheA->>LocalA: Delete(key)
    CacheA->>Redis: DEL key
    Redis-->>CacheA: OK
    CacheA->>PubSub: PUBLISH cache:invalidate key
    CacheA-->>AppA: Success
    PubSub->>CacheB: Invalidation message (key)
    CacheB->>LocalB: Delete(key)
    Note over LocalB: Cache invalidated
```

### Key Components

- **SyncedCache**: Main API providing Get, Set, Delete, Clear operations
- **Local Cache**: In-process cache build-in(LFU via Ristretto or LRU via golang-lru) or custom implementation
- **Redis Store**: Persistent backing store for cache data
- **Redis Pub/Sub**: Synchronization channel for cache invalidation and value propagation
- **Marshaller**: Pluggable serialization (JSON, MessagePack, Protobuf, etc.)
- **Logger**: Pluggable logging interface (Console, Zap, Slog, etc.)

### Data Flow

1. **Set Operation**: Value stored in local cache → Redis → Pub/sub event sent to other pods
2. **Get Operation (Local Hit)**: Value retrieved from local cache (~100ns)
3. **Get Operation (Remote Hit)**: Value fetched from Redis → Stored in local cache (~1-5ms)
4. **Synchronization**: Other pods receive pub/sub event → Update/invalidate local cache

## Installation

```bash
go get github.com/huykn/distributed-cache
```

## Quick Start

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
	opts.PodID = "pod-1"
	opts.RedisAddr = "localhost:6379"

	c, err := cache.New(opts)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set a value
	if err := c.Set(ctx, "user:123", map[string]string{
		"name": "John Doe",
		"email": "john@example.com",
	}); err != nil {
		log.Fatal(err)
	}

	// Get a value
	value, found := c.Get(ctx, "user:123")
	if found {
		log.Printf("Found: %v", value)
	}

	// Delete a value
	if err := c.Delete(ctx, "user:123"); err != nil {
		log.Fatal(err)
	}

	// Get cache statistics
	stats := c.Stats()
	log.Printf("Stats: %+v", stats)
}
```

## Configuration

### Programmatic Configuration

```go
opts := cache.Options{
	PodID:                   "pod-1",
	RedisAddr:               "redis.default.svc.cluster.local:6379",
	RedisPassword:           "secret",
	RedisDB:                 0,
	InvalidationChannel:     "cache:invalidate",
	SerializationFormat:     "json",
	ContextTimeout:          5 * time.Second,
	EnableMetrics:           true,
	LocalCacheConfig: cache.LocalCacheConfig{
		NumCounters:        1e7,
		MaxCost:            1 << 30,
		BufferItems:        64,
		IgnoreInternalCost: false,
	},
}

c, err := cache.New(opts)
```

## API Reference

### Cache Interface

```go
type Cache interface {
	Get(ctx context.Context, key string) (any, bool)
	Set(ctx context.Context, key string, value any) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Close() error
	Stats() Stats
}
```

## Performance Characteristics

- **Local Cache Hit**: ~100ns (in-process)
- **Remote Cache Hit**: ~1-5ms (Redis round-trip)
- **Cache Miss**: ~1-5ms (Redis lookup)
- **Set Operation**: ~1-5ms (Redis + Pub/Sub)

## Examples

The library includes comprehensive examples demonstrating various features and use cases:

### Getting Started

- **[Basic Example](examples/basic/)** - Core functionality, multi-pod synchronization, and value propagation
- **[Debug Mode](examples/debug-mode/)** - Detailed logging and troubleshooting techniques

### Cache Strategies

- **[LFU Cache](examples/lfu/)** - Least Frequently Used cache (default) for varying access patterns
- **[LRU Cache](examples/lru/)** - Least Recently Used cache for sequential access patterns
- **[LFU vs LRU Comparison](examples/comparison/)** - Side-by-side comparison to help choose the right strategy

### Customization

- **[Custom Logger](examples/custom-logger/)** - Integrate with Zap, Slog, Logrus, or custom logging systems
- **[Custom Marshaller](examples/custom-marshaller/)** - Use MessagePack, Protobuf, compression, or encryption
- **[Custom Local Cache](examples/custom-local-cache/)** - Implement custom eviction strategies and storage
- **[Custom Configuration](examples/custom-config/)** - Advanced tuning for production environments

### Production Deployment

- **[Kubernetes](examples/kubernetes/)** - Multi-pod deployment with HTTP API, health checks, and environment configuration

Each example includes:
- Detailed README with explanation
- Runnable code demonstrating the feature
- Expected output and behavior
- Configuration options and best practices
- Troubleshooting tips

**Quick Start**: Begin with the [Basic Example](examples/basic/) to understand core concepts, then explore other examples based on your needs.

## Kubernetes Deployment

See the [Kubernetes example](examples/kubernetes/) for a complete production deployment with:
- Multiple pod replicas with synchronized caches
- Environment-based configuration using ConfigMaps
- Health checks and readiness probes
- HTTP API endpoints for cache operations
- Value propagation across all pods

## Contributing

Contributions are welcome! Please see CONTRIBUTING.md for guidelines.

## License

MIT License - see LICENSE file for details.
