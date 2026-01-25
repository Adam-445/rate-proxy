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

var buckets sync.Map

const (
	MaxCapacity     float64 = 10
	TokensPerSecond float64 = 1
)

type bucket struct {
	tokens    float64
	timestamp time.Time
	mu        sync.Mutex
}

func main() {
	// Use httputil.ReverseProxy to forward requests from client to process / server
	target := url.URL{
		Scheme: "http",
		Host:   "localhost:8081",
	}
	proxy := httputil.NewSingleHostReverseProxy(&target)

	// Create a handler that wraps the proxy
	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			host = req.RemoteAddr
		}

		now := time.Now()
		// Check if a bucket for the current user exists
		actual, _ := buckets.LoadOrStore(host, &bucket{tokens: MaxCapacity, timestamp: now})

		userBucket, ok := actual.(*bucket)
		if !ok {
			http.Error(rw, "Internal server error", http.StatusInternalServerError)
			return
		}

		userBucket.mu.Lock()
		defer userBucket.mu.Unlock()

		// Refill
		elapsed := time.Since(userBucket.timestamp).Seconds()

		userBucket.tokens = math.Min(MaxCapacity, userBucket.tokens+(elapsed*TokensPerSecond))
		userBucket.timestamp = now

		// Consume
		if userBucket.tokens >= 1 {
			userBucket.tokens -= 1
		} else {
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
