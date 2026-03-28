// Package balancer implements load balancing across backend servers
package balancer

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
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
	hc        HealthCheckConfig
	logger    *slog.Logger

	stop     chan struct{}
	stopOnce sync.Once
}

// Stop signals all background healthchecks goroutines to exit.
// It is safe to call multiple times from any goroutine. After Stop returns, no further healthchecks
// state changes will occur.
func (b *Balancer) Stop() {
	b.stopOnce.Do(func() { close(b.stop) })
}

// runHealthcheck runs in its own goroutine for each backend. It probes the backend on the configured interval
// and marks it up or down accordingly.
//
// TODO: Make health checks injectable so tests don't rely on timing coincidence
func (b *Balancer) runHealthcheck(be *backend) {
	maxRetrySleep := 10.0

	client := http.Client{
		Timeout: time.Duration(b.hc.TimeoutSeconds) * time.Second,
	}

	for {
		// Retry loop: on failure, back off exponentially up to maxRetrySleep seconds
		for retries := 0; retries <= b.hc.MaxRetries; retries++ {
			// Check for stop before each probe attempt
			select {
			case <-b.stop:
				return
			default:
			}

			_, err := client.Head(be.address)
			if err == nil {
				be.up.Store(true)
				break // healthy, skip remaining entries
			}

			be.up.Store(false)
			b.logger.Warn("backend unreachable", "address", be.address, "attempt", retries+1)

			sleep := time.Duration(math.Min(math.Exp(float64(retries)/2), maxRetrySleep) * float64(time.Second))

			select {
			case <-b.stop:
				return
			case <-time.After(sleep):
			}
		}

		// Wait for next check, but bail out immediately on Stop
		select {
		case <-b.stop:
			return
		case <-time.After(time.Duration(b.hc.IntervalSeconds) * time.Second):
		}
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

		currBackend.up.Store(true) // optimistic: assume up until the first healthcheck says otherwise
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
