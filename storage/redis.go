package storage

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements the Store interface using Redis.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new Redis-based store.
func NewRedisStore(addr, password string, db int) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*1000*1000*1000) // 5 seconds
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisStore{
		client: client,
	}, nil
}

// Get retrieves a value from Redis.
func (rs *RedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := rs.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return val, nil
}

// Set stores a value in Redis.
func (rs *RedisStore) Set(ctx context.Context, key string, value []byte) error {
	return rs.client.Set(ctx, key, value, 0).Err()
}

// Delete removes a value from Redis.
func (rs *RedisStore) Delete(ctx context.Context, key string) error {
	return rs.client.Del(ctx, key).Err()
}

// Clear removes all values from Redis.
func (rs *RedisStore) Clear(ctx context.Context) error {
	return rs.client.FlushDB(ctx).Err()
}

// Close closes the Redis connection.
func (rs *RedisStore) Close() error {
	return rs.client.Close()
}

// GetClient returns the underlying Redis client.
func (rs *RedisStore) GetClient() *redis.Client {
	return rs.client
}

// ErrNotFound is returned when a key is not found.
var ErrNotFound = errors.New("key not found in redis")
