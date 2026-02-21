package main

import (
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

// TODO:
// - Given a list of backends, pick the next one.

// A balancer needs:
// - A list of backends
// - Which one to pick next
// A balancer:
// - Returns the next backend
// (- Eventually skip dead backends)

type balancer struct {
	backends []string                          // List of backend addresses
	counter  atomic.Uint32                     // Thread-safe counter
	proxies  map[string]*httputil.ReverseProxy // Cached proxies
}

func NewBalancer(backendAdresses []string) *balancer {
	return &balancer{backends: backendAdresses, proxies: make(map[string]*httputil.ReverseProxy)}
}

func (b *balancer) GetNextBackend() string {
	n := b.counter.Add(1)
	idx := int(n-1) % len(b.backends)
	return b.backends[idx]
}

func (b *balancer) GetProxy(target *url.URL) *httputil.ReverseProxy {
	proxy, ok := b.proxies[target.Host]
	if !ok {
		proxy = httputil.NewSingleHostReverseProxy(target)
		b.proxies[target.Host] = proxy
	}
	return proxy
}
