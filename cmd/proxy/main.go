package main

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

const (
	MaxCapacity     float64 = 10
	TokensPerSecond float64 = 1
)

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

type limiter struct {
	store    BucketStore
	capacity float64
	rate     float64
}

func NewLimiter(store BucketStore, maxCapacity float64, tokensPerSecond float64) *limiter {
	return &limiter{store: store, capacity: maxCapacity, rate: tokensPerSecond}
}

func (l *limiter) Allow(clientID string, now time.Time) bool {
	key := "client:" + clientID

	// Get from store
	userBucket, err := l.store.Get(key)
	if err != nil {
		return false
	}

	// Create if it doesnt exist
	if userBucket == nil {
		userBucket = &bucket{
			Tokens:    l.capacity,
			Timestamp: now,
		}
	}

	// Refill
	elapsed := now.Sub(userBucket.Timestamp).Seconds()
	userBucket.Tokens = math.Min(l.capacity, userBucket.Tokens+(elapsed*l.rate))
	userBucket.Timestamp = now

	// Consume
	if userBucket.Tokens >= 1 {
		userBucket.Tokens -= 1
	} else {
		return false
	}

	// Updates timestamp only if consumes token
	_ = l.store.Set(key, userBucket, time.Hour)
	return true
}

func main() {
	// Use httputil.ReverseProxy to forward requests from client to process / server
	target := url.URL{
		Scheme: "http",
		Host:   "localhost:8081",
	}
	proxy := httputil.NewSingleHostReverseProxy(&target)
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

	// Create a handler that wraps the proxy
	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			host = req.RemoteAddr
		}
		if !limiter.Allow(host, time.Now()) {
			http.Error(rw, "Request limit reached", http.StatusTooManyRequests)
			return
		}

		// Original proxy's ServeHTTP method
		proxy.ServeHTTP(rw, req)
	})

	if err := http.ListenAndServe("localhost:8080", handler); err != nil {
		return
	}
}
