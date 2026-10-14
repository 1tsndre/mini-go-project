// Package memory is an in-memory caches.Cache for tests. It stores JSON like the
// Redis cache, but ignores expiry.
package memory

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/1tsndre/mini-go-project/store-service/internal/repository/caches"
)

var errMiss = errors.New("cache miss")

type Cache struct {
	mu   sync.Mutex
	data map[string][]byte
}

func NewCache() *Cache {
	return &Cache{data: make(map[string][]byte)}
}

func (c *Cache) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.data[key]
	if !ok {
		return nil, errMiss
	}
	return value, nil
}

func (c *Cache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = data
	return nil
}

func (c *Cache) SetNX(_ context.Context, key string, value any, _ time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.data[key]; ok {
		return false, nil
	}
	c.data[key] = data
	return true, nil
}

func (c *Cache) Take(_ context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.data[key]
	if !ok {
		return nil, caches.ErrMiss
	}
	delete(c.data, key)
	return value, nil
}

func (c *Cache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *Cache) Exists(_ context.Context, key string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.data[key]
	return ok, nil
}

// Clear removes every key.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string][]byte)
}
