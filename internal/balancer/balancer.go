// Package balancer implements load balancing across backend servers
// includes proxy caching and health checks
package balancer

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type HealthCheckConfig struct {
	IntervalSeconds int
	TimeoutSeconds  int
	MaxRetries      int
}

type backend struct {
	address string
	up      atomic.Bool
}

func (ba *backend) isUp() bool {
	return ba.up.Load()
}

type Balancer struct {
	backends  []*backend
	algorithm Algorithm

	hc     HealthCheckConfig
	logger *slog.Logger
}

// TODO: Make health checks injectable or configurable
//   - This makes tests guaranteed and not coincidence.
//     Because healthchecks spin up as soon as the balancer is created
func (b *Balancer) runHealthcheck(backend *backend) {
	client := http.Client{
		Timeout: time.Duration(b.hc.TimeoutSeconds) * time.Second,
	}

	maxRetrySleep := 10.0

	for {
		retries := 0
		for retries <= b.hc.MaxRetries {
			_, err := client.Head(backend.address)
			if err != nil {
				backend.up.Store(false)
				b.logger.Warn("Server is down", "address", backend.address, "retries", retries)
				sleep := math.Min(math.Exp(float64(retries)/2), maxRetrySleep)
				time.Sleep(time.Duration(sleep) * time.Second)
				retries++
			} else {
				backend.up.Store(true)
				break
			}
		}
		time.Sleep(time.Duration(b.hc.IntervalSeconds) * time.Second)
	}
}

func NewBalancer(addresses []string, hc HealthCheckConfig, algorithm Algorithm, logger *slog.Logger) *Balancer {
	b := &Balancer{hc: hc, algorithm: algorithm, logger: logger}
	backends := make([]*backend, len(addresses))
	for i, addr := range addresses {
		// Add scheme if missing
		if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
			addr = "http://" + addr
		}
		currBackend := &backend{address: addr}

		// Start optimistic, assume backends are up until proven otherwise
		// (if you configured a backend it's probably up :D )
		currBackend.up.Store(true)
		backends[i] = currBackend
		go b.runHealthcheck(currBackend)
	}
	b.backends = backends
	return b
}

func (b *Balancer) GetNextBackend() (string, error) {
	idx := b.algorithm.Next(len(b.backends))
	for attempts := 0; attempts < len(b.backends); attempts++ {
		backend := b.backends[(idx+attempts)%len(b.backends)]
		if backend.isUp() {
			return backend.address, nil
		}
	}
	return "", fmt.Errorf("no backends available") // No backends up
}
