// Package limiter implements a token bucket rate limiter,
// supports both in-memory and Redis-backed storage
package limiter

import (
	"context"
	"time"
)

type Limiter struct {
	store    BucketStore
	capacity float64
	rate     float64
}

func NewLimiter(store BucketStore, maxCapacity float64, tokensPerSecond float64) *Limiter {
	return &Limiter{store: store, capacity: maxCapacity, rate: tokensPerSecond}
}

func (l *Limiter) Allow(ctx context.Context, clientID string, now time.Time) (bool, error) {
	key := "client:" + clientID
	return l.store.Consume(ctx, key, l.capacity, l.rate, now)
}
