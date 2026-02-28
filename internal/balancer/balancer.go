// Package balancer implements load balancing across backend servers
// includes proxy caching and health checks
package balancer

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	// TODO: skip dead backends / healthchecks
	backends []*backend // List of backend addresses
	// TODO: Consider making balancing algorithms swappable
	counter atomic.Uint32 // Thread-safe counter
	// TODO: Replace permanent caching to face changes
	// (DNS changes, proxy.Transport settings can change)
	// - TTL
	// - Periodic refreshes
	// - Close old connections...
	proxies sync.Map // Cached proxies
	hc      HealthCheckConfig
	logger  *slog.Logger
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

func NewBalancer(addresses []string, hc HealthCheckConfig, logger *slog.Logger) *Balancer {
	b := &Balancer{hc: hc, logger: logger}
	backends := make([]*backend, len(addresses))
	for i, addr := range addresses {
		// Add scheme if missing
		if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
			addr = "http://" + addr
		}
		currBackend := &backend{address: addr}
		currBackend.up.Store(true)
		backends[i] = currBackend
		go b.runHealthcheck(currBackend)
	}
	b.backends = backends
	return b
}

func (b *Balancer) GetNextBackend() (string, error) {
	for attempts := 0; attempts < len(b.backends); attempts++ {
		n := b.counter.Add(1)
		idx := int(n-1) % len(b.backends)
		backend := b.backends[idx]
		if backend.isUp() {
			return backend.address, nil
		}
	}
	return "", fmt.Errorf("no backends available") // No backends up
}

func (b *Balancer) GetProxy(target *url.URL) *httputil.ReverseProxy {
	key := target.String()
	if val, ok := b.proxies.Load(key); ok {
		return val.(*httputil.ReverseProxy)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	actual, _ := b.proxies.LoadOrStore(key, proxy)
	return actual.(*httputil.ReverseProxy)
}
