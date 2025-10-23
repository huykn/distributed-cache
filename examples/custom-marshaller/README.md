# Custom Marshaller Example

This example demonstrates how to implement custom serialization/deserialization for the distributed-cache library, allowing you to use different formats like MessagePack, Protocol Buffers, or custom binary formats.

## What This Example Demonstrates

- **Custom Marshaller interface implementation** for serialization
- **Default JSON marshaller** usage
- **Custom uppercase JSON marshaller** as an example
- **Integration with distributed-cache** serialization system
- **Use cases for custom marshallers** (compression, encryption, binary formats)

## Key Concepts

### Marshaller Interface

The `Marshaller` interface requires implementing two methods:

```go
type Marshaller interface {
    Marshal(v any) ([]byte, error)
    Unmarshal(data []byte, v any) error
}
```

### Default JSON Marshaller

The library uses JSON by default:

```go
type JSONMarshaller struct{}

func (jm *JSONMarshaller) Marshal(v any) ([]byte, error) {
    return json.Marshal(v)
}

func (jm *JSONMarshaller) Unmarshal(data []byte, v any) error {
    return json.Unmarshal(data, v)
}
```

## Prerequisites

- Go 1.24 or later
- Redis server running on `localhost:6379`

## How to Run

### 1. Start Redis

```bash
# Using Redis directly
redis-server

# Or using Docker
docker run -d -p 6379:6379 redis:latest
```

### 2. Run the Example

```bash
cd examples/custom-marshaller
go run main.go
```

## Expected Output

```
========================================
Custom Marshaller Example
========================================

Marshaller Overview:
  Default: JSON (encoding/json)
  Custom: Uppercase JSON (example)
  Supported: MessagePack, Protobuf, CBOR, etc.

=== Example 1: Default JSON Marshaller ===
✓ Default marshaller: {ID:1 Name:Alice Email:alice@example.com}

=== Example 2: Custom Uppercase JSON Marshaller ===
✓ Custom marshaller: {ID:2 Name:Bob Email:bob@example.com}

========================================
Custom Marshaller Use Cases:
  - MessagePack: Smaller payload sizes
  - Protocol Buffers: Schema validation
  - CBOR: Compact binary, similar to MessagePack
  - Custom: Optimized for specific use cases

Performance Considerations:
  - Binary formats (MessagePack, Protobuf) are faster
  - Compression reduces network/storage costs
  - JSON is slower but easier to debug
  - Consider payload size vs. CPU overhead
========================================
```

## Custom Marshaller Examples

### 1. MessagePack Marshaller

MessagePack provides efficient binary serialization:

```go
import "github.com/vmihailenco/msgpack/v5"

type MessagePackMarshaller struct{}

func (m *MessagePackMarshaller) Marshal(v any) ([]byte, error) {
    return msgpack.Marshal(v)
}

func (m *MessagePackMarshaller) Unmarshal(data []byte, v any) error {
    return msgpack.Unmarshal(data, v)
}

// Usage
cfg.Marshaller = &MessagePackMarshaller{}
```

### 2. Protocol Buffers Marshaller

For schema-based serialization:

```go
import "google.golang.org/protobuf/proto"

type ProtobufMarshaller struct{}

func (m *ProtobufMarshaller) Marshal(v any) ([]byte, error) {
    msg, ok := v.(proto.Message)
    if !ok {
        return nil, errors.New("value must be a proto.Message")
    }
    return proto.Marshal(msg)
}

func (m *ProtobufMarshaller) Unmarshal(data []byte, v any) error {
    msg, ok := v.(proto.Message)
    if !ok {
        return errors.New("value must be a proto.Message")
    }
    return proto.Unmarshal(data, msg)
}

// Usage
cfg.Marshaller = &ProtobufMarshaller{}
```

### 3. CBOR Marshaller

Compact binary object representation:

```go
import "github.com/fxamacker/cbor/v2"

type CBORMarshaller struct{}

func (m *CBORMarshaller) Marshal(v any) ([]byte, error) {
    return cbor.Marshal(v)
}

func (m *CBORMarshaller) Unmarshal(data []byte, v any) error {
    return cbor.Unmarshal(data, v)
}

// Usage
cfg.Marshaller = &CBORMarshaller{}
```

### 4. Compressed JSON Marshaller

JSON with gzip compression:

