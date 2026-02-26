package main

import (
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

var (
	maxRetries                  = 3
	timeoutSeconds              = 2
	maxRetrySleepDurationSec    = 10
	healthcheckSleepDurationSec = 5
)

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
	logger  *slog.Logger
}

func (b *Balancer) runHealthcheck(backend *backend) {
	client := http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}

	for {
		retries := 0
		for retries <= maxRetries {
			_, err := client.Head(backend.address)
			if err != nil {
				backend.up.Store(false)
				b.logger.Warn("Server is down", "address", backend.address, "retries", retries)
				time.Sleep(time.Duration(math.Min(math.Exp(float64(retries)/2), float64(maxRetrySleepDurationSec))) * time.Second)
				retries++
			} else {
				backend.up.Store(true)
				break
			}
		}
		time.Sleep(time.Duration(healthcheckSleepDurationSec) * time.Second)
	}
}

func NewBalancer(backendAdresses []string, logger *slog.Logger) *Balancer {
	b := &Balancer{logger: logger}
	backends := make([]*backend, len(backendAdresses))
	for i, addr := range backendAdresses {
		// Add scheme if missing
		if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
			addr = "http://" + addr
		}
		currBackend := &backend{address: addr}
		backends[i] = currBackend
		go b.runHealthcheck(currBackend)
	}
	b.backends = backends
	return b
}

func (b *Balancer) GetNextBackend() string {
	for attempts := 0; attempts < len(b.backends); attempts++ {
		n := b.counter.Add(1)
		idx := int(n-1) % len(b.backends)
		backend := b.backends[idx]
		if backend.isUp() {
			return backend.address
		}
	}
	// HACK: should return error
	return "" // No backends up
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
