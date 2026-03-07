package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseRecorder(t *testing.T) {
	tests := []struct {
		name           string
		handler        func(w http.ResponseWriter)
		expectedStatus int
	}{
		{
			name: "Excplicit 400 status",
			handler: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusBadRequest)
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Implicit 200 via Write",
			handler: func(w http.ResponseWriter) {
				if _, err := w.Write([]byte("Hello")); err != nil {
					t.Errorf("error writing: %v", err)
				}
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "No calls to writer",
			handler: func(w http.ResponseWriter) {
				// empty
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rr := &responseRecorder{ResponseWriter: rec, status: http.StatusOK}

			tt.handler(rr)

			if rr.status != tt.expectedStatus {
				t.Errorf("got status %d, want %d", rr.status, tt.expectedStatus)
			}
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	handler := LoggingMiddleware(logger, next)

	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("Middleware did not pass through the correct status: got %d, want %d", rec.Code, http.StatusAccepted)
	}
}
