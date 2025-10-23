package main

import (
	"context"
	"fmt"
	"log"
	"time"

	dc "github.com/huykn/distributed-cache"
)

// User represents a sample user object.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ConsoleLogger is a simple logger that prints to console.
type ConsoleLogger struct {
	prefix string
}

func (cl *ConsoleLogger) Debug(msg string, args ...any) {
	fmt.Printf("[DEBUG] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

func (cl *ConsoleLogger) Info(msg string, args ...any) {
	fmt.Printf("[INFO] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

func (cl *ConsoleLogger) Warn(msg string, args ...any) {
	fmt.Printf("[WARN] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

func (cl *ConsoleLogger) Error(msg string, args ...any) {
	fmt.Printf("[ERROR] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

func NewConsoleLogger(prefix string) dc.Logger {
	return &ConsoleLogger{prefix: prefix}
}

func main() {
	fmt.Println("========================================")
	fmt.Println("Debug Mode Example")
	fmt.Println("========================================")
	fmt.Println()

	// This example demonstrates debug mode with detailed logging output.
	//
	// Debug mode enables comprehensive logging throughout the cache lifecycle:
	// - Get operations: local cache checks, remote fetches, deserialization
	// - Set operations: serialization, local cache updates, remote storage
	// - Delete operations: local cache invalidation, remote deletion
	// - Clear operations: full cache clearing
	//
	// Use debug mode for:
	// - Development and testing
	// - Troubleshooting cache issues
	// - Understanding cache behavior
	// - Performance analysis
	//
	// WARNING: Debug mode can generate significant log volume in production!

	fmt.Println("Creating cache with debug mode enabled...")
	fmt.Println()

	cfg := dc.DefaultConfig()
	cfg.PodID = "debug-mode-pod"
	cfg.RedisAddr = "localhost:6379"
	cfg.Logger = NewConsoleLogger("DebugMode")
	cfg.DebugMode = true // Enable debug logging

	cache, err := dc.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	fmt.Println("✓ Cache initialized with debug mode enabled")
	fmt.Println()
	fmt.Println("Performing cache operations with debug logging...")
	fmt.Println("========================================")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Example 1: Set operation (with value propagation)
	fmt.Println("\n--- Example 1: Set Operation (Value Propagation) ---")
	user1 := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
	if err := cache.Set(ctx, "user:1", user1); err != nil {
		log.Printf("Error: %v", err)
	}

	// Example 2: Get operation (cache hit)
	fmt.Println("\n--- Example 2: Get Operation (Local Cache Hit) ---")
	if value, found := cache.Get(ctx, "user:1"); found {
		if user, ok := value.(User); ok {
			fmt.Printf("Retrieved: %+v\n", user)
		}
	} else {
		log.Println("User not found")
	}

	// Example 2b: SetWithInvalidate operation
	fmt.Println("\n--- Example 2b: SetWithInvalidate Operation (Invalidate-Only) ---")
	user2 := User{ID: 2, Name: "Bob", Email: "bob@example.com"}
	if err := cache.SetWithInvalidate(ctx, "user:2", user2); err != nil {
		log.Printf("Error: %v", err)
	}

	// Example 3: Get operation (cache miss)
	fmt.Println("\n--- Example 3: Get Operation (Cache Miss) ---")
	if _, found := cache.Get(ctx, "user:999"); !found {
		fmt.Println("Expected miss: user not found")
	}

	// Example 4: Delete operation
	fmt.Println("\n--- Example 4: Delete Operation ---")
	if err := cache.Delete(ctx, "user:1"); err != nil {
		log.Printf("Error: %v", err)
	}

	// Example 5: Set multiple values
	fmt.Println("\n--- Example 5: Set Multiple Values ---")
	for i := 1; i <= 3; i++ {
		user := User{
			ID:    i,
			Name:  fmt.Sprintf("User%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
		}
		if err := cache.Set(ctx, fmt.Sprintf("user:%d", i), user); err != nil {
			log.Printf("Error: %v", err)
		}
	}

	// Example 6: Clear operation
	fmt.Println("\n--- Example 6: Clear Operation ---")
	if err := cache.Clear(ctx); err != nil {
		log.Printf("Error: %v", err)
	}

	fmt.Println("\n========================================")
	fmt.Println()
	fmt.Println("Debug Mode Output Explained:")
	fmt.Println("  - Shows cache operation flow (local → remote)")
	fmt.Println("  - Displays serialization/deserialization steps")
	fmt.Println("  - Tracks cache hits and misses")
	fmt.Println("  - Reveals synchronization events (set/invalidate/delete)")
	fmt.Println("  - Shows value propagation vs invalidation-only modes")
	fmt.Println("  - Helps identify performance bottlenecks")
	fmt.Println()
	fmt.Println("Best Practices:")
	fmt.Println("  - Enable debug mode only when needed")
	fmt.Println("  - Use structured logging in production")
	fmt.Println("  - Monitor log volume to avoid storage issues")
	fmt.Println("  - Disable debug mode in high-traffic environments")
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Example completed successfully!")
	fmt.Println("========================================")
}
