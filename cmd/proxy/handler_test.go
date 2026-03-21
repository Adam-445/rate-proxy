package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type (
	FakeLimiter struct {
		allow bool
		err   error
	}
	FakeBalancer struct {
		backendAddress string
		err            error
	}
)

func (l *FakeLimiter) Allow(clientID string, now time.Time) (bool, error) {
	return l.allow, l.err
}

func (b *FakeBalancer) GetNextBackend() (string, error) {
	return b.backendAddress, b.err
}

func TestProxyMiddleware_Scenarios(t *testing.T) {
	// Spin up a single fake backend for the success cases to route to
	expectedBody := "Hello"
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, expectedBody); err != nil {
			t.Fatalf("error writing response: %s", err)
		}
	}))
	defer svr.Close()

	tests := []struct {
		name           string
		limiterAllow   bool
		limiterErr     error
		balancerAddr   string
		balancerErr    error
		expectedStatus int
	}{
		{
			name:           "Normal Request Success",
			limiterAllow:   true,
			limiterErr:     nil,
			balancerAddr:   svr.URL,
			balancerErr:    nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Rate Limit Exceeded",
			limiterAllow:   false,
			limiterErr:     nil,
			balancerAddr:   svr.URL, // Balancer shouldn't even be reached
			balancerErr:    nil,
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "Limiter Internal Error",
			limiterAllow:   false,
			limiterErr:     errors.New("redis connection timeout"),
			balancerAddr:   svr.URL,
			balancerErr:    nil,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "No Backends Available",
			limiterAllow:   true,
			limiterErr:     nil,
			balancerAddr:   "",
			balancerErr:    errors.New("all backends down"),
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup the fakes based on the table row
			limiter := &FakeLimiter{allow: tt.limiterAllow, err: tt.limiterErr}
			balancer := &FakeBalancer{backendAddress: tt.balancerAddr, err: tt.balancerErr}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			// Build the handler chain
			proxy := NewProxyHandler(balancer, logger)
			handler := RateLimitMiddleware(limiter, proxy)

			// Execute the request
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			// Assert
			if recorder.Code != tt.expectedStatus {
				t.Errorf("got status %d, want %d", recorder.Code, tt.expectedStatus)
			}

			// If it's the success case, also verify the body passed through
			if tt.expectedStatus == http.StatusOK {
				body, _ := io.ReadAll(recorder.Body)
				if string(body) != expectedBody {
					t.Errorf("Result body mismatch. got %s, want %s", string(body), expectedBody)
				}
			}
		})
	}
}
