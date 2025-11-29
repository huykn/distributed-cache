package shared

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"time"
)

// TopicName is the Redis Pub/Sub channel for post updates
const TopicName = "posts_updates"

// Post represents a blog post or note (like Facebook notes)
type Post struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Author    string `json:"author"`
	Timestamp int64  `json:"timestamp"`
	Hash      string `json:"hash"` // ETag for HTTP 304 support
}

// NewPost creates a new post with computed hash
func NewPost(id, title, content, author string) *Post {
	p := &Post{
		ID:        id,
		Title:     title,
		Content:   content,
		Author:    author,
		Timestamp: time.Now().UnixNano(),
	}
	p.ComputeHash()
	return p
}

// ComputeHash calculates the MD5 hash of the post for ETag support
func (p *Post) ComputeHash() {
	data := p.ID + p.Title + p.Content + p.Author
	hash := md5.Sum([]byte(data))
	p.Hash = hex.EncodeToString(hash[:])
}

// ToBytes serializes the post to JSON bytes
func (p *Post) ToBytes() ([]byte, error) {
	return json.Marshal(p)
}

// FromBytes deserializes a post from JSON bytes
func FromBytes(data []byte) (*Post, error) {
	var p Post
	err := json.Unmarshal(data, &p)
	return &p, err
}
