package limiter

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type bucket struct {
	Tokens    float64   `json:"tokens"`
	Timestamp time.Time `json:"timestamp"`
}

type BucketStore interface {
	Get(key string) (*bucket, error)
	Set(key string, b *bucket, ttl time.Duration) error
}

type RedisBucketStore struct {
	client *redis.Client
}

func NewRedisBucketStore(client *redis.Client) *RedisBucketStore {
	return &RedisBucketStore{client: client}
}

func (r *RedisBucketStore) Get(key string) (*bucket, error) {
	// TODO: move to accepting context.Context as function parameters instead of calling it on every function call
	val, err := r.client.Get(context.Background(), key).Bytes()
	if err == redis.Nil {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}

	var b bucket
	if err := json.Unmarshal(val, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *RedisBucketStore) Set(key string, b *bucket, ttl time.Duration) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return r.client.Set(context.Background(), key, data, ttl).Err()
}

type InMemoryBucketStore struct {
	data map[string]*bucket
	mu   sync.Mutex
}

func NewInMemoryBucketStore() *InMemoryBucketStore {
	return &InMemoryBucketStore{data: make(map[string]*bucket)}
}

func (m *InMemoryBucketStore) Get(key string) (*bucket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.data[key]
	if !ok {
		return nil, nil
	}
	return b, nil
}

func (m *InMemoryBucketStore) Set(key string, b *bucket, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = b
	return nil
}
