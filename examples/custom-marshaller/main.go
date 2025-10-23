package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	dc "github.com/huykn/distributed-cache"
)

// User represents a sample user object.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UppercaseJSONMarshaller is a custom marshaller that converts
// all string values to uppercase before marshalling.
// This demonstrates how to implement the Marshaller interface.
//
// The Marshaller interface requires two methods:
//   - Marshal(v any) ([]byte, error)
//   - Unmarshal(data []byte, v any) error
type UppercaseJSONMarshaller struct{}

// Marshal serializes a value to JSON with uppercase strings.
func (um *UppercaseJSONMarshaller) Marshal(v any) ([]byte, error) {
	// First, marshal to JSON normally
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// Convert to uppercase (for demonstration purposes)
	// In a real implementation, you might use a different serialization format
	// like MessagePack, Protocol Buffers, or a custom binary format
	uppercased := strings.ToUpper(string(data))

	return []byte(uppercased), nil
}

// Unmarshal deserializes a value from uppercase JSON.
func (um *UppercaseJSONMarshaller) Unmarshal(data []byte, v any) error {
	// Convert back to lowercase before unmarshalling
	lowercased := strings.ToLower(string(data))

	return json.Unmarshal([]byte(lowercased), v)
}

// NewUppercaseJSONMarshaller creates a new uppercase JSON marshaller.
func NewUppercaseJSONMarshaller() dc.Marshaller {
	return &UppercaseJSONMarshaller{}
}

func main() {
	fmt.Println("========================================")
	fmt.Println("Custom Marshaller Implementation Example")
	fmt.Println("========================================")
	fmt.Println()

	// This example demonstrates how to implement and configure a custom
	// marshaller for the distributed cache.
	//
	// By default, the cache uses the standard JSON marshaller (encoding/json).
	// You can provide your own marshaller to:
	// - Use different serialization formats (MessagePack, Protocol Buffers, etc.)
	// - Add compression (gzip, snappy, etc.)
	// - Implement custom encoding/decoding logic
	// - Optimize for specific data types
	// - Add encryption/decryption
	//
	// Common use cases:
	// - MessagePack for smaller payload sizes
	// - Protocol Buffers for schema validation
	// - Custom binary formats for performance
	// - Compression for large objects

	fmt.Println("Custom Marshaller Overview:")
	fmt.Println("  Interface: Marshaller")
	fmt.Println("  Methods: Marshal(v any) ([]byte, error)")
	fmt.Println("           Unmarshal(data []byte, v any) error")
	fmt.Println()

	// Example 1: Default JSON Marshaller
	fmt.Println("=== Example 1: Default JSON Marshaller ===")
	cfg1 := dc.DefaultConfig()
	cfg1.PodID = "default-marshaller-pod"
	cfg1.RedisAddr = "localhost:6379"
	// cfg1.Marshaller is nil, so it will use default JSON marshaller

	cache1, err := dc.New(cfg1)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer cache1.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user1 := User{ID: 1, Name: "Alice", Email: "alice@example.com"}
	if err := cache1.Set(ctx, "user:1", user1); err != nil {
		log.Printf("Error: %v", err)
	}

	if value, found := cache1.Get(ctx, "user:1"); found {
		if retrieved1, ok := value.(User); ok {
			fmt.Printf("✓ Default marshaller: %+v\n", retrieved1)
		}
	} else {
		log.Println("User not found")
	}
	fmt.Println()

	// Example 2: Custom Uppercase JSON Marshaller
	fmt.Println("=== Example 2: Custom Uppercase JSON Marshaller ===")
	cfg2 := dc.DefaultConfig()
	cfg2.PodID = "custom-marshaller-pod"
	cfg2.RedisAddr = "localhost:6379"
	cfg2.Marshaller = NewUppercaseJSONMarshaller() // Use custom marshaller

	cache2, err := dc.New(cfg2)
	if err != nil {
		log.Fatalf("Failed to create cache: %v", err)
	}
	defer cache2.Close()

	user2 := User{ID: 2, Name: "Bob", Email: "bob@example.com"}
	if err := cache2.Set(ctx, "user:2", user2); err != nil {
		log.Printf("Error: %v", err)
	}

	if value, found := cache2.Get(ctx, "user:2"); found {
		if retrieved2, ok := value.(User); ok {
			fmt.Printf("✓ Custom marshaller: %+v\n", retrieved2)
		}
	} else {
		log.Println("User not found")
	}
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("Marshaller Implementation Guide:")
	fmt.Println()
	fmt.Println("1. Implement the Marshaller interface:")
	fmt.Println("   type MyMarshaller struct {}")
	fmt.Println("   func (m *MyMarshaller) Marshal(v any) ([]byte, error)")
	fmt.Println("   func (m *MyMarshaller) Unmarshal(data []byte, v any) error")
	fmt.Println()
	fmt.Println("2. Configure the cache:")
	fmt.Println("   cfg.Marshaller = NewMyMarshaller()")
	fmt.Println()
	fmt.Println("Popular Marshaller Options:")
	fmt.Println("  - JSON (default): Human-readable, widely supported")
	fmt.Println("  - MessagePack: Compact binary format, faster than JSON")
	fmt.Println("  - Protocol Buffers: Schema-based, efficient, type-safe")
	fmt.Println("  - CBOR: Compact binary, similar to MessagePack")
	fmt.Println("  - Custom: Optimized for specific use cases")
	fmt.Println()
	fmt.Println("Performance Considerations:")
	fmt.Println("  - Binary formats (MessagePack, Protobuf) are faster")
	fmt.Println("  - Compression reduces network/storage costs")
	fmt.Println("  - JSON is slower but easier to debug")
	fmt.Println("  - Consider payload size vs. CPU overhead")
	fmt.Println()
	fmt.Println("Example: MessagePack Marshaller")
	fmt.Println("  import \"github.com/vmihailenco/msgpack/v5\"")
	fmt.Println("  func (m *MsgPackMarshaller) Marshal(v any) ([]byte, error) {")
	fmt.Println("      return msgpack.Marshal(v)")
	fmt.Println("  }")
	fmt.Println("  func (m *MsgPackMarshaller) Unmarshal(data []byte, v any) error {")
	fmt.Println("      return msgpack.Unmarshal(data, v)")
	fmt.Println("  }")
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Example completed successfully!")
	fmt.Println("========================================")
}
