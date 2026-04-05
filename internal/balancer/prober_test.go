package balancer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingProber blocks until the context is cancelled. Used in GetNextBackend tests so
// that healthcheck goroutines never mutate backend state during the test, they sit inside
// Probe() the entire time and only return when Stop() cancels the context.
type blockingProber struct{}

func (p *blockingProber) Probe(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

// fakeProber returns a configurable error on every call and counts how many times it
// has been called. Used in healthcheck behavior tests
type fakeProber struct {
	mu         sync.Mutex
	err        error
	probeCount atomic.Int64
}

func (f *fakeProber) setErr(err error) {
	f.mu.Lock()
	f.err = err
	f.mu.Unlock()
}

func (f *fakeProber) Probe(_ context.Context, _ string) error {
	f.probeCount.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// waitFor polls condition every 5 ms until it returns true or timeout elapses.
// Returns true if the condition was met within the deadline
func waitFor(condition func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// TestBackendRecovery verifies that a backend is marked up again
// once probes start succeeding after a period of failure.
func TestBackendRecovery(t *testing.T) {
	prober := &fakeProber{}
	prober.setErr(errors.New("connection refused"))

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := NewBalancer(
		[]string{"http://backend-0"},
		HealthCheckConfig{MaxRetries: 0},
		&fakeAlgorithm{},
		prober,
		logger,
	)
	t.Cleanup(b.Stop)

	// Phase 1: wait for the backend to go down.
	if !waitFor(func() bool { return !b.backends[0].isUp() }, 1*time.Second) {
		t.Fatal("backend did not go down as expected")
	}

	// Phase 2: simulate recovery and wait for the backend to come back up.
	prober.setErr(nil)
	if !waitFor(func() bool { return b.backends[0].isUp() }, 1*time.Second) {
		t.Error("expected backend to recover after probe starts succeeding")
	}
}

// TestStopHaltsProbing verifies that calling Stop() prevents any
// further probes from being issued.
func TestStopHaltsProbing(t *testing.T) {
	prober := &fakeProber{} // returns nil (healthy)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := NewBalancer(
		[]string{"http://backend-0"},
		HealthCheckConfig{MaxRetries: 0},
		&fakeAlgorithm{},
		prober,
		logger,
	)

	// Let the loop spin for a bit to accumulate probe calls.
	time.Sleep(50 * time.Millisecond)
	b.Stop()

	// Give the goroutine time to observe the cancellation and exit.
	time.Sleep(20 * time.Millisecond)
	countAfterStop := prober.probeCount.Load()

	// Wait longer and confirm the count is frozen.
	time.Sleep(50 * time.Millisecond)
	if prober.probeCount.Load() != countAfterStop {
		t.Error("expected probing to stop after Stop() was called, but probeCount kept increasing")
	}
}
