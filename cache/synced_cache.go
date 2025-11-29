package cache

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/huykn/distributed-cache/storage"
	cachesync "github.com/huykn/distributed-cache/sync"
)

// SyncedCache is a two-level cache with local and remote storage.
type SyncedCache struct {
	local        LocalCache
	store        Store
	synchronizer Synchronizer
	serializer   Marshaller
	logger       Logger
	options      Options
	closed       int32
	stats        Stats
	statsMutex   sync.RWMutex
}

// New creates a new SyncedCache instance.
func New(opts Options) (*SyncedCache, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Set defaults for optional fields
	if opts.LocalCacheFactory == nil {
		opts.LocalCacheFactory = NewLFUCacheFactory(opts.LocalCacheConfig)
	}
	if opts.Marshaller == nil {
		opts.Marshaller = NewJSONMarshaller()
	}
	if opts.Logger == nil {
		opts.Logger = NewNoOpLogger()
	}

	// Create local cache
	local, err := opts.LocalCacheFactory.Create()
	if err != nil {
		return nil, err
	}

	// Create Redis store
	store, err := storage.NewRedisStore(opts.RedisAddr, opts.RedisPassword, opts.RedisDB)
	if err != nil {
		local.Close()
		return nil, err
	}

	// Create synchronizer
	synchronizer := cachesync.NewPubSubSynchronizer(store.GetClient(), opts.InvalidationChannel, opts.PodID)

	sc := &SyncedCache{
		local:        local,
		store:        store,
		synchronizer: synchronizer,
		serializer:   opts.Marshaller,
		logger:       opts.Logger,
		options:      opts,
	}

	// Subscribe to invalidation events
	ctx, cancel := context.WithTimeout(context.Background(), opts.ContextTimeout)
	defer cancel()

	if err := synchronizer.Subscribe(ctx); err != nil {
		sc.Close()
		return nil, err
	}

	// Register invalidation callback
	synchronizer.OnInvalidate(sc.handleInvalidation)

	return sc, nil
}

// Get retrieves a value from the cache.
func (sc *SyncedCache) Get(ctx context.Context, key string) (any, bool) {
	if atomic.LoadInt32(&sc.closed) != 0 {
		return nil, false
	}

	if sc.options.DebugMode {
		sc.logger.Debug("Get: attempting to retrieve key", "key", key)
	}

	// Try local cache first
	value, found := sc.local.Get(key)
	if found {
		sc.recordLocalHit()
		if sc.options.DebugMode {
			sc.logger.Debug("Get: found in local cache", "key", key)
		}
		return value, true
	}

	sc.recordLocalMiss()
	if sc.options.DebugMode {
		sc.logger.Debug("Get: not found in local cache, checking remote", "key", key)
	}

	// Fallback to Redis
	data, err := sc.store.Get(ctx, key)
	if err != nil {
		sc.recordRemoteMiss()
		if sc.options.DebugMode {
			sc.logger.Debug("Get: not found in remote cache", "key", key, "error", err)
		}
		return nil, false
	}

	sc.recordRemoteHit()
	if sc.options.DebugMode {
		sc.logger.Debug("Get: found in remote cache", "key", key)
	}

	// Deserialize
	var result any
	if err := sc.serializer.Unmarshal(data, &result); err != nil {
		if sc.options.OnError != nil {
			sc.options.OnError(err)
		}
		if sc.options.DebugMode {
			sc.logger.Error("Get: deserialization failed", "key", key, "error", err)
		}
		return nil, false
	}

	// Populate local cache
	sc.local.Set(key, result, 1)
	if sc.options.DebugMode {
		sc.logger.Debug("Get: populated local cache", "key", key)
	}

	return result, true
}

// Set stores a value in the cache and propagates it to other pods.
// This is the default behavior - the value is sent to other pods so they can
// update their local caches without fetching from Redis.
func (sc *SyncedCache) Set(ctx context.Context, key string, value any) error {
	return sc.setInternal(ctx, key, value, false)
}

// SetWithInvalidate stores a value in the cache and invalidates it on other pods.
// Use this when you want other pods to fetch the value from Redis instead of
// receiving it directly (useful for large values or when you want lazy loading).
func (sc *SyncedCache) SetWithInvalidate(ctx context.Context, key string, value any) error {
	return sc.setInternal(ctx, key, value, true)
}

