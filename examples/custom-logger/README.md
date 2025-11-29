# Custom Logger Example

This example demonstrates how to implement a custom logger for the distributed-cache library, allowing you to integrate with your existing logging infrastructure.

## What This Example Demonstrates

- **Custom Logger interface implementation** for console output
- **Integration with distributed-cache** logging system
- **Debug mode logging** to observe cache operations
- **Structured logging** with message and arguments
- **Log level support** (Debug, Info, Warn, Error)

## Key Concepts

### Logger Interface

The `Logger` interface requires implementing four methods:

```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
}
```

### Custom Logger Implementation

The example provides a console logger:

```go
type CustomConsoleLogger struct {
    prefix string
}

func (cl *CustomConsoleLogger) Debug(msg string, args ...any) {
    fmt.Printf("[DEBUG] %s: %s", cl.prefix, msg)
    if len(args) > 0 {
        fmt.Printf(" %v", args)
    }
    fmt.Println()
}
```

## Prerequisites

- Go 1.25 or later
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
cd examples/custom-logger
go run main.go
```

## Expected Output

```
========================================
Custom Logger Example
========================================

Logger Overview:
  Implementation: Console Logger
  Levels: Debug, Info, Warn, Error
  Format: [LEVEL] prefix: message args

Creating cache with custom logger...
✓ Cache initialized with custom logger

Performing cache operations (watch for log output)...
----------------------------------------
[DEBUG] DistributedCache: Set: storing in local cache [key user:1]
[DEBUG] DistributedCache: Set: stored in remote cache [key user:1]
[DEBUG] DistributedCache: Set: publishing synchronization event [key user:1 action set]
[DEBUG] DistributedCache: Get: checking local cache [key user:1]
[DEBUG] DistributedCache: Get: local cache hit [key user:1]
[DEBUG] DistributedCache: Delete: removing from local cache [key user:1]
[DEBUG] DistributedCache: Delete: removed from remote cache [key user:1]
[DEBUG] DistributedCache: Delete: publishing invalidation event [key user:1]
----------------------------------------

Cache Statistics:
  Local Hits: 1
  Local Misses: 0
  Hit Ratio: 100.00%

========================================
Custom Logger Integration:
  ✓ Implements Logger interface
  ✓ Supports all log levels
  ✓ Integrates with cache operations
  ✓ Provides visibility into cache behavior
========================================
```

## Logger Integration Examples

### 1. Console Logger (This Example)

```go
type CustomConsoleLogger struct {
    prefix string
}

func NewConsoleLogger(prefix string) dc.Logger {
    return &CustomConsoleLogger{prefix: prefix}
}

cfg.Logger = NewConsoleLogger("DistributedCache")
cfg.DebugMode = true
```

### 2. File Logger

```go
type FileLogger struct {
    file   *os.File
    logger *log.Logger
}

func NewFileLogger(filename string) (dc.Logger, error) {
    file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        return nil, err
    }
    
    return &FileLogger{
        file:   file,
        logger: log.New(file, "", log.LstdFlags),
    }, nil
}

func (fl *FileLogger) Debug(msg string, args ...any) {
    fl.logger.Printf("[DEBUG] %s %v", msg, args)
}
```

### 3. Structured Logger (e.g., Zap, Logrus)

```go
import "go.uber.org/zap"

type ZapLogger struct {
    logger *zap.Logger
}

func NewZapLogger() (dc.Logger, error) {
    logger, err := zap.NewProduction()
    if err != nil {
        return nil, err
    }
    return &ZapLogger{logger: logger}, nil
}

func (zl *ZapLogger) Debug(msg string, args ...any) {
    fields := make([]zap.Field, 0, len(args)/2)
    for i := 0; i < len(args)-1; i += 2 {
        key := fmt.Sprintf("%v", args[i])
        value := args[i+1]
        fields = append(fields, zap.Any(key, value))
    }
    zl.logger.Debug(msg, fields...)
}
```

### 4. Slog Logger (Go 1.25+)

```go
import "log/slog"

