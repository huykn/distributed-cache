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

	dc "github.com/huykn/distributed-cache"
	"github.com/huykn/distributed-cache/cache"
)

var (
	dcache cache.Cache
	ctx    = context.Background()
)

func main() {
	// Get configuration from environment
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	podID := getEnv("POD_ID", "writer-1")
	port := getEnv("PORT", "8080")

	// Initialize distributed cache
	cfg := dc.DefaultConfig()
	cfg.PodID = podID
	cfg.RedisAddr = redisAddr
	cfg.InvalidationChannel = shared.TopicName
	cfg.DebugMode = true
	cfg.Logger = cache.NewConsoleLogger(podID)

	var err error
	dcache, err = dc.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create distributed cache: %v", err)
	}
	defer dcache.Close()

	// Setup HTTP handlers
	http.HandleFunc("/create", handleCreatePost)
	http.HandleFunc("/health", handleHealth)

	log.Printf("Writer service %s started on port %s", podID, port)
	log.Printf("Connected to Redis at %s", redisAddr)
	log.Printf("Publishing to channel: %s", shared.TopicName)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleCreatePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Content string `json:"content"`
		Author  string `json:"author"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Create post with hash
	post := shared.NewPost(req.ID, req.Title, req.Content, req.Author)

	// Record publish time for latency measurement
	publishTime := time.Now()

	// Store in distributed cache - this will propagate to all reader instances
	// We use Set() which sends the full value to other pods via pub/sub
	// The cache will handle serialization internally
	if err := dcache.Set(ctx, post.ID, post); err != nil {
		http.Error(w, fmt.Sprintf("Cache set failed: %v", err), http.StatusInternalServerError)
		return
	}

	publishDuration := time.Since(publishTime)

	// Return success response with metrics
	response := map[string]any{
		"status":           "success",
		"post_id":          post.ID,
		"hash":             post.Hash,
		"publish_time_ns":  publishTime.UnixNano(),
		"publish_duration": publishDuration.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("Published post %s (hash: %s) in %v", post.ID, post.Hash, publishDuration)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"role":   "writer",
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
