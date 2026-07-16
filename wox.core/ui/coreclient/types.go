package coreclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// Backend is the launcher-facing API supplied by the embedding core.
type Backend interface {
	Connect(ctx context.Context) error
	Post(ctx context.Context, path string, data any, target any) error
	Get(ctx context.Context, path string, target any) error
	Close() error
}

// NewID returns a UUID-shaped random identifier without another dependency.
func NewID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("generate id: %v", err))
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := make([]byte, 32)
	hex.Encode(encoded, value[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:])
}
