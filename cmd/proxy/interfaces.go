package main

import (
	"time"
)

type RateLimiter interface {
	Allow(string, time.Time) bool
}

type BackendBalancer interface {
	GetNextBackend() (string, error)
}
