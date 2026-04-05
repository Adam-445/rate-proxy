// Package balancer implements load balancing across backend servers
package balancer

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// HealthCheckConfig controls the cadence and resilience of the health probes.
// The probe mechanism itself is owned by the Prober passed to NewBalancer, this struct only
// holds what the balancer loop needs
type HealthCheckConfig struct {
	IntervalSeconds int // how long to wait between full probe cycles
	MaxRetries      int // how many extra probes to attempt immediately after a failure.
}

type backend struct {
	address string
	up      atomic.Bool
}

func (ba *backend) isUp() bool {
	return ba.up.Load()
}

// Balancer routes requests across a pool of backends and keeps their health status
// up to date via background goroutines
type Balancer struct {
	backends  []*backend
	algorithm Algorithm
	hc        HealthCheckConfig
	prober    Prober
	logger    *slog.Logger

	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// Stop signals all background healthchecks goroutines to exit and waits for them to do so.
// It is safe to call multiple times from any goroutine. After Stop returns, no further healthchecks
// state changes will occur.
func (b *Balancer) Stop() {
	b.stopOnce.Do(b.cancel)
	b.wg.Wait()
}

// runHealthcheck runs in its own goroutine per backend. On each cycle it calls b.prober.Probe() up to
// MaxRetries+1 times with exponential backoff. A successful probe marks the backend up and waits IntervalSeconds
// before the next cycle. All sleeps respond to stop() immediately via b.ctx
func (b *Balancer) runHealthcheck(be *backend) {
	defer b.wg.Done()

	maxRetrySleep := 10.0

	for {
		// Retry loop: probe up to MaxRetries+1 times before giving up.
		for attempt := 0; attempt <= b.hc.MaxRetries; attempt++ {
			// Bail out before each probe so Stop() is never blocked waiting
			// for a probe that will be discarded anyway
			select {
			case <-b.ctx.Done():
				return
			default:
			}

			err := b.prober.Probe(b.ctx, be.address)
			if err == nil {
				be.up.Store(true)
				break // healthy, skip remaining entries
			}

			// If the error is due to shutdown, return without mutating state or logging.
			if b.ctx.Err() != nil {
				return
			}

			be.up.Store(false)
			b.logger.Warn("backend unreachable", "address", be.address, "attempt", attempt+1, "error", err)

			// Exponential backoff between retries, capped at maxRetrySleep
			sleep := time.Duration(math.Min(math.Exp(float64(attempt)/2), maxRetrySleep) * float64(time.Second))

			select {
			case <-b.ctx.Done():
				return
			case <-time.After(sleep):
			}
		}

		// Wait for next scheduled probe cycle
		select {
		case <-b.ctx.Done():
			return
		case <-time.After(time.Duration(b.hc.IntervalSeconds) * time.Second):
		}
	}
}

// NewBalancer creates a Balancer and immediately starts one healthcheck goroutine per address.
// Call Stop() to release those goroutines.
func NewBalancer(
	addresses []string,
	hc HealthCheckConfig,
	algorithm Algorithm,
	prober Prober,
	logger *slog.Logger,
) *Balancer {
	ctx, cancel := context.WithCancel(context.Background())

	b := &Balancer{
		hc:        hc,
		algorithm: algorithm,
		prober:    prober,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
	}

	backends := make([]*backend, len(addresses))
	for i, addr := range addresses {
		// Add scheme if missing
		if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
			addr = "http://" + addr
		}
		currBackend := &backend{address: addr}
		currBackend.up.Store(true) // optimistic: assume up until first probe says otherwise
		backends[i] = currBackend
		b.wg.Add(1)
		go b.runHealthcheck(currBackend)
	}
	b.backends = backends
	return b
}

// GetNextBackend returns the address of the next healthy backend according to the configured
// algorithm. It skips unhealthy backends by walking forward through the pool.
// Returns an error if every backend is currently down.
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
