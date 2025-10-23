package storage

import (
	"encoding/json"
	"errors"
)

// Serializer defines the interface for serialization.
type Serializer interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// JSONSerializer implements Serializer using JSON.
type JSONSerializer struct{}

// Marshal serializes a value to JSON.
func (js *JSONSerializer) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal deserializes a value from JSON.
func (js *JSONSerializer) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// NewJSONSerializer creates a new JSON serializer.
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// GetSerializer returns a serializer for the given format.
func GetSerializer(format string) (Serializer, error) {
	switch format {
	case "json":
		return NewJSONSerializer(), nil
	default:
		return nil, errors.New("unsupported serialization format: " + format)
	}
}
