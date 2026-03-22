package limiter

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestAllowsRequestsUpToCapacity(t *testing.T) {
	ctx := context.Background()
	store := &InMemoryBucketStore{data: make(map[string]*bucket)}
	limiter := NewLimiter(store, 3, 1)
	fixedTime := time.Unix(0, 0).UTC()

	for i := range 4 {
		allowed, err := limiter.Allow(ctx, "TestClient", fixedTime)
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
	ctx := context.Background()
	store := &InMemoryBucketStore{data: make(map[string]*bucket)}
	limiter := NewLimiter(store, 3, 1)
	fixedTime := time.Unix(0, 0).UTC()
	requestNumber := 0

	for ; requestNumber < 3; requestNumber++ {
		allowed, err := limiter.Allow(ctx, "TestClient", fixedTime)
		if err != nil {
			t.Fatalf("unexpected error from limiter: %v", err)
		}

		if !allowed {
			t.Errorf("expected request %d to be allowed", requestNumber)
		}
	}

	fixedTime = fixedTime.Add(2 * time.Second)
	for ; requestNumber < 5; requestNumber++ {
		allowed, err := limiter.Allow(ctx, "TestClient", fixedTime)
		if err != nil {
			t.Fatalf("unexpected error from limiter: %v", err)
		}

		if !allowed {
			t.Errorf("expected request %d to be allowed", requestNumber)
		}
	}

	allowed, err := limiter.Allow(ctx, "TestClient", fixedTime)
	if err != nil {
		t.Fatalf("unexpected error from limiter: %v", err)
	}
	requestNumber++
	if allowed {
		t.Errorf("expected request %d to be denied", requestNumber)
	}
}

func TestConcurrency(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryBucketStore()

	bucketCapacity := 900
	l := NewLimiter(store, float64(bucketCapacity), 1)

	clientID := "concurrent-client"
	fixedTime := time.Unix(0, 0).UTC()

	var wg sync.WaitGroup
	requestCount := 1000
	results := make(chan bool, requestCount)

	// Fire requests simultaneously
	for range requestCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _ := l.Allow(ctx, clientID, fixedTime)
			results <- allowed
		}()
	}

	wg.Wait()
	close(results)

	allowedCount := 0
	for res := range results {
		if res {
			allowedCount++
		}
	}

	if allowedCount != bucketCapacity {
		t.Errorf("expected %d allowed requests under concurrency, got %d", bucketCapacity, allowedCount)
	}
}
