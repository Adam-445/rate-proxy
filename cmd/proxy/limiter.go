package main

import (
	"math"
	"time"
)

type limiter struct {
	store    BucketStore
	capacity float64
	rate     float64
}

func NewLimiter(store BucketStore, maxCapacity float64, tokensPerSecond float64) *limiter {
	return &limiter{store: store, capacity: maxCapacity, rate: tokensPerSecond}
}

func (l *limiter) Allow(clientID string, now time.Time) bool {
	key := "client:" + clientID

	// Get from store
	userBucket, err := l.store.Get(key)
	if err != nil {
		return false
	}

	// Create if it doesnt exist
	if userBucket == nil {
		userBucket = &bucket{
			Tokens:    l.capacity,
			Timestamp: now,
		}
	}

	// Refill
	elapsed := now.Sub(userBucket.Timestamp).Seconds()
	userBucket.Tokens = math.Min(l.capacity, userBucket.Tokens+(elapsed*l.rate))
	userBucket.Timestamp = now

	// Consume
	if userBucket.Tokens >= 1 {
		userBucket.Tokens -= 1
	} else {
		return false
	}

	// Updates timestamp only if consumes token
	_ = l.store.Set(key, userBucket, time.Hour)
	return true
}