```go
import (
    "bytes"
    "compress/gzip"
    "encoding/json"
)

type CompressedJSONMarshaller struct{}

func (m *CompressedJSONMarshaller) Marshal(v any) ([]byte, error) {
    jsonData, err := json.Marshal(v)
    if err != nil {
        return nil, err
    }
    
    var buf bytes.Buffer
    gzipWriter := gzip.NewWriter(&buf)
    if _, err := gzipWriter.Write(jsonData); err != nil {
        return nil, err
    }
    if err := gzipWriter.Close(); err != nil {
        return nil, err
    }
    
    return buf.Bytes(), nil
}

func (m *CompressedJSONMarshaller) Unmarshal(data []byte, v any) error {
    gzipReader, err := gzip.NewReader(bytes.NewReader(data))
    if err != nil {
        return err
    }
    defer gzipReader.Close()
    
    var buf bytes.Buffer
    if _, err := buf.ReadFrom(gzipReader); err != nil {
        return err
    }
    
    return json.Unmarshal(buf.Bytes(), v)
}

// Usage
cfg.Marshaller = &CompressedJSONMarshaller{}
```

### 5. Encrypted Marshaller

JSON with AES encryption:

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/json"
    "io"
)

type EncryptedMarshaller struct {
    key []byte
}

func NewEncryptedMarshaller(key []byte) *EncryptedMarshaller {
    return &EncryptedMarshaller{key: key}
}

func (m *EncryptedMarshaller) Marshal(v any) ([]byte, error) {
    jsonData, err := json.Marshal(v)
    if err != nil {
        return nil, err
    }
    
    block, err := aes.NewCipher(m.key)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    
    return gcm.Seal(nonce, nonce, jsonData, nil), nil
}

func (m *EncryptedMarshaller) Unmarshal(data []byte, v any) error {
    block, err := aes.NewCipher(m.key)
    if err != nil {
        return err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return err
    }
    
    nonceSize := gcm.NonceSize()
    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    
    jsonData, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return err
    }
    
    return json.Unmarshal(jsonData, v)
}

// Usage
key := []byte("your-32-byte-key-here-1234567890") // 32 bytes for AES-256
cfg.Marshaller = NewEncryptedMarshaller(key)
```

## Performance Comparison

| Format | Speed | Size | Compatibility | Use Case |
|--------|-------|------|---------------|----------|
| JSON | Slow | Large | Excellent | Development, debugging |
| MessagePack | Fast | Small | Good | Production, high throughput |
| Protocol Buffers | Fast | Small | Good | Schema validation, microservices |
| CBOR | Fast | Small | Good | IoT, embedded systems |
| Compressed JSON | Medium | Small | Excellent | Large objects, bandwidth-limited |
| Encrypted | Slow | Medium | Good | Sensitive data |

## What You'll Learn

1. **How to implement the Marshaller interface** for custom serialization
2. **How to use different serialization formats** (MessagePack, Protobuf, CBOR)
3. **How to add compression** to reduce payload size
4. **How to add encryption** for sensitive data
5. **Performance trade-offs** between different formats

## Next Steps

After understanding custom marshallers, explore:

- **[Basic Example](../basic/)** - Learn basic cache operations
- **[Custom Logger](../custom-logger/)** - Implement custom logging
- **[Custom Local Cache](../custom-local-cache/)** - Implement custom cache storage

## Best Practices

1. **Choose the right format** for your use case
2. **Benchmark performance** with your actual data
3. **Consider compatibility** with other systems
4. **Test serialization/deserialization** thoroughly
5. **Handle errors gracefully** in Marshal/Unmarshal
6. **Document format requirements** for users

## Troubleshooting

### "value must be a proto.Message" Error

**Cause**: Using Protobuf marshaller with non-protobuf types

**Solution**: Ensure all cached values are protobuf messages
```go
// Define protobuf message
message User {
    int32 id = 1;
    string name = 2;
    string email = 3;
}

// Use in cache
user := &pb.User{Id: 1, Name: "Alice", Email: "alice@example.com"}
cache.Set(ctx, "user:1", user)
```

### Large Payload Size

**Cause**: Using JSON for large objects

**Solution**: Use MessagePack or add compression
```go
cfg.Marshaller = &MessagePackMarshaller{}
// or
cfg.Marshaller = &CompressedJSONMarshaller{}
```

## Related Documentation

- [Main README](../../README.md) - Complete library documentation
- [Marshaller Interface](../../cache/interfaces.go) - Interface definition
- [Getting Started Guide](../../GETTING_STARTED.md) - Basic setup
