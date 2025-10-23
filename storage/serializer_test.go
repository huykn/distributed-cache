package storage

import (
	"testing"
)

func TestJSONSerializerMarshal(t *testing.T) {
	serializer := NewJSONSerializer()

	testData := map[string]any{
		"name": "John",
		"age":  30,
	}

	data, err := serializer.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Marshaled data should not be empty")
	}
}

func TestJSONSerializerUnmarshal(t *testing.T) {
	serializer := NewJSONSerializer()

	testData := map[string]any{
		"name": "John",
		"age":  30,
	}

	// Marshal
	data, err := serializer.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	var result map[string]any
	err = serializer.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["name"] != "John" {
		t.Fatalf("Expected 'John', got %v", result["name"])
	}
}

func TestGetSerializer(t *testing.T) {
	tests := []struct {
		format string
		valid  bool
	}{
		{"json", true},
		{"invalid", false},
	}

	for _, test := range tests {
		serializer, err := GetSerializer(test.format)
		if test.valid && err != nil {
			t.Fatalf("Failed to get serializer for format %s: %v", test.format, err)
		}
		if !test.valid && err == nil {
			t.Fatalf("Should return error for invalid format %s", test.format)
		}
		if test.valid && serializer == nil {
			t.Fatalf("Serializer should not be nil for format %s", test.format)
		}
	}
}

func TestJSONSerializerWithStruct(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	serializer := NewJSONSerializer()
	user := User{ID: 1, Name: "John"}

	// Marshal
	data, err := serializer.Marshal(user)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	var result User
	err = serializer.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result.ID != user.ID || result.Name != user.Name {
		t.Fatalf("Unmarshaled data doesn't match original")
	}
}
