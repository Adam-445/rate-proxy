package main

import (
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type RateLimiter interface {
	Allow(string, time.Time) bool
}

type BackendBalancer interface {
	GetNextBackend() (string, error)
	GetProxy(*url.URL) *httputil.ReverseProxy
}

type ProxyHandler struct {
	limiter  RateLimiter
	balancer BackendBalancer
	logger   *slog.Logger
}

func NewProxyHandler(l RateLimiter, b BackendBalancer, logger *slog.Logger) *ProxyHandler {
	return &ProxyHandler{
		limiter:  l,
		balancer: b,
		logger:   logger,
	}
}

func (h *ProxyHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}

	if !h.limiter.Allow(host, time.Now()) {
		h.logger.Warn("rate limit exceeded", "client", host)
		http.Error(rw, "Request limit reached", http.StatusTooManyRequests)
		return
	}

	backend, err := h.balancer.GetNextBackend()
	if err != nil {
		h.logger.Error("Cannot route request", "error", err)
		http.Error(rw, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	h.logger.Info("proxying request", "client", host, "backend", backend, "path", req.URL.Path)

	target, _ := url.Parse(backend)
	proxy := h.balancer.GetProxy(target)
	proxy.ServeHTTP(rw, req)
}
