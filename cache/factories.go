package cache

import (
	"encoding/json"
	"fmt"
)

// NoOpLogger is a logger that does nothing.
type NoOpLogger struct{}

// Debug logs a debug message (no-op).
func (n *NoOpLogger) Debug(msg string, args ...any) {}

// Info logs an info message (no-op).
func (n *NoOpLogger) Info(msg string, args ...any) {}

// Warn logs a warning message (no-op).
func (n *NoOpLogger) Warn(msg string, args ...any) {}

// Error logs an error message (no-op).
func (n *NoOpLogger) Error(msg string, args ...any) {}

// NewNoOpLogger creates a new no-op logger.
func NewNoOpLogger() Logger {
	return &NoOpLogger{}
}

type ConsoleLogger struct {
	prefix string
}

// Debug logs a debug message to console.
func (cl *ConsoleLogger) Debug(msg string, args ...any) {
	fmt.Printf("[DEBUG] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

// Info logs an info message to console.
func (cl *ConsoleLogger) Info(msg string, args ...any) {
	fmt.Printf("[INFO] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

// Warn logs a warning message to console.
func (cl *ConsoleLogger) Warn(msg string, args ...any) {
	fmt.Printf("[WARN] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

// Error logs an error message to console.
func (cl *ConsoleLogger) Error(msg string, args ...any) {
	fmt.Printf("[ERROR] %s: %s", cl.prefix, msg)
	if len(args) > 0 {
		fmt.Printf(" %v", args)
	}
	fmt.Println()
}

// NewConsoleLogger creates a new console logger.
func NewConsoleLogger(prefix string) Logger {
	return &ConsoleLogger{prefix: prefix}
}

// JSONMarshaller is a marshaller that uses the standard JSON library.
type JSONMarshaller struct{}

// Marshal serializes a value to JSON.
func (jm *JSONMarshaller) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal deserializes a value from JSON.
func (jm *JSONMarshaller) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// NewJSONMarshaller creates a new JSON marshaller.
func NewJSONMarshaller() Marshaller {
	return &JSONMarshaller{}
}
