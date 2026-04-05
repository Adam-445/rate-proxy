// Package main is the entry point for the reverse proxy server
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Adam-445/configr"
	"github.com/Adam-445/rate-proxy/internal/balancer"
	"github.com/Adam-445/rate-proxy/internal/config"
	"github.com/Adam-445/rate-proxy/internal/limiter"
	"github.com/redis/go-redis/v9"
)

// app owns every long lived component of the proxy.
type app struct {
	config      *config.Config
	limiter     *limiter.Limiter
	balancer    *balancer.Balancer
	server      *http.Server
	logger      *slog.Logger
	redisClient *redis.Client //	nil when storage type != "redis"
}

// newApp builds every component from the config file at configPath. If any step fails the error is returned
// and no resources are left open
func newApp(configPath string, logger *slog.Logger) (*app, error) {
	a := &app{logger: logger}

	var err error
	a.config, err = configr.Load(
		configPath,
		configr.WithDefaults(config.ApplyDefaults),
		configr.WithValidate(config.Validate),
	)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	store, redisClient, err := buildStore(a.config)
	if err != nil {
		return nil, err
	}
	a.redisClient = redisClient

	a.limiter = limiter.NewLimiter(
		store,
		float64(a.config.Frontend.RateLimit.Capacity),
		float64(a.config.Frontend.RateLimit.Rate),
	)

	a.balancer = buildBalancer(a.config, logger)

	a.server = &http.Server{
		Addr:    a.config.Frontend.Port,
		Handler: buildHandler(a.balancer, a.limiter, logger),
	}

	return a, nil
}

// run starts the HTTP server and blocks until ctx is cancelled (eg. on SIGNINT/SIGTERM) or the server
// exits with an error. It always attempts a graceful shutdown before returning.
func (a *app) run(ctx context.Context) error {
	errChan := make(chan error, 1)
	go func() {
		a.logger.Info("proxy listening", "addr", a.server.Addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
		close(errChan)
	}()

	select {
	case <-ctx.Done():
		a.logger.Info("signal received, shutting down")
	case err := <-errChan:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	a.logger.Info("shut down cleanly")
	return nil
}

// stop releases background resources. Safe to call after run() has returned
func (a *app) stop() {
	a.balancer.Stop()
	if a.redisClient != nil {
		if err := a.redisClient.Close(); err != nil {
			a.logger.Warn("error closing redis client", "error", err)
		}
	}
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	configPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	a, err := newApp(*configPath, logger)
	if err != nil {
		logger.Error("startup failed", "error", err)
		os.Exit(1)
	}
	defer a.stop()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := a.run(ctx); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

// Helpers

func buildStore(cfg *config.Config) (limiter.BucketStore, *redis.Client, error) {
	switch cfg.Storage.Type {
	case "redis":
		client := redis.NewClient(&redis.Options{
			Addr:     cfg.Storage.Redis.Address,
			Password: cfg.Storage.Redis.Password,
			DB:       cfg.Storage.Redis.DB,
		})

		pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, err := client.Ping(pingCtx).Result(); err != nil {
			_ = client.Close() // dont leak the connection pool on failure
			return nil, nil, fmt.Errorf("connecting to redis at %s: %w", cfg.Storage.Redis.Address, err)
		}
		return limiter.NewRedisBucketStore(client), client, nil
	case "memory":
		return limiter.NewInMemoryBucketStore(), nil, nil
	default:
		return nil, nil, fmt.Errorf("unrecognized storage type %q", cfg.Storage.Type)
	}
}

func buildBalancer(cfg *config.Config, logger *slog.Logger) *balancer.Balancer {
	addrs := make([]string, len(cfg.Backend.Servers))
	for i, s := range cfg.Backend.Servers {
		addrs[i] = s.Address
	}

	hc := balancer.HealthCheckConfig{
		IntervalSeconds: cfg.Backend.HealthCheck.IntervalSeconds,
		MaxRetries:      cfg.Backend.HealthCheck.MaxRetries,
	}

	var algorithm balancer.Algorithm
	switch cfg.Backend.Algorithm {
	case "round-robin":
		algorithm = &balancer.RoundRobin{}
	case "random-selection":
		algorithm = &balancer.RandomSelection{}
	default:
		// Validate() already rejects unknown algorithms, so this branch is
		// unreachable in practice. Panic is intentional.
		panic(
			fmt.Sprintf(
				"unrecognized algorithm %q (should have been caught by config.Validate)",
				cfg.Backend.Algorithm,
			),
		)
	}

	prober := balancer.NewHTTPProber(
		cfg.Backend.HealthCheck.Path,
		time.Duration(cfg.Backend.HealthCheck.TimeoutSeconds)*time.Second,
	)

	return balancer.NewBalancer(addrs, hc, algorithm, prober, logger)
}

func buildHandler(b *balancer.Balancer, l *limiter.Limiter, logger *slog.Logger) http.Handler {
	proxy := NewProxyHandler(b, logger)
	return LoggingMiddleware(logger, RateLimitMiddleware(l, proxy, logger))
}
