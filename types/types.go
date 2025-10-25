package types

type Action string

const (
	Set        Action = "set"
	Invalidate Action = "invalidate"
	Delete     Action = "delete"
	Clear      Action = "clear"
)

// InvalidationEvent represents a cache synchronization event.
// It can be used to propagate cache values or invalidate entries across pods.
type InvalidationEvent struct {
	Key    string `json:"key"`
	Sender string `json:"sender"`
	Action Action `json:"action"`          // "set", "invalidate", "delete", or "clear"
	Value  []byte `json:"value,omitempty"` // Serialized value for "set" action
}