// setInternal is the internal implementation of Set operations.
func (sc *SyncedCache) setInternal(ctx context.Context, key string, value any, invalidateOnly bool) error {
	if atomic.LoadInt32(&sc.closed) != 0 {
		return ErrCacheClosed
	}

	if sc.options.DebugMode {
		sc.logger.Debug("Set: storing value", "key", key, "invalidateOnly", invalidateOnly)
	}

	// Set in local cache
	sc.local.Set(key, value, 1)
	if sc.options.DebugMode {
		sc.logger.Debug("Set: stored in local cache", "key", key)
	}

	// Serialize
	data, err := sc.serializer.Marshal(value)
	if err != nil {
		if sc.options.OnError != nil {
			sc.options.OnError(err)
		}
		if sc.options.DebugMode {
			sc.logger.Error("Set: serialization failed", "key", key, "error", err)
		}
		return err
	}

	// ReaderCanSetToRedis prevents reader nodes from overwriting data in Redis with potentially stale values
	if sc.options.ReaderCanSetToRedis {
		// Set in Redis
		if err := sc.store.Set(ctx, key, data); err != nil {
			if sc.options.OnError != nil {
				sc.options.OnError(err)
			}
			if sc.options.DebugMode {
				sc.logger.Error("Set: failed to store in remote cache", "key", key, "error", err)
			}
			return err
		}
	} else {
		if sc.options.DebugMode {
			sc.logger.Debug("Set: skipping Redis write (ReaderCanSetToRedis=false)", "key", key)
		}
	}

	if sc.options.DebugMode {
		sc.logger.Debug("Set: stored in remote cache", "key", key)
	}

	// Publish synchronization event
	var event InvalidationEvent
	if invalidateOnly {
		// Invalidate-only mode: other pods will delete the key from local cache
		event = InvalidationEvent{
			Key:    key,
			Sender: sc.options.PodID,
			Action: ActionInvalidate,
		}
	} else {
		// Propagation mode: other pods will update their local cache with the value
		event = InvalidationEvent{
			Key:    key,
			Sender: sc.options.PodID,
			Action: ActionSet,
			Value:  data,
		}
	}

	if err := sc.synchronizer.Publish(ctx, event); err != nil {
		if sc.options.OnError != nil {
			sc.options.OnError(err)
		}
		if sc.options.DebugMode {
			sc.logger.Warn("Set: failed to publish synchronization event", "key", key, "action", event.Action, "error", err)
		}
	} else if sc.options.DebugMode {
		sc.logger.Debug("Set: published synchronization event", "key", key, "action", event.Action)
	}

	return nil
}

// Delete removes a value from the cache.
func (sc *SyncedCache) Delete(ctx context.Context, key string) error {
	if atomic.LoadInt32(&sc.closed) != 0 {
		return ErrCacheClosed
	}

	if sc.options.DebugMode {
		sc.logger.Debug("Delete: removing key", "key", key)
	}

	// Delete from local cache
	sc.local.Delete(key)
	if sc.options.DebugMode {
		sc.logger.Debug("Delete: removed from local cache", "key", key)
	}

	// Delete from Redis
	if err := sc.store.Delete(ctx, key); err != nil {
		if sc.options.OnError != nil {
			sc.options.OnError(err)
		}
		if sc.options.DebugMode {
			sc.logger.Error("Delete: failed to remove from remote cache", "key", key, "error", err)
		}
		return err
	}

	if sc.options.DebugMode {
		sc.logger.Debug("Delete: removed from remote cache", "key", key)
	}

	// Publish delete event
	event := InvalidationEvent{
		Key:    key,
		Sender: sc.options.PodID,
		Action: ActionDelete,
	}
	if err := sc.synchronizer.Publish(ctx, event); err != nil {
		if sc.options.OnError != nil {
			sc.options.OnError(err)
		}
		if sc.options.DebugMode {
			sc.logger.Warn("Delete: failed to publish delete event", "key", key, "error", err)
		}
	} else if sc.options.DebugMode {
		sc.logger.Debug("Delete: published delete event", "key", key)
	}

	return nil
}

