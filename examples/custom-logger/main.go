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

// ConsoleLogger is a custom logger implementation that prints to console.
// This demonstrates how to implement the Logger interface.
//
// The Logger interface requires four methods:
//   - Debug(msg string, args ...any)
//   - Info(msg string, args ...any)
//   - Warn(msg string, args ...any)
//   - Error(msg string, args ...any)
type CustomConsoleLogger struct {
	prefix string
}

// Debug logs a debug message to console.
func (cl *CustomConsoleLogger) Debug(msg string, args ...any) {
	fmt.Printf("[DEBUG] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

// Info logs an info message to console.
func (cl *CustomConsoleLogger) Info(msg string, args ...any) {
	fmt.Printf("[INFO] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

// Warn logs a warning message to console.
func (cl *CustomConsoleLogger) Warn(msg string, args ...any) {
	fmt.Printf("[WARN] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

// Error logs an error message to console.
func (cl *CustomConsoleLogger) Error(msg string, args ...any) {
	fmt.Printf("[ERROR] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

// NewConsoleLogger creates a new console logger.
func NewConsoleLogger(prefix string) dc.Logger {
	return &CustomConsoleLogger{prefix: prefix}
}

func main() {
	fmt.Println("========================================")
	fmt.Println("Custom Logger Implementation Example")
	fmt.Println("========================================")
	fmt.Println()

	// This example demonstrates how to implement and use a custom logger
	// with the distributed cache library.
	//
	// By default, the cache uses a no-op logger that doesn't output anything.
	// You can provide your own logger implementation to integrate with your
	// application's logging framework (e.g., logrus, zap, zerolog).

	fmt.Println("Creating cache with custom console logger...")
	fmt.Println()

	// Create configuration with custom logger
	cfg := dc.DefaultConfig()
	cfg.PodID = "custom-logger-pod"
	cfg.RedisAddr = "localhost:6379"
	cfg.Logger = NewConsoleLogger("DistributedCache")
	cfg.DebugMode = true // Enable debug logging to see logger in action

	// Initialize cache
	cache, err := dc.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	fmt.Println("✓ Cache initialized with custom logger")
	fmt.Println()
	fmt.Println("Performing cache operations (watch for log output)...")
	fmt.Println("----------------------------------------")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform operations that will trigger logging
	user := User{
		ID:    1,
		Name:  "Alice",
		Email: "alice@example.com",
	}

	// Set operation - will trigger debug logs
	if err := cache.Set(ctx, "user:1", user); err != nil {
		log.Printf("Error setting value: %v", err)
	}

	// Get operation - will trigger debug logs
	if _, found := cache.Get(ctx, "user:1"); !found {
		log.Println("User not found")
	}

	// Delete operation - will trigger debug logs
	if err := cache.Delete(ctx, "user:1"); err != nil {
		log.Printf("Error deleting value: %v", err)
	}

	fmt.Println("----------------------------------------")
	fmt.Println()
	fmt.Println("✓ Custom logger demonstrated successfully!")
	fmt.Println()
	fmt.Println("Integration Tips:")
	fmt.Println("  - Implement the Logger interface with your logging library")
	fmt.Println("  - Use structured logging for better observability")
	fmt.Println("  - Enable debug mode only in development/troubleshooting")
	fmt.Println("  - Consider log levels based on your monitoring needs")
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Example completed successfully!")
	fmt.Println("========================================")
}
