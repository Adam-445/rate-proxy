package main

import (
	"net/http/httputil"
	"net/url"
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
	proxies []*httputil.ReverseProxy // List of backend ports
	next    int                      // current port index
}

func NewBalancer(ports []string) *balancer {
	balancer := &balancer{proxies: make([]*httputil.ReverseProxy, len(ports)), next: 0}

	for _, port := range ports {
		target := &url.URL{
			Scheme: "http",
			Host:   ":" + port,
		}
		proxy := httputil.NewSingleHostReverseProxy(target)

		balancer.proxies = append(balancer.proxies, proxy)
	}

	return balancer
}

func (b *balancer) getNextProxy() *httputil.ReverseProxy {
	proxy := b.proxies[b.next]
	b.next = (b.next + 1) % len(b.proxies)
	return proxy
}
