package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/huykn/distributed-cache/cache"
	"github.com/huykn/distributed-cache/types"
	"github.com/redis/go-redis/v9"
)

// Default Redis configuration
const (
	DefaultRedisAddr     = "localhost:6379"
	DefaultRedisPassword = ""
	DefaultRedisDB       = 0
)

// SimulatedCache implements cache.Cache interface for in-process simulation.
// It mocks the behavior of a distributed system where multiple "pods" communicate via a shared "Global Bus" (simulating Redis Pub/Sub).
type SimulatedCache struct {
	podID               string
	local               cache.LocalCache
	globalBus           *GlobalBus
	onSetLocal          func(event types.InvalidationEvent) any
	stopCh              chan struct{}
	invalidationChannel string
	subCh               chan types.InvalidationEvent
}

// GlobalBus using Redis Pub/Sub and Storage for inter-pod communication.
type GlobalBus struct {
	subscribers map[string][]chan types.InvalidationEvent
	mu          sync.RWMutex
	rdb         *redis.Client
}

var (
	globalBusInstance *GlobalBus
	globalBusMu       sync.Mutex
)

// InitGlobalBus initializes the singleton GlobalBus with a Redis connection.
// Must be called before GetGlobalBus(). If addr is empty, uses DefaultRedisAddr.
func InitGlobalBus(addr string) error {
	globalBusMu.Lock()
	defer globalBusMu.Unlock()

	if globalBusInstance != nil {
		return nil // Already initialized
	}

	if addr == "" {
		addr = DefaultRedisAddr
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: DefaultRedisPassword,
		DB:       DefaultRedisDB,
		PoolSize: 100, // High pool size for concurrent access
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis at %s: %w", addr, err)
	}

	globalBusInstance = &GlobalBus{
		subscribers: make(map[string][]chan types.InvalidationEvent),
		rdb:         client,
	}

	log.Printf("[GlobalBus] Connected to Redis at %s", addr)
	return nil
}

// GetGlobalBus returns the singleton GlobalBus.
// Panics if InitGlobalBus() was not called first.
func GetGlobalBus() *GlobalBus {
	globalBusMu.Lock()
	defer globalBusMu.Unlock()

	if globalBusInstance == nil {
		log.Fatal("[GlobalBus] Not initialized. Call InitGlobalBus() first.")
	}
	return globalBusInstance
}

// CloseGlobalBus closes the Redis connection and resets the singleton.
func CloseGlobalBus() error {
	globalBusMu.Lock()
	defer globalBusMu.Unlock()

	if globalBusInstance != nil {
		err := globalBusInstance.rdb.Close()
		globalBusInstance = nil
		return err
	}
	return nil
}

// Publish sends an event to all subscribers of a channel (non-blocking).
func (b *GlobalBus) Publish(channel string, event types.InvalidationEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if subs, ok := b.subscribers[channel]; ok {
		for _, sub := range subs {
			select {
			case sub <- event:
			default:
				// Dropping event if receiver is full - avoids full traffic queue block
			}
		}
	}
}

// Subscribe creates a new subscription channel for a topic.
func (b *GlobalBus) Subscribe(channel string) chan types.InvalidationEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan types.InvalidationEvent, 100)
	b.subscribers[channel] = append(b.subscribers[channel], ch)
	return ch
}

