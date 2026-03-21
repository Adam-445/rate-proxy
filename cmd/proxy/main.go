// Package main is the entry point for the reverse proxy server
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Adam-445/rate-proxy/internal/balancer"
	"github.com/Adam-445/rate-proxy/internal/config"
	"github.com/Adam-445/rate-proxy/internal/limiter"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	f, err := os.Open(*configPath)
	if err != nil {
		logger.Error("failed to open config", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := f.Close(); err != nil {
			logger.Warn("error closing config file", "error", err)
		}
	}()

	cfg, err := config.LoadConfig(f)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	var store limiter.BucketStore
	switch cfg.Storage.Type {
	case "redis":
		client := redis.NewClient(&redis.Options{
			Addr:     cfg.Storage.Redis.Address,
			Password: cfg.Storage.Redis.Password,
			DB:       cfg.Storage.Redis.DB,
		})
		if _, err := client.Ping(context.Background()).Result(); err != nil {
			logger.Error("failed to connect to redis", "error", err)
			os.Exit(1)
		}
		store = limiter.NewRedisBucketStore(client)
	case "memory":
		store = limiter.NewInMemoryBucketStore()
	default:
		logger.Error("Unrecognized storage type", "type", cfg.Storage.Type)
		os.Exit(1)
	}
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
	var algorithm balancer.Algorithm
	switch cfg.Backend.Algorithm {
	case "round-robin":
		algorithm = &balancer.RoundRobin{}
	case "random-selection":
		algorithm = &balancer.RandomSelection{}
	default:
		logger.Error("Unrecognized balancing algorithm", "algorithm", cfg.Backend.Algorithm)
		os.Exit(1)

	}
	b := balancer.NewBalancer(addrs, hc, algorithm, logger)

	proxy := NewProxyHandler(b, logger)
	handler := LoggingMiddleware(logger, RateLimitMiddleware(l, proxy))

	server := &http.Server{
		Addr:    cfg.Frontend.Port,
		Handler: handler,
	}
	errChan := make(chan error, 1)
	logger.Info("proxy listening", "addr", cfg.Frontend.Port)
	go startServer(server, errChan)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// wait for either a signal or a server error
	select {
	case sig := <-sigChan:
		logger.Info("Received signal. Shutting down.", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Shutdown closes the listener and waits for in-flight requests
		if err := server.Shutdown(ctx); err != nil {
			logger.Warn("Graceful shutdown failed", "error", err)
		} else {
			logger.Info("Server shut down.")
		}
	case err := <-errChan:
		if err != nil {
			logger.Error("Server error", "error", err)
		}
	}
}

func startServer(server *http.Server, errorChannel chan error) {
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		errorChannel <- err
	}
	close(errorChannel)
}
