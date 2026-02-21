package main

import (
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

type Balancer struct {
	// TODO: skip dead backends / healthchecks
	backends []string // List of backend addresses
	// TODO: Consider making balancing algorithms swappable
	counter atomic.Uint32 // Thread-safe counter
	// TODO: Replace permanent caching to face changes
	// (DNS changes, proxy.Transport settings can change)
	// - TTL
	// - Periodic refreshes
	// - Close old connections...
	proxies sync.Map // Cached proxies
}

func NewBalancer(backendAdresses []string) *Balancer {
	return &Balancer{backends: backendAdresses}
}

func (b *Balancer) GetNextBackend() string {
	n := b.counter.Add(1)
	idx := int(n-1) % len(b.backends)
	return b.backends[idx]
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
