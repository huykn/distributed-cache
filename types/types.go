package types

// InvalidationEvent represents a cache synchronization event.
// It can be used to propagate cache values or invalidate entries across pods.
type InvalidationEvent struct {
	Key    string `json:"key"`
	Sender string `json:"sender"`
	Action string `json:"action"`          // "set", "invalidate", "delete", or "clear"
	Value  []byte `json:"value,omitempty"` // Serialized value for "set" action
}
