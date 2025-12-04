package cache

import (
	"context"
	"time"
)

// WithContext creates a context with timeout for cache operations
func WithContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// KeyBuilder helps build consistent cache keys
type KeyBuilder struct {
	prefix string
	parts  []string
}

// NewKeyBuilder creates a new key builder
func NewKeyBuilder(prefix string) *KeyBuilder {
	return &KeyBuilder{prefix: prefix}
}

// Add adds a part to the key
func (k *KeyBuilder) Add(part string) *KeyBuilder {
	k.parts = append(k.parts, part)
	return k
}

// Build constructs the final key
func (k *KeyBuilder) Build() string {
	if len(k.parts) == 0 {
		return k.prefix
	}

	result := k.prefix
	for _, part := range k.parts {
		if result != "" {
			result += ":"
		}
		result += part
	}
	return result
}
