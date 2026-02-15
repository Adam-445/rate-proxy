package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

const (
	MaxCapacity     float64 = 10
	TokensPerSecond float64 = 1
)

type bucket struct {
	Tokens    float64   `json:"tokens"`
	Timestamp time.Time `json:"timestamp"`
}

type limiter struct {
	client   *redis.Client
	buckets  map[string]*bucket // TODO: Add bucket cleanup (TTL, periodic cleanup, LRU)
	capacity float64
	rate     float64
}

func NewLimiter(redisClient *redis.Client, maxCapacity float64, tokensPerSecond float64) *limiter {
	return &limiter{client: redisClient, buckets: make(map[string]*bucket), capacity: maxCapacity, rate: tokensPerSecond}
}

func (l *limiter) GetOrCreateBucket(key string, now time.Time) (*bucket, error) {
	val, err := l.client.Get(ctx, key).Bytes()
	if err == nil {
		var b bucket
		if err := json.Unmarshal(val, &b); err != nil {
			return nil, err
		}
		return &b, nil
	}

	if err != redis.Nil {
		return nil, err
	}

	// Not found. Create new
	newBucket := bucket{
		Tokens:    l.capacity,
		Timestamp: now,
	}

	data, _ := json.Marshal(newBucket)

	err = l.client.Set(ctx, key, data, time.Hour).Err()
	if err != nil {
		return nil, err
	}

	return &newBucket, nil
}

func (l *limiter) Allow(clientID string, now time.Time) bool {
	key := "client:" + clientID
	userBucket, err := l.GetOrCreateBucket(key, now)
	if err != nil {
		return false
	}

	defer func() {
		data, err := json.Marshal(userBucket)
		if err != nil {
			return
		}
		_ = l.client.Set(ctx, key, data, time.Hour)
	}()

	// Refill
	elapsed := now.Sub(userBucket.Timestamp).Seconds()

	fmt.Println("elapsed ", elapsed,
		"tokens ", userBucket.Tokens,
		"timestamp ", userBucket.Timestamp)
	userBucket.Tokens = math.Min(l.capacity, userBucket.Tokens+(elapsed*l.rate))
	userBucket.Timestamp = now

	// Consume
	if userBucket.Tokens >= 1 {
		userBucket.Tokens -= 1
	} else {
		return false
	}
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
	limiter := NewLimiter(client, 10, 1)

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
