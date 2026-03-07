package main

import (
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
