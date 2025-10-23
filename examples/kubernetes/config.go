package main

import (
	"os"
	"strconv"
	"time"

	"github.com/huykn/distributed-cache/cache"
)

// Config represents the complete configuration for a distributed cache.
type Config struct {
	Cache cache.Options
}

// FromEnv loads configuration from environment variables.
func FromEnv() Config {
	cfg := Config{
		Cache: cache.DefaultOptions(),
	}

	// Pod ID
	if podID := os.Getenv("CACHE_POD_ID"); podID != "" {
		cfg.Cache.PodID = podID
	}

	// Redis configuration
	if redisAddr := os.Getenv("CACHE_REDIS_ADDR"); redisAddr != "" {
		cfg.Cache.RedisAddr = redisAddr
	}

	if redisPassword := os.Getenv("CACHE_REDIS_PASSWORD"); redisPassword != "" {
		cfg.Cache.RedisPassword = redisPassword
	}

	if redisDB := os.Getenv("CACHE_REDIS_DB"); redisDB != "" {
		if db, err := strconv.Atoi(redisDB); err == nil {
			cfg.Cache.RedisDB = db
		}
	}

	// Invalidation channel
	if channel := os.Getenv("CACHE_INVALIDATION_CHANNEL"); channel != "" {
		cfg.Cache.InvalidationChannel = channel
	}

	// Serialization format
	if format := os.Getenv("CACHE_SERIALIZATION_FORMAT"); format != "" {
		cfg.Cache.SerializationFormat = format
	}

	// Context timeout
	if timeout := os.Getenv("CACHE_CONTEXT_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.Cache.ContextTimeout = d
		}
	}

	// Enable metrics
	if metrics := os.Getenv("CACHE_ENABLE_METRICS"); metrics != "" {
		cfg.Cache.EnableMetrics = metrics == "true"
	}

	// Local cache configuration
	if numCounters := os.Getenv("CACHE_LOCAL_NUM_COUNTERS"); numCounters != "" {
		if n, err := strconv.ParseInt(numCounters, 10, 64); err == nil {
			cfg.Cache.LocalCacheConfig.NumCounters = n
		}
	}

	if maxCost := os.Getenv("CACHE_LOCAL_MAX_COST"); maxCost != "" {
		if n, err := strconv.ParseInt(maxCost, 10, 64); err == nil {
			cfg.Cache.LocalCacheConfig.MaxCost = n
		}
	}

	if bufferItems := os.Getenv("CACHE_LOCAL_BUFFER_ITEMS"); bufferItems != "" {
		if n, err := strconv.ParseInt(bufferItems, 10, 64); err == nil {
			cfg.Cache.LocalCacheConfig.BufferItems = n
		}
	}

	return cfg
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	return c.Cache.Validate()
}
