package balancer

import "testing"

func TestRoundRobin(t *testing.T) {
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

func TestRandomSelection(t *testing.T) {
	rs := &RandomSelection{}
	n := 5
	// Run many times to ensure we never hit an out-of-bounds index
	for range 100 {
		got := rs.Next(n)
		if got < 0 || got >= n {
			t.Errorf("Random index %d out of bounds for n=%d", got, n)
		}
	}
}
