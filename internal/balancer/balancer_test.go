package balancer

import (
	"io"
	"log/slog"
	"testing"
)

func TestBalancer_RoundRobin(t *testing.T) {
	backends := []string{
		"http://localhost:8081",
		"http://localhost:8082",
		"http://localhost:8083",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := NewBalancer(backends, logger)

	// Call 9 times, should cycle 3 times
	for i := range 9 {
		got, err := b.GetNextBackend()
		if err != nil {
			t.Fatalf("b.GetNextBackend() returned an error: %s", err)
		}
		want := backends[i%3]
		if got != want {
			t.Errorf("request %d: got %s, want %s", i, got, want)
		}
	}
}
