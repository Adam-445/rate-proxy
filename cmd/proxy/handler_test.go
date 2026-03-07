package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"
)

type (
	FakeLimiter struct {
		allow bool
	}
	FakeBalancer struct {
		backendAddress string
		error          error
	}
)

func (l *FakeLimiter) Allow(clientID string, now time.Time) bool {
	return l.allow
}

func (b *FakeBalancer) GetNextBackend() (string, error) {
	return b.backendAddress, b.error
}

func (b *FakeBalancer) GetProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	return proxy
}

func TestNormalRequest(t *testing.T) {
	// Normal request -> expected to be handled correctly
	expected := "Hello"
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, expected); err != nil {
			t.Fatalf("error writing response: %s", err)
		}
	}))
	defer svr.Close()

	limiter := &FakeLimiter{allow: true}
	balancer := &FakeBalancer{backendAddress: svr.URL, error: nil}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	proxy := NewProxyHandler(balancer, logger)
	handler := RateLimitMiddleware(limiter, proxy)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	res := recorder.Result()
	defer func() { _ = res.Body.Close() }()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("expected error to be nil. got %v", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("Status code error. got %d, want %d", res.StatusCode, http.StatusOK)
	}
	if string(data) != expected {
		t.Errorf("Result body mismatch. got %s, want %s", string(data), expected)
	}
}

func TestRateLimiting(t *testing.T) {
	// Request is rate limited -> expect 429
	limiter := &FakeLimiter{allow: false}
	balancer := &FakeBalancer{backendAddress: "", error: nil}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	proxy := NewProxyHandler(balancer, logger)
	handler := RateLimitMiddleware(limiter, proxy)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("Status code error. got %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
}

func TestNoBackendsAvailable(t *testing.T) {
	// No backends available -> expect 503
	limiter := &FakeLimiter{allow: true}
	balancer := &FakeBalancer{backendAddress: "", error: errors.New("no backends")}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	proxy := NewProxyHandler(balancer, logger)
	handler := RateLimitMiddleware(limiter, proxy)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("Status code error. got %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}
