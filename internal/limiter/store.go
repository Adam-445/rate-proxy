package limiter

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Pre-load the Lua script for maximum performance (EVALSHA)
var consumeScript = redis.NewScript(`
	local key = KEYS[1]
	local capacity = tonumber(ARGV[1])
	local rate = tonumber(ARGV[2])
	local now = tonumber(ARGV[3])
	local ttl = tonumber(ARGV[4])

	local tokens = capacity
	local last_ts = now

	-- Read current state using HMGET for Redis Hashes
	local bucket = redis.call("HMGET", key, "tokens", "ts")
	if bucket[1] then
		tokens = tonumber(bucket[1])
		last_ts = tonumber(bucket[2])
	end

	-- Refill logic
	local elapsed = math.max(0, now - last_ts)
	tokens = math.min(capacity, tokens + (elapsed * rate))

	-- Consume logic
	if tokens >= 1 then
		tokens = tokens - 1
		-- Save back to Redis using modern HSET
		redis.call("HSET", key, "tokens", tostring(tokens), "ts", tostring(now))
		redis.call("EXPIRE", key, ttl)
		return 1
	end

	return 0
`)

type bucket struct {
	Tokens    float64   `json:"tokens"`
	Timestamp time.Time `json:"timestamp"`
}

// BucketStore is the persistence layer for the token buckets
type BucketStore interface {
	// Consume attempts to take 1 token. It returns true if allowed, false if rate limited
	Consume(ctx context.Context, key string, capacity float64, rate float64, now time.Time) (bool, error)
}

// RedisBucketStore persists buckets in Redis. Safe for use across multiple proxy instances
// (because the refill and consume logic runs in a lua script, which redis executes single-threaded)
type RedisBucketStore struct {
	client *redis.Client
}

func NewRedisBucketStore(client *redis.Client) *RedisBucketStore {
	return &RedisBucketStore{client: client}
}

func (r *RedisBucketStore) Consume(
	ctx context.Context,
	key string,
	capacity float64,
	rate float64,
	now time.Time,
) (bool, error) {
	// Convert Go time to float seconds so Lua can do the math
	nowFloat := float64(now.UnixNano()) / 1e9
	ttlSeconds := 3600 // 1 hour TTL to clean up inactive clients

	res, err := consumeScript.Run(ctx, r.client, []string{key}, capacity, rate, nowFloat, ttlSeconds).Result()
	if err != nil {
		return false, err
	}

	// Lua script returns 1 for success, 0 for rate limited
	return res.(int64) == 1, nil
}

// InMemoryBucketStore persists buckets in a plain go map behind a mutex.
// Intended for single-instance deployments. Use RedisBucketStore for horizontally
// scaled proxies.
type InMemoryBucketStore struct {
	data map[string]*bucket
	mu   sync.Mutex
}

func NewInMemoryBucketStore() *InMemoryBucketStore {
	return &InMemoryBucketStore{data: make(map[string]*bucket)}
}

func (m *InMemoryBucketStore) Consume(
	ctx context.Context,
	key string,
	capacity float64,
	rate float64,
	now time.Time,
) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.data[key]
	if !ok {
		// First time seeing this client, initialize full bucket
		b = &bucket{
			Tokens:    capacity,
			Timestamp: now,
		}
		m.data[key] = b
	}

	// Refill: add tokens proportional to elapsed time, capped at capacity
	elapsed := now.Sub(b.Timestamp).Seconds()
	newTokens := math.Min(capacity, b.Tokens+(elapsed*rate))

	// Consume
	if newTokens < 1 {
		return false, nil
	}

	b.Tokens = newTokens - 1
	b.Timestamp = now
	return true, nil
}
