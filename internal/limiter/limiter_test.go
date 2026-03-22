package limiter

import (
	"testing"
	"time"
)

func TestAllowsRequestsUpToCapacity(t *testing.T) {
	store := &InMemoryBucketStore{data: make(map[string]*bucket)}
	limiter := NewLimiter(store, 3, 1)
	fixedTime := time.Unix(0, 0).UTC()

	for i := range 4 {
		allowed, err := limiter.Allow("TestClient", fixedTime)
		if err != nil {
			t.Fatalf("unexpected error from limiter: %v", err)
		}
		if i < 3 && !allowed {
			t.Errorf("expected request %d to be allowed", i)
		}
		if i >= 3 && allowed {
			t.Errorf("expected request %d to be denied", i)
		}
	}
}

func TestRefillsTokensOverTime(t *testing.T) {
	store := &InMemoryBucketStore{data: make(map[string]*bucket)}
	limiter := NewLimiter(store, 3, 1)
	fixedTime := time.Unix(0, 0).UTC()
	requestNumber := 0

	for ; requestNumber < 3; requestNumber++ {
		allowed, err := limiter.Allow("TestClient", fixedTime)
		if err != nil {
			t.Fatalf("unexpected error from limiter: %v", err)
		}

		if !allowed {
			t.Errorf("expected request %d to be allowed", requestNumber)
		}
	}

	fixedTime = fixedTime.Add(2 * time.Second)
	for ; requestNumber < 5; requestNumber++ {
		allowed, err := limiter.Allow("TestClient", fixedTime)
		if err != nil {
			t.Fatalf("unexpected error from limiter: %v", err)
		}

		if !allowed {
			t.Errorf("expected request %d to be allowed", requestNumber)
		}
	}

	allowed, err := limiter.Allow("TestClient", fixedTime)
	if err != nil {
		t.Fatalf("unexpected error from limiter: %v", err)
	}
	requestNumber++
	if allowed {
		t.Errorf("expected request %d to be denied", requestNumber)
	}
}
