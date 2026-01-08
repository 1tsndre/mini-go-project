package caches

import (
	"context"
	"time"
)

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	// Take gets and deletes key in one step.
	Take(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}
