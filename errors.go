package distributedcache

import "errors"

// ErrNotFound is returned when a key is not found in the cache.
var ErrNotFound = errors.New("key not found")

// ErrCacheClosed is returned when operations are performed on a closed cache.
var ErrCacheClosed = errors.New("cache is closed")

// ErrInvalidConfig is returned when the cache configuration is invalid.
var ErrInvalidConfig = errors.New("invalid cache configuration")

// ErrSerializationFailed is returned when serialization fails.
var ErrSerializationFailed = errors.New("serialization failed")

// ErrDeserializationFailed is returned when deserialization fails.
var ErrDeserializationFailed = errors.New("deserialization failed")

// ErrRedisConnection is returned when Redis connection fails.
var ErrRedisConnection = errors.New("redis connection failed")

// ErrPubSubFailed is returned when pub/sub operations fail.
var ErrPubSubFailed = errors.New("pub/sub operation failed")
