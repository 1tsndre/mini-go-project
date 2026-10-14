package caches

import (
	"context"
	"errors"
	"time"
)

// ErrMiss is returned by Take for a key that does not exist.
var ErrMiss = errors.New("cache miss")

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	// SetNX stores value only if key does not exist yet, and reports whether it did.
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
	// Take returns the value of key and deletes the key in one step, so only one
	// caller can ever get it; ErrMiss if the key does not exist.
	Take(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}
