package main

import (
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"
)

type ProxyHandler struct {
	limiter  *Limiter
	balancer *Balancer
	logger   *slog.Logger
}

func NewProxyHandler(l *Limiter, b *Balancer, logger *slog.Logger) *ProxyHandler {
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
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	h.logger.Info("proxying request", "client", host, "backend", backend, "path", req.URL.Path)

	target, _ := url.Parse(backend)
	proxy := h.balancer.GetProxy(target)
	proxy.ServeHTTP(rw, req)
}
