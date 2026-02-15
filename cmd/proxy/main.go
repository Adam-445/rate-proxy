package main

import (
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

const (
	MaxCapacity     float64 = 10
	TokensPerSecond float64 = 1
)

type bucket struct {
	tokens    float64
	timestamp time.Time
	mu        sync.Mutex
}

type limiter struct {
	buckets  map[string]*bucket // TODO: Add bucket cleanup (TTL, periodic cleanup, LRU)
	mu       sync.Mutex
	capacity float64
	rate     float64
}

func NewLimiter(maxCapacity float64, tokensPerSecond float64) *limiter {
	return &limiter{buckets: make(map[string]*bucket), capacity: maxCapacity, rate: tokensPerSecond}
}

func (l *limiter) Allow(clientID string, now time.Time) bool {
	// Ckeck if a bucket for the current user exists
	_, ok := l.buckets[clientID]
	if !ok {
		l.buckets[clientID] = &bucket{tokens: l.capacity, timestamp: now}
	}
	userBucket := l.buckets[clientID]

	userBucket.mu.Lock()
	defer userBucket.mu.Unlock()

	// Refill
	elapsed := time.Since(userBucket.timestamp).Seconds()

	userBucket.tokens = math.Min(l.capacity, userBucket.tokens+(elapsed*l.rate))
	userBucket.timestamp = now

	// Consume
	if userBucket.tokens >= 1 {
		userBucket.tokens -= 1
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
	limiter := NewLimiter(10, 1)

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
