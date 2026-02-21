package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

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

type bucket struct {
	Tokens    float64   `json:"tokens"`
	Timestamp time.Time `json:"timestamp"`
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

func main() {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if _, err := client.Ping(ctx).Result(); err != nil {
		panic(err)
	}
	redisStore := &RedisBucketStore{client: client}

	limiter := NewLimiter(redisStore, 10, 1)

	balancer := NewBalancer([]string{"localhost:8081", "localhost:8082", "localhost:8083"})

	// Create a handler that wraps the proxy
	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			host = req.RemoteAddr
		}

		// Rate limiting check
		if !limiter.Allow(host, time.Now()) {
			http.Error(rw, "Request limit reached", http.StatusTooManyRequests)
			return
		}

		// Get next backend
		backend := balancer.GetNextBackend()

		// Create a proxy for this specific request
		target, _ := url.Parse("http://" + backend)
		proxy := balancer.GetProxy(target)

		// Original proxy's ServeHTTP method
		proxy.ServeHTTP(rw, req)
	})

	if err := http.ListenAndServe("localhost:8080", handler); err != nil {
		return
	}
}
