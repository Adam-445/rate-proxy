package main

import (
	"log/slog"
	"net/http"
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
	backendAddress, err := h.balancer.GetNextBackend()
	if err != nil {
		h.logger.Error("Cannot route request", "error", err)
		http.Error(rw, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	h.logger.Info("proxying request", "client", req.RemoteAddr, "backend", backendAddress, "path", req.URL.Path)

	proxy := h.balancer.GetProxy(backendAddress)
	proxy.ServeHTTP(rw, req)
}
