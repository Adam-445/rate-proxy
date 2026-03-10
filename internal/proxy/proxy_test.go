package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockTransport allows us to define the backend behavior per test case
type MockTransport func(req *http.Request) (*http.Response, error)

func (m MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func TestServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		target         string
		clientRequest  func() *http.Request
		mockBackend    MockTransport
		expectedStatus int
		expectedBody   string
		verifyRequest  func(t *testing.T, r *http.Request)
	}{
		{
			name:   "Successful forwarding and header stripping",
			target: "http://backend-service",
			clientRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/api/test?foo=bar", nil)
				req.Header.Set("X-Custom", "hello")
				req.Header.Set("Upgrade", "websocket") // Should be stripped
				req.Header.Set("Connection", "Upgrade, X-Delete-Me")
				req.Header.Set("X-Delete-Me", "gone")
				return req
			},
			mockBackend: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("backend-ok")),
					Header:     make(http.Header),
				}, nil
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "backend-ok",
			verifyRequest: func(t *testing.T, r *http.Request) {
				// Verify URL reconstruction
				if r.URL.String() != "http://backend-service/api/test?foo=bar" {
					t.Errorf("URL mismatch, got %s", r.URL.String())
				}
				// Verify header stripping
				if r.Header.Get("Upgrade") != "" || r.Header.Get("X-Delete-Me") != "" {
					t.Error("Hop-by-hop headers were not stripped")
				}
				if r.Header.Get("X-Custom") != "hello" {
					t.Error("Custom header was lost")
				}
			},
		},
		{
			name:   "Backend connection failure",
			target: "http://dead-backend",
			clientRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			mockBackend: func(req *http.Request) (*http.Response, error) {
				return nil, io.ErrUnexpectedEOF // Simulate network drop
			},
			expectedStatus: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			// Create a wrapper for the transport so we can trigger verifyRequest
			transport := MockTransport(func(req *http.Request) (*http.Response, error) {
				res, err := tt.mockBackend(req)

				if tt.verifyRequest != nil {
					tt.verifyRequest(t, req)
				}

				return res, err
			})

			// Inject our mock transport
			rp, err := NewReverseProxy(tt.target, transport, logger)
			if err != nil {
				t.Fatalf("Failed to create proxy: %v", err)
			}

			req := tt.clientRequest()
			rec := httptest.NewRecorder()

			rp.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Status mismatch: got %d, want %d", rec.Code, tt.expectedStatus)
			}

			if tt.expectedBody != "" {
				if rec.Body.String() != tt.expectedBody {
					t.Errorf("Body mismatch: got %q, want %q", rec.Body.String(), tt.expectedBody)
				}
			}
		})
	}
}
