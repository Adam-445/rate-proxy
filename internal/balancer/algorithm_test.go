package balancer

import "testing"

func TestAlgorithm_RoundRobin(t *testing.T) {
	rr := &RoundRobin{}
	backendCount := 3
	// Call 9 times, should cycle 3 times
	for i := range 9 {
		got := rr.Next(backendCount)
		want := i % backendCount
		if got != want {
			t.Errorf("index %d: got %d, want %d", i, got, want)
		}
	}
}
