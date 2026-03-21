package main

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/Adam-445/rate-proxy/internal/proxy"
)

type ProxyHandler struct {
	balancer BackendBalancer
	logger   *slog.Logger
	proxies  sync.Map // cached proxies
}

func NewProxyHandler(b BackendBalancer, logger *slog.Logger) *ProxyHandler {
	return &ProxyHandler{
		balancer: b,
		logger:   logger,
	}
}

func (h *ProxyHandler) getProxy(target string) (http.Handler, error) {
	if val, ok := h.proxies.Load(target); ok {
		return val.(*proxy.ReverseProxy), nil
	}

	p, err := proxy.NewReverseProxy(target, nil, h.logger)
	if err != nil {
		h.logger.Error("Failed to create proxy", "target", target, "error", err)
		return nil, err
	}
	actual, _ := h.proxies.LoadOrStore(target, p)
	return actual.(*proxy.ReverseProxy), nil
}

func (h *ProxyHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	backendAddress, err := h.balancer.GetNextBackend()
	if err != nil {
		h.logger.Error("Cannot route request", "error", err)
		http.Error(rw, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	h.logger.Info("proxying request", "client", req.RemoteAddr, "backend", backendAddress, "path", req.URL.Path)

	proxy, err := h.getProxy(backendAddress)
	if err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	proxy.ServeHTTP(rw, req)
}
