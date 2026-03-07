package main

import (
	"log/slog"
	"net"
	"net/http"
	"time"
)

func RateLimitMiddleware(limiter RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !limiter.Allow(host, time.Now()) {
			http.Error(w, "Request limit reached", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type responseRecorder struct {
	writer http.ResponseWriter
	status int
}

func (r *responseRecorder) Header() http.Header {
	return r.writer.Header()
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.writer.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.writer.Write(b)
}

func LoggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &responseRecorder{writer: w}

		next.ServeHTTP(recorder, r)

		duration := time.Since(start)
		logger.Info("Request processesed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration", duration,
			"ip", r.RemoteAddr)
	})
}
