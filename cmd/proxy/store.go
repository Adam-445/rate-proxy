package main

import (
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

func (r *RedisBucketStore) Get(key string) (*bucket, error) {
	val, err := r.client.Get(ctx, key).Bytes()
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
	return r.client.Set(ctx, key, data, ttl).Err()
}

type InMemroyBucketStore struct {
	data map[string]*bucket
	mu   sync.Mutex
}

func (m *InMemroyBucketStore) Get(key string) (*bucket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.data[key]
	if !ok {
		return nil, nil
	}
	return b, nil
}

func (m *InMemroyBucketStore) Set(key string, b *bucket, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = b
	return nil
}
