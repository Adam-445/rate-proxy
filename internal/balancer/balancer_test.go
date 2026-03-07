package balancer

import (
	"io"
	"log/slog"
	"strconv"
	"testing"
)

type fakeAlgorithm struct {
	idx int
}

func (f *fakeAlgorithm) Next(n int) int {
	return f.idx
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
			name:            "first backend down",
			startingIdx:     0,
			downBackendsIdx: []int{0},
			backendCount:    3,
			expected:        "http://1",
			expectedErr:     false,
		},
		{
			name:            "multiple backends down skip to last",
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
			name:            "all backends down",
			startingIdx:     0,
			downBackendsIdx: []int{0, 1, 2, 3},
			backendCount:    4,
			expected:        "",
			expectedErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alg := &fakeAlgorithm{idx: tt.startingIdx}
			addrs := make([]string, tt.backendCount)
			for i := range tt.backendCount {
				addrs[i] = "http://" + strconv.Itoa(i)
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			balancer := NewBalancer(addrs, HealthCheckConfig{}, alg, logger)

			for _, idx := range tt.downBackendsIdx {
				balancer.backends[idx].up.Store(false)
			}

			actual, err := balancer.GetNextBackend()
			if (err != nil) != tt.expectedErr {
				t.Fatalf("GetNextBackend() error = %v, expectErr %v", err, tt.expectedErr)
			}

			// if we expected an error we can stop here
			if tt.expectedErr {
				return
			}

			if actual != tt.expected {
				t.Errorf("got %s, want %s", actual, tt.expected)
			}
		})
	}
}
