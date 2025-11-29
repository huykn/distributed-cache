# Heavy-Read API Architecture

## Overview

This example demonstrates a production-ready architecture for handling heavy-read workloads with ultra-low latency. The system is designed to handle 1m+ requests per second with sub-millisecond read latency.

## Architecture Diagram

```mermaid
graph TB
    Client[CLIENT LAYER]

    Client --> APISIX[APISIX GATEWAY<br/>Port 9080<br/>• Standalone Mode<br/>• Round-Robin Load Balancing<br/>• Request ID Tracking<br/>• Prometheus Metrics]

    APISIX --> Reader1[Reader 1<br/>Port 80<br/>Local Cache<br/>Ristretto<br/>byte store]
    APISIX --> Reader2[Reader 2<br/>Port 80<br/>Local Cache<br/>Ristretto<br/>byte store]
    APISIX --> Reader3[Reader 3<br/>Port 80<br/>Local Cache<br/>Ristretto<br/>byte store]

    Reader1 --> Redis[REDIS<br/>Port 6379<br/>• Pub/Sub Channel: posts_updates<br/>• Cache Store Backup<br/>• Synchronization Hub]
    Reader2 --> Redis
    Reader3 --> Redis

    Writer[WRITER<br/>Port 8080<br/>• Accepts POST /create<br/>• Serializes to byte once<br/>• Publishes via Distributed Cache] --> Redis

    style Client fill:#e1f5ff
    style APISIX fill:#fff4e6
    style Reader1 fill:#e8f5e9
    style Reader2 fill:#e8f5e9
    style Reader3 fill:#e8f5e9
    style Redis fill:#ffebee
    style Writer fill:#f3e5f5
```

## Data Flow

### Write Path (POST /create)

1. **Client** sends POST request to Writer
2. **Writer** receives request and creates Post object
3. **Writer** serializes Post to `[]byte` (JSON) **once**
4. **Writer** calls `cache.Set(ctx, postID, postBytes)`
5. **Distributed Cache** stores in Redis
6. **Distributed Cache** publishes to Redis Pub/Sub channel
7. **All Readers** receive pub/sub event with serialized data
8. **Each Reader** stores `[]byte` directly in local cache
9. **Writer** returns success response

**Total Time**: ~10-50ms (including propagation to all readers)

### Read Path (GET /post?id=X)

1. **Client** sends GET request to APISIX Gateway
2. **APISIX** routes to one of the Reader instances (round-robin)
3. **Reader** checks local cache for postID
4. **Reader** retrieves `[]byte` from local cache (cache hit)
5. **Reader** writes bytes directly to HTTP response (zero-copy)
6. **Client** receives response

**Total Time**: ~0.5-2ms (P99)

### Cache Miss Path

1. **Reader** checks local cache → miss
2. **Reader** fetches from Redis
3. **Reader** deserializes to verify data
4. **Reader** stores in local cache
5. **Reader** returns to client

**Total Time**: ~5-10ms (rare, only on cold start)

## Key Optimizations

### 1. Zero-Serialization Read Path

**Traditional Approach:**
```
Redis → []byte → Unmarshal → Object → Marshal → []byte → HTTP Response
```

**Our Approach:**
```
Local Cache → []byte → HTTP Response
```

**Benefit**: 
- No CPU cycles wasted on serialization
- No garbage collection pressure
- Faster read latency

### 2. Pre-Propagation via Pub/Sub

**Traditional Approach:**
```
Write → Redis
Read → Check Local → Miss → Fetch Redis → Deserialize → Cache
```

**Our Approach:**
```
Write → Redis + Pub/Sub → All Readers Update Local Cache
Read → Check Local → Hit → Return
```
