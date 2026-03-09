package main

import (
	"time"

	"github.com/Adam-445/rate-proxy/internal/proxy"
)

type RateLimiter interface {
	Allow(string, time.Time) bool
}

type BackendBalancer interface {
	GetNextBackend() (string, error)
	GetProxy(string) *proxy.ReverseProxy
}