// Clear removes all values from the cache.
func (sc *SyncedCache) Clear(ctx context.Context) error {
	if atomic.LoadInt32(&sc.closed) != 0 {
		return ErrCacheClosed
	}

	if sc.options.DebugMode {
		sc.logger.Debug("Clear: clearing all cache entries")
	}

	// Clear local cache
	sc.local.Clear()
	if sc.options.DebugMode {
		sc.logger.Debug("Clear: cleared local cache")
	}

	// Clear Redis
	if err := sc.store.Clear(ctx); err != nil {
		if sc.options.OnError != nil {
			sc.options.OnError(err)
		}
		if sc.options.DebugMode {
			sc.logger.Error("Clear: failed to clear remote cache", "error", err)
		}
		return err
	}

	if sc.options.DebugMode {
		sc.logger.Debug("Clear: cleared remote cache")
	}

	// Publish clear event
	event := InvalidationEvent{
		Key:    "*",
		Sender: sc.options.PodID,
		Action: ActionClear,
	}
	if err := sc.synchronizer.Publish(ctx, event); err != nil {
		if sc.options.OnError != nil {
			sc.options.OnError(err)
		}
		if sc.options.DebugMode {
			sc.logger.Warn("Clear: failed to publish clear event", "error", err)
		}
	} else if sc.options.DebugMode {
		sc.logger.Debug("Clear: published clear event")
	}

	return nil
}

// Close closes the cache and releases all resources.
func (sc *SyncedCache) Close() error {
	if !atomic.CompareAndSwapInt32(&sc.closed, 0, 1) {
		return nil
	}

	var errs []error

	if err := sc.synchronizer.Close(); err != nil {
		errs = append(errs, err)
	}

	if err := sc.store.Close(); err != nil {
		errs = append(errs, err)
	}

	sc.local.Close()

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// Stats returns cache statistics.
func (sc *SyncedCache) Stats() Stats {
	sc.statsMutex.RLock()
	defer sc.statsMutex.RUnlock()
	return sc.stats
}

// handleInvalidation handles cache synchronization events.
func (sc *SyncedCache) handleInvalidation(event InvalidationEvent) {
	if sc.options.DebugMode {
		sc.logger.Info("Received synchronization event", "action", event.Action, "key", event.Key, "sender", event.Sender)
	}

	switch event.Action {
	case ActionSet:
		// Propagate the value to local cache
		if len(event.Value) > 0 {
			var value any
			if sc.options.OnSetLocalCache != nil {
				// Use custom callback to process and transform the event data
				value = sc.options.OnSetLocalCache(event)
				if sc.options.DebugMode {
					sc.logger.Debug("Sync: processed event via OnSetLocalCache callback", "key", event.Key, "sender", event.Sender)
				}
			} else {
				// Default behavior: unmarshal before storing
				if err := sc.serializer.Unmarshal(event.Value, &value); err != nil {
					if sc.options.OnError != nil {
						sc.options.OnError(err)
					}
					if sc.options.DebugMode {
						sc.logger.Error("Sync: failed to deserialize value", "key", event.Key, "error", err)
					}
					return
				}
				if sc.options.DebugMode {
					sc.logger.Debug("Sync: unmarshaled value for local cache", "key", event.Key, "sender", event.Sender)
				}
			}
			// Store the processed/unmarshaled value in local cache
			sc.local.Set(event.Key, value, 1)
			if sc.options.DebugMode {
				sc.logger.Debug("Sync: updated local cache", "key", event.Key, "sender", event.Sender)
			}
		}

	case ActionInvalidate, ActionDelete:
		// Remove from local cache
		sc.local.Delete(event.Key)
		atomic.AddInt64(&sc.stats.Invalidations, 1)
		if sc.options.DebugMode {
			sc.logger.Debug("Sync: deleted key from local cache", "key", event.Key, "action", event.Action, "sender", event.Sender)
		}

	case ActionClear:
		// Clear entire local cache
		sc.local.Clear()
		atomic.AddInt64(&sc.stats.Invalidations, 1)
		if sc.options.DebugMode {
			sc.logger.Debug("Sync: cleared local cache", "sender", event.Sender)
		}

	default:
		if sc.options.DebugMode {
			sc.logger.Warn("Sync: unknown action", "action", event.Action, "key", event.Key, "sender", event.Sender)
		}
	}
}

// recordLocalHit records a local cache hit.
func (sc *SyncedCache) recordLocalHit() {
	atomic.AddInt64(&sc.stats.LocalHits, 1)
}

// recordLocalMiss records a local cache miss.
func (sc *SyncedCache) recordLocalMiss() {
	atomic.AddInt64(&sc.stats.LocalMisses, 1)
}

// recordRemoteHit records a remote cache hit.
func (sc *SyncedCache) recordRemoteHit() {
	atomic.AddInt64(&sc.stats.RemoteHits, 1)
}

// recordRemoteMiss records a remote cache miss.
func (sc *SyncedCache) recordRemoteMiss() {
	atomic.AddInt64(&sc.stats.RemoteMisses, 1)
}

// ErrCacheClosed is returned when operations are performed on a closed cache.
var ErrCacheClosed = NewError("cache is closed")
