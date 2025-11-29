package cache

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNoOpLogger(t *testing.T) {
	logger := NewNoOpLogger()
	if logger == nil {
		t.Fatal("Logger should not be nil")
	}

	// These should not panic - they're no-ops
	logger.Debug("test message", "key", "value")
	logger.Info("test message", "key", "value")
	logger.Warn("test message", "key", "value")
	logger.Error("test message", "key", "value")

	// Test with no args
	logger.Debug("test message")
	logger.Info("test message")
	logger.Warn("test message")
	logger.Error("test message")

	// Test with nil
	logger.Debug("test message", nil)
	logger.Info("test message", nil)
	logger.Warn("test message", nil)
	logger.Error("test message", nil)
}

func TestConsoleLoggerDebug(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger := NewConsoleLogger("TestPrefix")
	logger.Debug("test message", "key", "value")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "[DEBUG]") {
		t.Errorf("Expected [DEBUG] in output, got: %s", output)
	}
	if !strings.Contains(output, "TestPrefix") {
		t.Errorf("Expected TestPrefix in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in output, got: %s", output)
	}
}

func TestConsoleLoggerInfo(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger := NewConsoleLogger("TestPrefix")
	logger.Info("info message", "key", "value")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "[INFO]") {
		t.Errorf("Expected [INFO] in output, got: %s", output)
	}
	if !strings.Contains(output, "info message") {
		t.Errorf("Expected 'info message' in output, got: %s", output)
	}
}

func TestConsoleLoggerWarn(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger := NewConsoleLogger("TestPrefix")
	logger.Warn("warning message", "key", "value")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "[WARN]") {
		t.Errorf("Expected [WARN] in output, got: %s", output)
	}
	if !strings.Contains(output, "warning message") {
		t.Errorf("Expected 'warning message' in output, got: %s", output)
	}
}

func TestConsoleLoggerError(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger := NewConsoleLogger("TestPrefix")
	logger.Error("error message", "key", "value")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "[ERROR]") {
		t.Errorf("Expected [ERROR] in output, got: %s", output)
	}
	if !strings.Contains(output, "error message") {
		t.Errorf("Expected 'error message' in output, got: %s", output)
	}
}

func TestConsoleLoggerWithoutArgs(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger := NewConsoleLogger("Test")
	logger.Debug("message without args")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "message without args") {
		t.Errorf("Expected 'message without args' in output, got: %s", output)
	}
}

func TestJSONMarshallerMarshal(t *testing.T) {
	marshaller := NewJSONMarshaller()
	if marshaller == nil {
		t.Fatal("Marshaller should not be nil")
	}

	type testStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	data, err := marshaller.Marshal(testStruct{Name: "John", Age: 30})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `{"name":"John","age":30}`
	if string(data) != expected {
		t.Fatalf("Expected %s, got %s", expected, string(data))
	}
}

func TestJSONMarshallerUnmarshal(t *testing.T) {
	marshaller := NewJSONMarshaller()

	type testStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	data := []byte(`{"name":"John","age":30}`)
	var result testStruct
	err := marshaller.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Name != "John" || result.Age != 30 {
		t.Fatalf("Expected {Name:John Age:30}, got %+v", result)
	}
}

func TestJSONMarshallerMarshalString(t *testing.T) {
	marshaller := NewJSONMarshaller()

	data, err := marshaller.Marshal("test string")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `"test string"`
	if string(data) != expected {
		t.Fatalf("Expected %s, got %s", expected, string(data))
	}
}

func TestJSONMarshallerMarshalInt(t *testing.T) {
	marshaller := NewJSONMarshaller()

	data, err := marshaller.Marshal(42)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `42`
	if string(data) != expected {
		t.Fatalf("Expected %s, got %s", expected, string(data))
	}
}

func TestJSONMarshallerMarshalMap(t *testing.T) {
	marshaller := NewJSONMarshaller()

	testMap := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	data, err := marshaller.Marshal(testMap)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal to verify
	var result map[string]any
	err = marshaller.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result["key1"] != "value1" {
		t.Fatalf("Expected key1=value1, got %v", result["key1"])
	}
	// JSON numbers are float64
	if result["key2"] != float64(42) {
		t.Fatalf("Expected key2=42, got %v", result["key2"])
	}
}

func TestJSONMarshallerUnmarshalInvalidJSON(t *testing.T) {
	marshaller := NewJSONMarshaller()

	var result map[string]any
	err := marshaller.Unmarshal([]byte("invalid json"), &result)
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}

func TestJSONMarshallerMarshalNil(t *testing.T) {
	marshaller := NewJSONMarshaller()

	data, err := marshaller.Marshal(nil)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	expected := `null`
	if string(data) != expected {
		t.Fatalf("Expected %s, got %s", expected, string(data))
	}
}

func TestJSONMarshallerRoundTrip(t *testing.T) {
	marshaller := NewJSONMarshaller()

	type testStruct struct {
		Name     string            `json:"name"`
		Age      int               `json:"age"`
		Active   bool              `json:"active"`
		Tags     []string          `json:"tags"`
		Metadata map[string]string `json:"metadata"`
	}

	original := testStruct{
		Name:   "Alice",
		Age:    25,
		Active: true,
		Tags:   []string{"tag1", "tag2"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Marshal
	data, err := marshaller.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	var result testStruct
	err = marshaller.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify
	if result.Name != original.Name {
		t.Errorf("Name mismatch: expected %s, got %s", original.Name, result.Name)
	}
	if result.Age != original.Age {
		t.Errorf("Age mismatch: expected %d, got %d", original.Age, result.Age)
	}
	if result.Active != original.Active {
		t.Errorf("Active mismatch: expected %v, got %v", original.Active, result.Active)
	}
	if len(result.Tags) != len(original.Tags) {
		t.Errorf("Tags length mismatch: expected %d, got %d", len(original.Tags), len(result.Tags))
	}
	if len(result.Metadata) != len(original.Metadata) {
		t.Errorf("Metadata length mismatch: expected %d, got %d", len(original.Metadata), len(result.Metadata))
	}
}
