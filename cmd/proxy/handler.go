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
	balancer BackendBalancer
	logger   *slog.Logger
}

func NewProxyHandler(b BackendBalancer, logger *slog.Logger) *ProxyHandler {
	return &ProxyHandler{
		balancer: b,
		logger:   logger,
	}
}

func RateLimitMiddleware(limiter RateLimiter, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !limiter.Allow(host, time.Now()) {
			logger.Warn("Rate limit exceeded", "client", host)
			http.Error(w, "Request limit reached", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *ProxyHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	backend, err := h.balancer.GetNextBackend()
	if err != nil {
		h.logger.Error("Cannot route request", "error", err)
		http.Error(rw, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	h.logger.Info("proxying request", "client", req.RemoteAddr, "backend", backend, "path", req.URL.Path)

	target, _ := url.Parse(backend)
	proxy := h.balancer.GetProxy(target)
	proxy.ServeHTTP(rw, req)
}
