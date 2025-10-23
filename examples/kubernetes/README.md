# Kubernetes Deployment Example

This example demonstrates how to deploy the distributed-cache library in a Kubernetes environment with multiple pods, showing real-world production usage with environment-based configuration and HTTP API endpoints.

## What This Example Demonstrates

- **Kubernetes deployment** with multiple replicas
- **Environment-based configuration** using ConfigMaps and Secrets
- **Pod-aware cache synchronization** using Downward API
- **HTTP API endpoints** for cache operations
- **Health checks** and readiness probes
- **Multi-pod value propagation** in a distributed environment
- **Production-ready patterns** for cache deployment

## Key Features

### Environment Configuration

```go
// Configuration from environment variables
cfg := config.FromEnv()

// Use pod name as pod ID (from Kubernetes Downward API)
if podName := os.Getenv("POD_NAME"); podName != "" {
    cfg.Cache.PodID = podName
}
```

### HTTP API Endpoints

- `GET /product?id=<id>` - Get product with caching
- `POST /product/update?id=<id>` - Update product (propagates to all pods)
- `POST /product/delete?id=<id>` - Delete product from cache
- `GET /stats` - Get cache statistics
- `GET /health` - Health check endpoint

### Value Propagation

When one pod updates a value, all other pods receive the new value directly:

```go
// Pod A updates product
cache.Set(ctx, "product:1", updatedProduct)

// Pod B, C, D automatically receive the updated value
// No need to fetch from Redis!
```

## Prerequisites

- Kubernetes cluster (local or cloud)
- kubectl configured
- Redis deployed in Kubernetes
- Docker for building images (optional)

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                    │
│                                                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │  Pod A   │  │  Pod B   │  │  Pod C   │              │
│  │  :8080   │  │  :8080   │  │  :8080   │              │
│  │          │  │          │  │          │              │
│  │ Local    │  │ Local    │  │ Local    │              │
│  │ Cache    │  │ Cache    │  │ Cache    │              │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘              │
│       │             │             │                     │
│       └─────────────┼─────────────┘                     │
│                     │                                   │
│              ┌──────▼──────┐                            │
│              │    Redis    │                            │
│              │   Service   │                            │
│              │  :6379      │                            │
│              └─────────────┘                            │
│                                                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │         Redis Pub/Sub Channel                     │  │
│  │         (cache:invalidate)                        │  │
│  │  - Synchronizes cache across all pods             │  │
│  │  - Propagates values directly                     │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

## How to Deploy

### 1. Deploy Redis

Create a Redis deployment and service:

```yaml
# redis-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        ports:
        - containerPort: 6379
---
apiVersion: v1
kind: Service
metadata:
  name: redis
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
```

Apply:
```bash
kubectl apply -f redis-deployment.yaml
```

### 2. Create ConfigMap

```yaml
# cache-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cache-config
data:
  CACHE_REDIS_ADDR: "redis:6379"
  CACHE_INVALIDATION_CHANNEL: "cache:invalidate"
  CACHE_CONTEXT_TIMEOUT: "5s"
  CACHE_ENABLE_METRICS: "true"
```

Apply:
```bash
kubectl apply -f cache-config.yaml
```

### 3. Deploy Application

```yaml
# app-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cache-app
spec:
  replicas: 3  # Multiple pods for distributed caching
  selector:
    matchLabels:
      app: cache-app
  template:
    metadata:
      labels:
        app: cache-app
    spec:
      containers:
      - name: app
        image: your-registry/cache-app:latest
        ports:
        - containerPort: 8080
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: CACHE_REDIS_ADDR
          valueFrom:
            configMapKeyRef:
              name: cache-config
              key: CACHE_REDIS_ADDR
        - name: CACHE_INVALIDATION_CHANNEL
          valueFrom:
            configMapKeyRef:
              name: cache-config
              key: CACHE_INVALIDATION_CHANNEL
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: cache-app
spec:
  selector:
    app: cache-app
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

Apply:
```bash
kubectl apply -f app-deployment.yaml
```

## Testing the Deployment

### 1. Check Pod Status

```bash
kubectl get pods -l app=cache-app
```

Expected output:
```
NAME                         READY   STATUS    RESTARTS   AGE
cache-app-7d9f8b5c4d-abc12   1/1     Running   0          1m
cache-app-7d9f8b5c4d-def34   1/1     Running   0          1m
cache-app-7d9f8b5c4d-ghi56   1/1     Running   0          1m
```

### 2. Test Cache Operations

Get the service endpoint:
```bash
kubectl get svc cache-app
```

Test endpoints:
```bash
# Get product (will cache)
curl http://<service-ip>/product?id=1

