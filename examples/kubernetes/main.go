package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/huykn/distributed-cache/cache"
)

var globalCache cache.Cache

// Product represents a sample product.
type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func init() {
	// Load configuration from environment
	cfg := FromEnv()

	// Use pod name as pod ID (set by Kubernetes downward API)
	if podName := os.Getenv("POD_NAME"); podName != "" {
		cfg.Cache.PodID = podName
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create cache instance
	var err error
	globalCache, err = cache.New(cfg.Cache)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}

	log.Printf("Cache initialized with pod ID: %s", cfg.Cache.PodID)
}

// getProduct retrieves a product from cache or database.
func getProduct(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Query().Get("id")
	if productID == "" {
		http.Error(w, "Missing product ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("product:%s", productID)

	// Try to get from cache
	value, found := globalCache.Get(ctx, key)
	if found {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		fmt.Fprintf(w, `{"product": %v, "cached": true}`, value)
		return
	}

	// Simulate database lookup
	product := Product{
		ID:    1,
		Name:  "Sample Product",
		Price: 99.99,
	}

	// Store in cache (value will be propagated to all pods)
	if err := globalCache.Set(ctx, key, product); err != nil {
		http.Error(w, fmt.Sprintf("Failed to cache product: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	fmt.Fprintf(w, `{"product": %v, "cached": false}`, product)
}

// updateProduct updates a product and propagates the new value to all pods.
func updateProduct(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Query().Get("id")
	if productID == "" {
		http.Error(w, "Missing product ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("product:%s", productID)

	// Simulate updating the product
	updatedProduct := Product{
		ID:    1,
		Name:  "Updated Product",
		Price: 149.99,
	}

	// Update in cache (value will be propagated to all pods)
	if err := globalCache.Set(ctx, key, updatedProduct); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update cache: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "updated", "key": "%s", "product": %v}`, key, updatedProduct)
}

// deleteProduct deletes a product and removes it from all pods.
func deleteProduct(w http.ResponseWriter, r *http.Request) {
	productID := r.URL.Query().Get("id")
	if productID == "" {
		http.Error(w, "Missing product ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("product:%s", productID)

	// Delete from cache (will be removed from all pods)
	if err := globalCache.Delete(ctx, key); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete from cache: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "deleted", "key": "%s"}`, key)
}

// stats returns cache statistics.
func stats(w http.ResponseWriter, r *http.Request) {
	stats := globalCache.Stats()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
		"local_hits": %d,
		"local_misses": %d,
		"remote_hits": %d,
		"remote_misses": %d,
		"invalidations": %d
	}`, stats.LocalHits, stats.LocalMisses, stats.RemoteHits, stats.RemoteMisses, stats.Invalidations)
}

// health returns health status.
func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "healthy"}`)
}

func main() {
	defer globalCache.Close()

	// Register HTTP handlers
	http.HandleFunc("/product", getProduct)
	http.HandleFunc("/product/update", updateProduct)
	http.HandleFunc("/product/delete", deleteProduct)
	http.HandleFunc("/stats", stats)
	http.HandleFunc("/health", health)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	log.Printf("Endpoints:")
	log.Printf("  GET  /product?id=<id>        - Get product (with cache)")
	log.Printf("  POST /product/update?id=<id> - Update product (propagates to all pods)")
	log.Printf("  POST /product/delete?id=<id> - Delete product (removes from all pods)")
	log.Printf("  GET  /stats                  - Cache statistics")
	log.Printf("  GET  /health                 - Health check")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
