package main

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

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
