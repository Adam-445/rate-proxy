package main

import (
	"io"
	"log/slog"
	"testing"
)

func TestBalancer_RoundRobin(t *testing.T) {
	backends := []string{
		"localhost:8081",
		"localhost:8082",
		"localhost:8083",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := NewBalancer(backends, logger)

	// Call 9 times, should cycle 3 times
	for i := range 9 {
		got := b.GetNextBackend()
		want := backends[i%3]
		if got != want {
			t.Errorf("request %d: got %s, want %s", i, got, want)
		}
	}
}
