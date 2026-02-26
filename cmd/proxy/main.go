package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if _, err := client.Ping(ctx).Result(); err != nil {
		logger.Error("failed to connect to redis", "error", err)
	}

	store := &RedisBucketStore{client: client}
	limiter := NewLimiter(store, 10, 1)
	balancer := NewBalancer([]string{"localhost:8081", "localhost:8082", "localhost:8083"}, logger)
	handler := NewProxyHandler(limiter, balancer, logger)

	logger.Info("proxy listening", "addr", "localhost:8080")

	if err := http.ListenAndServe("localhost:8080", handler); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