# Update product (propagates to all pods)
curl -X POST http://<service-ip>/product/update?id=1

# Get from another pod (should have updated value in local cache)
curl http://<service-ip>/product?id=1

# Check cache statistics
curl http://<service-ip>/stats

# Health check
curl http://<service-ip>/health
```

### 3. Verify Multi-Pod Synchronization

```bash
# Update product from one pod
POD1=$(kubectl get pods -l app=cache-app -o jsonpath='{.items[0].metadata.name}')
kubectl exec $POD1 -- curl -X POST localhost:8080/product/update?id=1

# Check logs on another pod to see synchronization
POD2=$(kubectl get pods -l app=cache-app -o jsonpath='{.items[1].metadata.name}')
kubectl logs $POD2 | grep "Received invalidation event"
```

## Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `POD_NAME` | Unique pod identifier | - | Yes |
| `CACHE_REDIS_ADDR` | Redis server address | localhost:6379 | Yes |
| `CACHE_REDIS_PASSWORD` | Redis password | - | No |
| `CACHE_REDIS_DB` | Redis database number | 0 | No |
| `CACHE_INVALIDATION_CHANNEL` | Pub/sub channel | cache:invalidate | No |
| `CACHE_CONTEXT_TIMEOUT` | Operation timeout | 5s | No |
| `CACHE_ENABLE_METRICS` | Enable metrics | true | No |
| `PORT` | HTTP server port | 8080 | No |

## Production Best Practices

### 1. Resource Limits

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

### 2. Redis High Availability

Use Redis Sentinel or Redis Cluster for production:

```yaml
env:
- name: CACHE_REDIS_ADDR
  value: "redis-sentinel:26379"
```

### 3. Monitoring

Add Prometheus metrics:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

### 4. Horizontal Pod Autoscaling

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: cache-app-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: cache-app
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### 5. Pod Disruption Budget

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: cache-app-pdb
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: cache-app
```

## What You'll Learn

1. **How to deploy distributed-cache in Kubernetes** with multiple replicas
2. **How to configure cache using environment variables** and ConfigMaps
3. **How to use Downward API** for pod-aware configuration
4. **How multi-pod synchronization works** in production
5. **How to implement health checks** for cache readiness
6. **Production deployment patterns** for distributed caching

## Next Steps

After understanding Kubernetes deployment, explore:

- **[Basic Example](../basic/)** - Learn basic cache operations
- **[Custom Config](../custom-config/)** - Advanced configuration
- **[Debug Mode](../debug-mode/)** - Troubleshooting in production

## Troubleshooting

### Pods Not Synchronizing

**Check**: Redis connectivity
```bash
kubectl exec <pod-name> -- redis-cli -h redis ping
```

**Check**: Pub/sub subscription
```bash
kubectl logs <pod-name> | grep "Subscribed to invalidation channel"
```

### High Memory Usage

**Solution**: Adjust cache size in ConfigMap
```yaml
data:
  CACHE_LOCAL_MAX_COST: "536870912"  # 512MB
```

### Connection Refused

**Check**: Redis service
```bash
kubectl get svc redis
kubectl describe svc redis
```

## Related Documentation

- [Main README](../../README.md) - Complete library documentation
- [Configuration Guide](../../README.md#configuration) - All configuration options
- [Getting Started](../../GETTING_STARTED.md) - Basic setup

