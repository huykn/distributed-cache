package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/huykn/heavy-read-api/shared"
	"github.com/redis/go-redis/v9"

	dc "github.com/huykn/distributed-cache"
	"github.com/huykn/distributed-cache/cache"
)

// CachedPost is a pre-processed wrapper struct stored in local cache.
// This eliminates all CPU-bound parsing operations from the request path.
type CachedPost struct {
	Hash         string      // ETag for HTTP 304 support
	Timestamp    int64       // Timestamp for latency metrics
	Data         []byte      // Raw JSON bytes to serve directly
	DataOriginal shared.Post // Original data for verification
	ReceiveTime  int64       // When the data was received via pub/sub
	Latency      string      // Time from publish to receive
}

var (
	dcache   cache.Cache
	ctx      = context.Background()
	hostname string
	podID    string
	rdclient *redis.Client
)

func main() {
	// Get configuration from environment
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	podID = getEnv("POD_ID", "reader-1")
	port := getEnv("PORT", "80")

	var err error
	hostname, err = os.Hostname()
	if err != nil {
		hostname = podID
	}

	// Initialize distributed cache
	cfg := dc.DefaultConfig()
	cfg.PodID = podID
	cfg.RedisAddr = redisAddr
	cfg.InvalidationChannel = shared.TopicName
	// cfg.DebugMode = true
	cfg.Logger = cache.NewConsoleLogger(podID)

	// Use OnSetLocalCache callback to parse and store pre-processed data
	// This eliminates all CPU-bound parsing operations from the request path
	cfg.OnSetLocalCache = func(event dc.InvalidationEvent) any {
		// Parse the event bytes once to extract hash, timestamp, and data
		post, err := shared.FromBytes(event.Value)
		if err != nil {
			log.Printf("Failed to parse post from event: %v", err)
			return nil
		}

		// Create a pre-processed wrapper struct with all extracted data
		return &CachedPost{
			Hash:         post.Hash,
			Timestamp:    post.Timestamp,
			Data:         event.Value, // Raw JSON bytes to serve directly
			DataOriginal: *post,
			ReceiveTime:  time.Now().UnixNano(),
			Latency:      time.Since(time.Unix(0, post.Timestamp)).String(),
		}
	}

	dcache, err = dc.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create distributed cache: %v", err)
	}
	defer dcache.Close()

	// Initialize Redis client
	rdclient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})
	if err := rdclient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Redis connection successful")

	// Setup HTTP handlers
	http.HandleFunc("/post", handleGetPost)
	http.HandleFunc("/post-redis", handleGetPostRedis)
	http.HandleFunc("/post-marshal", handleGetPostLocalAndMarshal)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/metrics", handleMetrics)

	log.Printf("Reader service %s (hostname: %s) started on port %s", podID, hostname, port)
	log.Printf("Connected to Redis at %s", redisAddr)
	log.Printf("Subscribed to channel: %s", shared.TopicName)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleGetPost(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Get post ID from query parameter
	postID := r.URL.Query().Get("id")
	if postID == "" {
		http.Error(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	// Get from distributed cache (local cache first, then Redis if needed)
	value, found := dcache.Get(ctx, postID)
	if !found {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// With OnSetLocalCache callback, the value is stored as *CachedPost
	// This eliminates all CPU-bound parsing operations from the request path
	cachedPost, ok := value.(*CachedPost)
	if !ok {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Check If-None-Match header for HTTP 304 support
	// Uses pre-extracted hash directly - no parsing needed
	// if match := r.Header.Get("If-None-Match"); match == cachedPost.Hash {
	// 	w.WriteHeader(http.StatusNotModified)
	// 	return
	// }

	// Set response headers using pre-extracted data - no parsing needed
	w.Header().Set("Content-Type", "application/json")
	// w.Header().Set("ETag", cachedPost.Hash)
	// w.Header().Set("X-Server-ID", hostname)
	// w.Header().Set("X-Pod-ID", podID)
	// if cachedPost.ReceiveTime > 0 {
	// 	w.Header().Set("X-Receive-Time", fmt.Sprintf("%d", cachedPost.ReceiveTime))
	// }
	w.Header().Set("X-Response-Time", fmt.Sprintf("%d", time.Since(startTime).Nanoseconds()))

	// Write raw JSON bytes directly - no re-marshaling needed
	w.Write(cachedPost.Data)
}

func handleGetPostRedis(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	// Get post ID from query parameter
	postID := r.URL.Query().Get("id")
	if postID == "" {
		http.Error(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	// Get from Redis directly
	value, err := rdclient.Get(ctx, postID).Result()
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", fmt.Sprintf("%d", time.Since(startTime).Nanoseconds()))
	w.Write([]byte(value))
}

func handleGetPostLocalAndMarshal(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	// Get post ID from query parameter
	postID := r.URL.Query().Get("id")
	if postID == "" {
		http.Error(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	// Get from distributed cache (local cache first, then Redis if needed)
	value, found := dcache.Get(ctx, postID)
	if !found {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// With OnSetLocalCache callback, the value is stored as *CachedPost
	// This eliminates all CPU-bound parsing operations from the request path
	cachedPost, ok := value.(*CachedPost)
	if !ok {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	// Re-marshal the pre-parsed data to JSON
	data, err := json.Marshal(cachedPost.DataOriginal)
	if err != nil {
		http.Error(w, "Failed to marshal post", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Response-Time", fmt.Sprintf("%d", time.Since(startTime).Nanoseconds()))
	w.Write(data)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "healthy",
		"role":     "reader",
		"hostname": hostname,
	})
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	stats := dcache.Stats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"hostname":    hostname,
		"cache_stats": stats,
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