type SlogLogger struct {
    logger *slog.Logger
}

func NewSlogLogger() dc.Logger {
    return &SlogLogger{
        logger: slog.Default(),
    }
}

func (sl *SlogLogger) Debug(msg string, args ...any) {
    sl.logger.Debug(msg, args...)
}

func (sl *SlogLogger) Info(msg string, args ...any) {
    sl.logger.Info(msg, args...)
}

func (sl *SlogLogger) Warn(msg string, args ...any) {
    sl.logger.Warn(msg, args...)
}

func (sl *SlogLogger) Error(msg string, args ...any) {
    sl.logger.Error(msg, args...)
}
```

## Log Messages You'll See

When `DebugMode` is enabled, the cache logs:

### Set Operations

```
[DEBUG] Set: storing in local cache [key user:1]
[DEBUG] Set: stored in remote cache [key user:1]
[DEBUG] Set: publishing synchronization event [key user:1 action set]
```

### Get Operations

```
[DEBUG] Get: checking local cache [key user:1]
[DEBUG] Get: local cache hit [key user:1]
```

or

```
[DEBUG] Get: local cache miss [key user:1]
[DEBUG] Get: checking remote cache [key user:1]
[DEBUG] Get: remote cache hit [key user:1]
```

### Delete Operations

```
[DEBUG] Delete: removing from local cache [key user:1]
[DEBUG] Delete: removed from remote cache [key user:1]
[DEBUG] Delete: publishing invalidation event [key user:1]
```

### Synchronization Events

```
[DEBUG] Received invalidation event [key user:1 sender pod-2 action set]
[DEBUG] Updating local cache with propagated value [key user:1]
```

## Configuration Options

### Enable Debug Logging

```go
cfg.DebugMode = true
cfg.Logger = NewConsoleLogger("MyApp")
```

### Disable Debug Logging (Production)

```go
cfg.DebugMode = false
cfg.Logger = NewProductionLogger()
```

### No-Op Logger (Default)

```go
// If no logger is specified, a no-op logger is used
cfg.Logger = nil // Will use NoOpLogger internally
```

## What You'll Learn

1. **How to implement the Logger interface** for custom logging
2. **How to integrate with existing logging infrastructure** (Zap, Logrus, Slog)
3. **How to enable debug mode** for detailed cache operation logs
4. **What log messages the cache produces** during operations
5. **How to structure log arguments** for key-value pairs

## Next Steps

After understanding custom logging, explore:

- **[Debug Mode Example](../debug-mode/)** - Deep dive into debug logging
- **[Basic Example](../basic/)** - Learn basic cache operations
- **[Kubernetes Example](../kubernetes/)** - Production deployment with logging

## Best Practices

1. **Use structured logging** for production (Zap, Slog, Logrus)
2. **Disable debug mode in production** to reduce log volume
3. **Log to files or log aggregation systems** for persistence
4. **Include context in log messages** (pod ID, operation type)
5. **Use appropriate log levels** (Debug for development, Info/Warn/Error for production)
6. **Implement log rotation** for file-based loggers

## Common Logging Integrations

### Logrus

```go
import "github.com/sirupsen/logrus"

type LogrusLogger struct {
    logger *logrus.Logger
}

func (ll *LogrusLogger) Debug(msg string, args ...any) {
    ll.logger.WithFields(argsToFields(args)).Debug(msg)
}
```

### Zerolog

```go
import "github.com/rs/zerolog"

type ZerologLogger struct {
    logger zerolog.Logger
}

func (zl *ZerologLogger) Debug(msg string, args ...any) {
    event := zl.logger.Debug()
    for i := 0; i < len(args)-1; i += 2 {
        event = event.Interface(fmt.Sprintf("%v", args[i]), args[i+1])
    }
    event.Msg(msg)
}
```

## Related Documentation

- [Main README](../../README.md) - Complete library documentation
- [Logger Interface](../../cache/interfaces.go) - Interface definition
- [Debug Mode Example](../debug-mode/) - Debug logging details
