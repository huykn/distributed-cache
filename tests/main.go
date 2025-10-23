package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/redis/go-redis/v9"
)

// InvalidationMessage defines the structure of the message sent over Redis Pub/Sub.
// Using JSON makes it extensible for future actions (e.g., "update").
type InvalidationMessage struct {
	Action string `json:"action"` // "invalidate"
	Key    string `json:"key"`
}

// CacheManager manages a local LRU cache and synchronizes it via Redis Pub/Sub.
type CacheManager struct {
	localCache    *lru.Cache[string, any]
	redisClient   *redis.Client
	pubsubChannel string
	subscription  *redis.PubSub
}

// NewCacheManager creates a new CacheManager.
func NewCacheManager(ctx context.Context, cacheSize int, redisAddr, pubsubChannel string) (*CacheManager, error) {
	// 1. Initialize the local LRU cache
	cache, err := lru.New[string, any](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("could not create LRU cache: %w", err)
	}

	// 2. Initialize the Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr, // e.g., "localhost:6379" or a k8s service name like "redis:6379"
	})

	// 3. Test the Redis connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("could not connect to Redis: %w", err)
	}

	cm := &CacheManager{
		localCache:    cache,
		redisClient:   rdb,
		pubsubChannel: pubsubChannel,
	}

	// 4. Start the background listener for invalidation messages
	cm.startInvalidationListener(ctx)

	log.Printf("CacheManager initialized. Listening for invalidations on channel '%s'", pubsubChannel)
	return cm, nil
}

// Get retrieves an item from the cache.
// On a cache miss, it calls fetchFunc to get the data, stores it, and publishes an invalidation.
func (cm *CacheManager) Get(ctx context.Context, key string, fetchFunc func() (interface{}, error)) (interface{}, error) {
	// Check local cache first
	if value, ok := cm.localCache.Get(key); ok {
		log.Printf("CACHE HIT for key: %s", key)
		return value, nil
	}

	log.Printf("CACHE MISS for key: %s. Fetching from source...", key)

	// Cache miss: fetch data using the provided function
	value, err := fetchFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data for key %s: %w", key, err)
	}

	// Populate the local cache
	cm.localCache.Add(key, value)

	// Publish invalidation message to other pods/services
	msg := InvalidationMessage{Action: "invalidate", Key: key}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		// Log error but don't fail the operation, as the primary request succeeded.
		log.Printf("ERROR: could not marshal invalidation message for key %s: %v", key, err)
		return value, nil
	}

	// Use a separate context with timeout for publishing to avoid blocking the main request
	pubCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	if err := cm.redisClient.Publish(pubCtx, cm.pubsubChannel, msgBytes).Err(); err != nil {
		// Log error but don't fail the operation.
		log.Printf("ERROR: could not publish invalidation for key %s: %v", key, err)
	}

	log.Printf("FETCHED and CACHED key: %s. Published invalidation.", key)
	return value, nil
}

// startInvalidationListener runs in a goroutine to listen for Redis Pub/Sub messages.
func (cm *CacheManager) startInvalidationListener(ctx context.Context) {
	cm.subscription = cm.redisClient.Subscribe(ctx, cm.pubsubChannel)

	go func() {
		ch := cm.subscription.Channel()
		for {
			select {
			case msg := <-ch:
				var invalidationMsg InvalidationMessage
				if err := json.Unmarshal([]byte(msg.Payload), &invalidationMsg); err != nil {
					log.Printf("ERROR: could not unmarshal invalidation message: %v", err)
					continue
				}
				if invalidationMsg.Action == "invalidate" {
					log.Printf("Received invalidation for key: %s. Removing from local cache.", invalidationMsg.Key)
					cm.localCache.Remove(invalidationMsg.Key)
				}
			case <-ctx.Done():
				// Context cancelled, shut down the listener
				log.Println("Invalidation listener shutting down.")
				return
			}
		}
	}()
}

// Close gracefully shuts down the cache manager.
func (cm *CacheManager) Close() error {
	if cm.subscription != nil {
		return cm.subscription.Close()
	}
	return nil
}

// --- Simulation Logic ---

// simulateDatabase is a mock function to represent fetching data from a primary source.
var simulateDatabase = func(key string) (string, error) {
	log.Printf("  [DATABASE] Fetching data for %s from slow database...", key)
	time.Sleep(150 * time.Millisecond) // Simulate network latency
	return fmt.Sprintf("data-for-%s-at-%d", key, time.Now().Unix()), nil
}

func main() {
	ctx := context.Background()

	// --- Configuration ---
	// In a real K8s environment, this would be the service name for Redis, e.g., "redis.default.svc.cluster.local:6379"
	redisAddr := "localhost:6379"
	pubsubChannel := "cache-invalidation-channel"

	// --- Simulate Pod A ---
	log.Println("--- Initializing Pod A ---")
	cacheA, err := NewCacheManager(ctx, 100, redisAddr, pubsubChannel)
	if err != nil {
		log.Fatalf("Failed to create cache manager for Pod A: %v", err)
	}
	defer cacheA.Close()

	// --- Simulate Pod B ---
	log.Println("\n--- Initializing Pod B ---")
	cacheB, err := NewCacheManager(ctx, 100, redisAddr, pubsubChannel)
	if err != nil {
		log.Fatalf("Failed to create cache manager for Pod B: %v", err)
	}
	defer cacheB.Close()

	// Give a moment for subscriptions to be active
	time.Sleep(100 * time.Millisecond)

	// --- Scenario 1: Pod A fetches, then Pod B fetches ---
	log.Println("\n--- SCENARIO 1: Pod A fetches, then Pod B fetches ---")
	key1 := "user:123"
	valA, _ := cacheA.Get(ctx, key1, func() (interface{}, error) { return simulateDatabase(key1) })
	log.Printf("Pod A got value: %s\n", valA)

	// Pod B now gets the same key. It should be a miss and fetch from DB.
	valB, _ := cacheB.Get(ctx, key1, func() (interface{}, error) { return simulateDatabase(key1) })
	log.Printf("Pod B got value: %s\n", valB)

	// --- Scenario 2: Pod A fetches again (cache hit), then updates, invalidating Pod B ---
	log.Println("\n--- SCENARIO 2: Pod A hits cache, then updates data ---")
	// Pod A hits its local cache
	valA, _ = cacheA.Get(ctx, key1, func() (interface{}, error) { return simulateDatabase(key1) })
	log.Printf("Pod A got value (from cache): %s\n", valA)

	// Now, let's say the data for user:123 is updated. We simulate this by forcing a refresh.
	// In a real app, this would happen after a POST/PUT request.
	log.Println("Data for user:123 was updated in the database. Simulating a refresh...")
	cacheA.localCache.Remove(key1) // Simulate a write-through or refresh action

	// Pod A fetches the new data, which will publish an invalidation
	newValA, _ := cacheA.Get(ctx, key1, func() (interface{}, error) { return simulateDatabase(key1) })
	log.Printf("Pod A got NEW value: %s\n", newValA)

	// Give a moment for the invalidation message to travel
	time.Sleep(50 * time.Millisecond)

	// Pod B now tries to get the same key. Its cache should have been invalidated.
	log.Println("Pod B now requests the same key. Its cache should be invalidated.")
	valB, _ = cacheB.Get(ctx, key1, func() (interface{}, error) { return simulateDatabase(key1) })
	log.Printf("Pod B got value (after invalidation): %s\n", valB)
}
