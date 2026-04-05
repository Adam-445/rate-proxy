package balancer

import (
	"errors"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"
)

type fakeAlgorithm struct {
	idx int
}

func (f *fakeAlgorithm) Next(n int) int {
	return f.idx
}

// newTestBalancer creates a Balancer for testing and registers Stop() as a cleanup function
// so healthcheck goroutines don't leak between tests
func newTestBalancer(t *testing.T, addrs []string, alg Algorithm) *Balancer {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := NewBalancer(addrs, HealthCheckConfig{}, alg, &blockingProber{}, logger)
	t.Cleanup(b.Stop)
	return b
}

func TestDownBackend(t *testing.T) {
	tests := []struct {
		name            string
		startingIdx     int
		downBackendsIdx []int
		backendCount    int
		expected        string
		expectedErr     bool
	}{
		{
			name:            "all backends up",
			startingIdx:     0,
			downBackendsIdx: []int{},
			backendCount:    3,
			expected:        "http://0",
			expectedErr:     false,
		},
		{
			name:            "first backend down, skips to next",
			startingIdx:     0,
			downBackendsIdx: []int{0},
			backendCount:    3,
			expected:        "http://1",
			expectedErr:     false,
		},
		{
			name:            "multiple backends down, skip to last",
			startingIdx:     0,
			downBackendsIdx: []int{0, 1},
			backendCount:    3,
			expected:        "http://2",
			expectedErr:     false,
		},
		{
			name:            "wrap around when at end and current is down",
			startingIdx:     1,
			downBackendsIdx: []int{1},
			backendCount:    2,
			expected:        "http://0",
			expectedErr:     false,
		},
		{
			name:            "all backends down returns error",
			startingIdx:     0,
			downBackendsIdx: []int{0, 1, 2, 3},
			backendCount:    4,
			expected:        "",
			expectedErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addrs := make([]string, tt.backendCount)
			for i := range tt.backendCount {
				addrs[i] = "http://" + strconv.Itoa(i)
			}

			b := newTestBalancer(t, addrs, &fakeAlgorithm{tt.startingIdx})

			for _, idx := range tt.downBackendsIdx {
				b.backends[idx].up.Store(false)
			}

			got, err := b.GetNextBackend()
			if (err != nil) != tt.expectedErr {
				t.Fatalf("GetNextBackend() error = %v, expectErr %v", err, tt.expectedErr)
			}

			// if we expected an error we can stop here
			if tt.expectedErr {
				return
			}

			if got != tt.expected {
				t.Errorf("got %s, want %s", got, tt.expected)
			}
		})
	}
}

// TestHealthCheck_MarksBackendDown verifies that a backend is marked down when
// every probe attempt fails.
func TestHealthCheck_MarksBackendDown(t *testing.T) {
	prober := &fakeProber{}
	prober.setErr(errors.New("connection refused"))

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := NewBalancer(
		[]string{"http://backend-0"},
		HealthCheckConfig{MaxRetries: 0}, // fail after first probe, loop immediately
		&fakeAlgorithm{},
		prober,
		logger,
	)
	t.Cleanup(b.Stop)

	if !waitFor(func() bool { return !b.backends[0].isUp() }, 500*time.Millisecond) {
		t.Error("expected backend to be marked down after probe failure")
	}
}
