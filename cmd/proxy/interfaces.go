package main

import (
	"context"
	"time"
)

type RateLimiter interface {
	Allow(context.Context, string, time.Time) (bool, error)
}

type BackendBalancer interface {
	GetNextBackend() (string, error)
}
