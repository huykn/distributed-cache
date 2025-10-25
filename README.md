# Distributed Cache Library

[![GitHub License](https://img.shields.io/github/license/huykn/distributed-cache)](https://github.com/huykn/distributed-cache?tab=readme-ov-file#license)
[![GitHub Repo stars](https://img.shields.io/github/stars/huykn/distributed-cache)](https://github.com/huykn/distributed-cache/stargazers)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/m/huykn/distributed-cache)](https://github.com/huykn/distributed-cache/commits/main/)
[![Go Report Card](https://img.shields.io/badge/go%20report-A%2B-brightgreen)](https://goreportcard.com/report/github.com/dgraph-io/distributed-cache)


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
graph TB
    subgraph "Application Layer"
        App1[Application Pod A]
        App2[Application Pod B]
        App3[Application Pod C]
    end

    subgraph "Pod A"
        App1 --> Cache1[SyncedCache API]
        Cache1 --> Local1[Local Cache<br/>LFU/LRU]
        Cache1 --> Serializer1[Marshaller<br/>JSON/MessagePack]
        Cache1 --> Logger1[Logger]
    end

    subgraph "Pod B"
        App2 --> Cache2[SyncedCache API]
        Cache2 --> Local2[Local Cache<br/>LFU/LRU]
        Cache2 --> Serializer2[Marshaller<br/>JSON/MessagePack]
        Cache2 --> Logger2[Logger]
    end

    subgraph "Pod C"
        App3 --> Cache3[SyncedCache API]
        Cache3 --> Local3[Local Cache<br/>LFU/LRU]
        Cache3 --> Serializer3[Marshaller<br/>JSON/MessagePack]
        Cache3 --> Logger3[Logger]
    end

    subgraph "Redis Layer"
        Redis[(Redis Store<br/>Persistent Cache)]
        PubSub[Redis Pub/Sub<br/>cache:invalidate]
    end

    Cache1 <--> Redis
    Cache2 <--> Redis
    Cache3 <--> Redis

    Cache1 <--> PubSub
    Cache2 <--> PubSub
    Cache3 <--> PubSub

    style Local1 fill:#e1f5ff
    style Local2 fill:#e1f5ff
    style Local3 fill:#e1f5ff
    style Redis fill:#ffebee
    style PubSub fill:#fff3e0
    style Cache1 fill:#f3e5f5
    style Cache2 fill:#f3e5f5
    style Cache3 fill:#f3e5f5
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
