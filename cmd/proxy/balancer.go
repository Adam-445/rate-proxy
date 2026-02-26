package main

import (
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

type backend struct {
	address string
	flag    bool
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
}

func NewBalancer(backendAdresses []string) *Balancer {
	backends := make([]*backend, len(backendAdresses))
	for i, addr := range backendAdresses {
		backends[i] = &backend{
			address: addr,
			flag:    false,
		}
	}
	return &Balancer{backends: backends}
}

func (b *Balancer) GetNextBackend() string {
	n := b.counter.Add(1)
	idx := int(n-1) % len(b.backends)
	return b.backends[idx].address
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
