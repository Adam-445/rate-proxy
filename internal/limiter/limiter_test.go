package limiter

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestLimiter_AllowsRequestsUpToCapacity(t *testing.T) {
	store := &InMemoryBucketStore{data: make(map[string]*bucket)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	limiter := NewLimiter(store, 3, 1, logger)
	fixedTime := time.Unix(0, 0).UTC()

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

func TestLimiter_RefillsTokensOverTime(t *testing.T) {
	store := &InMemoryBucketStore{data: make(map[string]*bucket)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	limiter := NewLimiter(store, 3, 1, logger)
	fixedTime := time.Unix(0, 0).UTC()
	requestNumber := 0

	for ; requestNumber < 3; requestNumber++ {
		allowed := limiter.Allow("TestClient", fixedTime)

		if !allowed {
			t.Errorf("expected request %d to be allowed", requestNumber)
		}
	}

	fixedTime = fixedTime.Add(2 * time.Second)
	for ; requestNumber < 5; requestNumber++ {
		allowed := limiter.Allow("TestClient", fixedTime)

		if !allowed {
			t.Errorf("expected request %d to be allowed", requestNumber)
		}
	}

	allowed := limiter.Allow("TestClient", fixedTime)
	requestNumber++
	if allowed {
		t.Errorf("expected request %d to be denied", requestNumber)
	}
}
