package main

import (
	"testing"
	"time"
)

func TestLimiter_AllowsRequestsUpToCapacity(t *testing.T) {
	fixedTime := time.Unix(0, 0).UTC()

	store := &InMemroyBucketStore{data: make(map[string]*bucket)}
	limiter := NewLimiter(store, 3, 1)

	for i := range 4 {
		allowed := limiter.Allow("TestClient", fixedTime)
		if i < 3 && !allowed {
			t.Errorf("expected request %d to be allowed", i)
		}
		if i >= 3 && allowed {
			t.Errorf("expected request %d to be denied", i)
		}
	}
}