// Unsubscribe removes a subscription channel from a topic.
func (b *GlobalBus) Unsubscribe(channel string, ch chan types.InvalidationEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if subs, ok := b.subscribers[channel]; ok {
		for i, sub := range subs {
			if sub == ch {
				b.subscribers[channel] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
}

// Set stores a value in Redis.
func (b *GlobalBus) Set(key string, value []byte) {
	ctx := context.Background()
	if err := b.rdb.Set(ctx, key, value, 0).Err(); err != nil {
		log.Printf("[GlobalBus] Redis SET error for key %s: %v", key, err)
	}
}

// BulkSet stores multiple values in Redis using pipelining.
func (b *GlobalBus) BulkSet(values *map[string][]byte) {
	ctx := context.Background()
	pipeline := b.rdb.Pipeline()
	for key, v := range *values {
		pipeline.Set(ctx, key, v, 0)
	}
	_, err := pipeline.Exec(ctx)
	if err != nil {
		log.Printf("[GlobalBus] Redis BULK SET error: %v", err)
	}
}

// Get retrieves a value from Redis.
func (b *GlobalBus) Get(key string) ([]byte, bool) {
	ctx := context.Background()
	val, err := b.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false
		}
		log.Printf("[GlobalBus] Redis GET error for key %s: %v", key, err)
		return nil, false
	}
	return val, true
}

// Delete removes a key from Redis.
func (b *GlobalBus) Delete(key string) {
	ctx := context.Background()
	if err := b.rdb.Del(ctx, key).Err(); err != nil {
		log.Printf("[GlobalBus] Redis DEL error for key %s: %v", key, err)
	}
}

// Keys returns all keys matching a given prefix from Redis using SCAN.
func (b *GlobalBus) Keys(prefix string) []string {
	ctx := context.Background()
	var keys []string
	var cursor uint64
	for {
		var batch []string
		var err error
		batch, cursor, err = b.rdb.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			log.Printf("[GlobalBus] Redis SCAN error for prefix %s: %v", prefix, err)
			return keys
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}
	return keys
}

// Clear removes all data from Redis using FLUSHDB.
func (b *GlobalBus) Clear() {
	ctx := context.Background()
	if err := b.rdb.FlushDB(ctx).Err(); err != nil {
		log.Printf("[GlobalBus] Redis FLUSHDB error: %v", err)
	}
}

// NewSimulatedCache creates a new simulated cache pod using distributed-cache library.
func NewSimulatedCache(podID string, invalidationChannel string, onSetLocal func(event types.InvalidationEvent) any) (*SimulatedCache, error) {
	localFactory := cache.NewLFUCacheFactory(cache.DefaultLocalCacheConfig())
	local, err := localFactory.Create()
	if err != nil {
		return nil, err
	}

	sc := &SimulatedCache{
		podID:      podID,
		local:      local,
		globalBus:  GetGlobalBus(),
		onSetLocal: onSetLocal,
		stopCh:     make(chan struct{}),
	}

	// Start listening to the global bus
	ch := sc.globalBus.Subscribe(invalidationChannel)
	sc.invalidationChannel = invalidationChannel
	sc.subCh = ch
	go sc.listenLoop(ch)

	return sc, nil
}

func (sc *SimulatedCache) listenLoop(ch chan types.InvalidationEvent) {
	for {
		select {
		case <-sc.stopCh:
			return
		case event := <-ch:
			if event.Sender == sc.podID {
				continue // Ignore self-sent events
			}

			switch event.Action {
			case types.Set:
				var val any
				if sc.onSetLocal != nil {
					val = sc.onSetLocal(event)
				} else {
					json.Unmarshal(event.Value, &val)
				}
				sc.local.Set(event.Key, val, 1)
			case types.Invalidate, types.Delete:
				sc.local.Delete(event.Key)
			case types.Clear:
				sc.local.Clear()
			}
		}
	}
}

// Get retrieves a value from local cache, then falls back to remote.
func (sc *SimulatedCache) Get(ctx context.Context, key string) (any, bool) {
	if val, ok := sc.local.Get(key); ok {
		return val, true
	}
	if data, ok := sc.globalBus.Get(key); ok {
		return data, true
	}
	return nil, false
}

// Set stores a value locally and broadcasts to other pods.
func (sc *SimulatedCache) Set(ctx context.Context, key string, value any) error {
	sc.local.Set(key, value, 1)
	data, _ := json.Marshal(value)
	sc.globalBus.Set(key, data)
	sc.globalBus.Publish(cache.DefaultOptions().InvalidationChannel, types.InvalidationEvent{
		Key: key, Value: data, Sender: sc.podID, Action: types.Set,
	})
	return nil
}

// SetWithInvalidate stores locally and sends invalidation (not value) to other pods.
func (sc *SimulatedCache) SetWithInvalidate(ctx context.Context, key string, value any) error {
	sc.local.Set(key, value, 1)
	data, _ := json.Marshal(value)
	sc.globalBus.Set(key, data)
	sc.globalBus.Publish(cache.DefaultOptions().InvalidationChannel, types.InvalidationEvent{
		Key: key, Sender: sc.podID, Action: types.Invalidate,
	})
	return nil
}

// Delete removes a value and broadcasts deletion.
func (sc *SimulatedCache) Delete(ctx context.Context, key string) error {
	sc.local.Delete(key)
	sc.globalBus.Publish(cache.DefaultOptions().InvalidationChannel, types.InvalidationEvent{
		Key: key, Sender: sc.podID, Action: types.Delete,
	})
	return nil
}

// Clear removes all values and broadcasts clear.
func (sc *SimulatedCache) Clear(ctx context.Context) error {
	sc.local.Clear()
	sc.globalBus.Publish(cache.DefaultOptions().InvalidationChannel, types.InvalidationEvent{
		Sender: sc.podID, Action: types.Clear,
	})
	return nil
}

// Close shuts down the cache.
func (sc *SimulatedCache) Close() error {
	close(sc.stopCh)
	if sc.invalidationChannel != "" && sc.subCh != nil {
		sc.globalBus.Unsubscribe(sc.invalidationChannel, sc.subCh)
	}
	sc.local.Close()
	return nil
}

// Stats returns cache statistics.
func (sc *SimulatedCache) Stats() cache.Stats {
	return cache.Stats{}
}
