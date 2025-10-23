package sync

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/huykn/distributed-cache/types"
	"github.com/redis/go-redis/v9"
)

// InvalidationEvent is an alias for types.InvalidationEvent
type InvalidationEvent = types.InvalidationEvent

// PubSubSynchronizer implements cache synchronization using Redis Pub/Sub.
type PubSubSynchronizer struct {
	client         *redis.Client
	channel        string
	podID          string
	pubsub         *redis.PubSub
	callbacks      []func(event InvalidationEvent)
	callbacksMutex sync.RWMutex
	done           chan struct{}
	wg             sync.WaitGroup
}

// NewPubSubSynchronizer creates a new Pub/Sub synchronizer.
func NewPubSubSynchronizer(client *redis.Client, channel, podID string) *PubSubSynchronizer {
	return &PubSubSynchronizer{
		client:    client,
		channel:   channel,
		podID:     podID,
		callbacks: make([]func(event InvalidationEvent), 0),
		done:      make(chan struct{}),
	}
}

// Subscribe starts listening for invalidation events.
func (ps *PubSubSynchronizer) Subscribe(ctx context.Context) error {
	ps.pubsub = ps.client.Subscribe(ctx, ps.channel)

	ps.wg.Add(1)
	go ps.listenForEvents()

	return nil
}

// Publish publishes an invalidation event.
func (ps *PubSubSynchronizer) Publish(ctx context.Context, event InvalidationEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return ps.client.Publish(ctx, ps.channel, string(data)).Err()
}

// OnInvalidate registers a callback for invalidation events.
func (ps *PubSubSynchronizer) OnInvalidate(callback func(event InvalidationEvent)) {
	ps.callbacksMutex.Lock()
	defer ps.callbacksMutex.Unlock()
	ps.callbacks = append(ps.callbacks, callback)
}

// Close closes the synchronizer.
func (ps *PubSubSynchronizer) Close() error {
	close(ps.done)
	ps.wg.Wait()

	if ps.pubsub != nil {
		return ps.pubsub.Close()
	}
	return nil
}

// listenForEvents listens for invalidation events from Redis Pub/Sub.
func (ps *PubSubSynchronizer) listenForEvents() {
	defer ps.wg.Done()

	if ps.pubsub == nil {
		return
	}

	ch := ps.pubsub.Channel()

	for {
		select {
		case <-ps.done:
			return
		case msg := <-ch:
			if msg == nil {
				return
			}

			var event InvalidationEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				continue
			}

			// Don't invalidate your own writes
			if event.Sender == ps.podID {
				continue
			}

			ps.callbacksMutex.RLock()
			callbacks := ps.callbacks
			ps.callbacksMutex.RUnlock()

			for _, callback := range callbacks {
				callback(event)
			}
		}
	}
}
