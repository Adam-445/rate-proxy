package main

import (
	"log/slog"
	"net/http"
	"net/url"
)

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
