// Package main is the entry point for the reverse proxy server
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/Adam-445/rate-proxy/internal/balancer"
	"github.com/Adam-445/rate-proxy/internal/config"
	"github.com/Adam-445/rate-proxy/internal/limiter"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if _, err := client.Ping(context.Background()).Result(); err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}

	store := limiter.NewRedisBucketStore(client)
	l := limiter.NewLimiter(store, float64(cfg.Frontend.RateLimit.Capacity), float64(cfg.Frontend.RateLimit.Rate))

	addrs := make([]string, len(cfg.Backend.Servers))
	for i, s := range cfg.Backend.Servers {
		addrs[i] = s.Address
	}
	hc := balancer.HealthCheckConfig{
		IntervalSeconds: cfg.Backend.HealthCheck.IntervalSeconds,
		TimeoutSeconds:  cfg.Backend.HealthCheck.TimeoutSeconds,
		MaxRetries:      cfg.Backend.HealthCheck.MaxRetries,
	}
	b := balancer.NewBalancer(addrs, hc, logger)

	handler := NewProxyHandler(l, b, logger)

	logger.Info("proxy listening", "addr", cfg.Frontend.Port)
	if err := http.ListenAndServe(cfg.Frontend.Port, handler); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
