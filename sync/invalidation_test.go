package sync

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/huykn/distributed-cache/types"
)

func setupRedisClient(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use DB 1 for tests
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Clear the database
	client.FlushDB(ctx)

	return client
}

func TestNewPubSubSynchronizer(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	sync := NewPubSubSynchronizer(client, "test-channel", "pod-1")
	if sync == nil {
		t.Fatal("Synchronizer should not be nil")
	}

	if sync.channel != "test-channel" {
		t.Fatalf("Expected channel 'test-channel', got %s", sync.channel)
	}

	if sync.podID != "pod-1" {
		t.Fatalf("Expected podID 'pod-1', got %s", sync.podID)
	}
}

func TestPubSubSynchronizerSubscribe(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	sync := NewPubSubSynchronizer(client, "test-channel", "pod-1")
	defer sync.Close()

	ctx := context.Background()
	err := sync.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Give it a moment to subscribe
	time.Sleep(100 * time.Millisecond)
}

func TestPubSubSynchronizerPublish(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	sync := NewPubSubSynchronizer(client, "test-channel", "pod-1")
	defer sync.Close()

	ctx := context.Background()
	event := InvalidationEvent{
		Key:    "test-key",
		Sender: "pod-1",
		Action: types.Set,
		Value:  []byte("test-value"),
	}

	err := sync.Publish(ctx, event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
}

func TestPubSubSynchronizerOnInvalidate(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	sync := NewPubSubSynchronizer(client, "test-channel", "pod-1")
	defer sync.Close()

	called := false
	sync.OnInvalidate(func(event InvalidationEvent) {
		called = true
	})

	if called {
		t.Fatal("Callback should not be called yet")
	}
}

func TestPubSubSynchronizerPublishAndReceive(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	// Create two synchronizers for different pods
	sync1 := NewPubSubSynchronizer(client, "test-channel-2", "pod-1")
	defer sync1.Close()

	sync2 := NewPubSubSynchronizer(client, "test-channel-2", "pod-2")
	defer sync2.Close()

	// Subscribe both
	ctx := context.Background()
	sync1.Subscribe(ctx)
	sync2.Subscribe(ctx)

	// Give them time to subscribe
	time.Sleep(100 * time.Millisecond)

	// Set up callback for sync2
	received := make(chan InvalidationEvent, 1)
	sync2.OnInvalidate(func(event InvalidationEvent) {
		received <- event
	})

	// Publish from sync1
	event := InvalidationEvent{
		Key:    "test-key",
		Sender: "pod-1",
		Action: types.Set,
		Value:  []byte("test-value"),
	}

	err := sync1.Publish(ctx, event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for event
	select {
	case receivedEvent := <-received:
		if receivedEvent.Key != "test-key" {
			t.Fatalf("Expected key 'test-key', got %s", receivedEvent.Key)
		}
		if receivedEvent.Sender != "pod-1" {
			t.Fatalf("Expected sender 'pod-1', got %s", receivedEvent.Sender)
		}
		if receivedEvent.Action != types.Set {
			t.Fatalf("Expected action 'set', got %s", receivedEvent.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestPubSubSynchronizerIgnoreOwnEvents(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	sync := NewPubSubSynchronizer(client, "test-channel-3", "pod-1")
	defer sync.Close()

	ctx := context.Background()
	sync.Subscribe(ctx)

	// Give it time to subscribe
	time.Sleep(100 * time.Millisecond)

	// Set up callback
	received := make(chan InvalidationEvent, 1)
	sync.OnInvalidate(func(event InvalidationEvent) {
		received <- event
	})

	// Publish from same pod
	event := InvalidationEvent{
		Key:    "test-key",
		Sender: "pod-1", // Same as sync's podID
		Action: types.Set,
		Value:  []byte("test-value"),
	}

	err := sync.Publish(ctx, event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Should NOT receive the event (own event)
	select {
	case <-received:
		t.Fatal("Should not receive own events")
	case <-time.After(500 * time.Millisecond):
		// Expected - no event received
	}
}

func TestPubSubSynchronizerMultipleCallbacks(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	sync1 := NewPubSubSynchronizer(client, "test-channel-4", "pod-1")
	defer sync1.Close()

	sync2 := NewPubSubSynchronizer(client, "test-channel-4", "pod-2")
	defer sync2.Close()

	ctx := context.Background()
	sync1.Subscribe(ctx)
	sync2.Subscribe(ctx)

	time.Sleep(100 * time.Millisecond)

	// Register multiple callbacks
	received1 := make(chan InvalidationEvent, 1)
	received2 := make(chan InvalidationEvent, 1)

	sync2.OnInvalidate(func(event InvalidationEvent) {
		received1 <- event
	})

	sync2.OnInvalidate(func(event InvalidationEvent) {
		received2 <- event
	})

	// Publish event
	event := InvalidationEvent{
		Key:    "test-key",
		Sender: "pod-1",
		Action: types.Delete,
	}

	err := sync1.Publish(ctx, event)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Both callbacks should be called
	timeout := time.After(2 * time.Second)
	count := 0

	for count < 2 {
		select {
		case <-received1:
			count++
		case <-received2:
			count++
		case <-timeout:
			t.Fatalf("Expected 2 callbacks, got %d", count)
		}
	}
}

func TestPubSubSynchronizerClose(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	sync := NewPubSubSynchronizer(client, "test-channel-5", "pod-1")

	ctx := context.Background()
	sync.Subscribe(ctx)

	time.Sleep(100 * time.Millisecond)

	err := sync.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestPubSubSynchronizerCloseWithoutSubscribe(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	sync := NewPubSubSynchronizer(client, "test-channel-6", "pod-1")

	err := sync.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestPubSubSynchronizerInvalidateAction(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	sync1 := NewPubSubSynchronizer(client, "test-channel-7", "pod-1")
	defer sync1.Close()

	sync2 := NewPubSubSynchronizer(client, "test-channel-7", "pod-2")
	defer sync2.Close()

	ctx := context.Background()
	sync1.Subscribe(ctx)
	sync2.Subscribe(ctx)

	time.Sleep(100 * time.Millisecond)

	received := make(chan InvalidationEvent, 1)
	sync2.OnInvalidate(func(event InvalidationEvent) {
		received <- event
	})

	event := InvalidationEvent{
		Key:    "test-key",
		Sender: "pod-1",
		Action: types.Invalidate,
	}

	sync1.Publish(ctx, event)

	select {
	case receivedEvent := <-received:
		if receivedEvent.Action != types.Invalidate {
			t.Fatalf("Expected action 'invalidate', got %s", receivedEvent.Action)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestPubSubSynchronizerClearAction(t *testing.T) {
	client := setupRedisClient(t)
	defer client.Close()

	sync1 := NewPubSubSynchronizer(client, "test-channel-8", "pod-1")
	defer sync1.Close()

	sync2 := NewPubSubSynchronizer(client, "test-channel-8", "pod-2")
	defer sync2.Close()

	ctx := context.Background()
	sync1.Subscribe(ctx)
	sync2.Subscribe(ctx)

	time.Sleep(100 * time.Millisecond)

	received := make(chan InvalidationEvent, 1)
	sync2.OnInvalidate(func(event InvalidationEvent) {
		received <- event
	})

	event := InvalidationEvent{
		Key:    "*",
		Sender: "pod-1",
		Action: types.Clear,
	}

	sync1.Publish(ctx, event)

	select {
	case receivedEvent := <-received:
		if receivedEvent.Action != types.Clear {
			t.Fatalf("Expected action 'clear', got %s", receivedEvent.Action)
		}
		if receivedEvent.Key != "*" {
			t.Fatalf("Expected key '*', got %s", receivedEvent.Key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}
