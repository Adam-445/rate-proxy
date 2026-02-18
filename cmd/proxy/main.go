package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

type RedisBucketStore struct {
	client *redis.Client
}

func (r *RedisBucketStore) Get(key string) (*bucket, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}

	var b bucket
	if err := json.Unmarshal(val, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *RedisBucketStore) Set(key string, b *bucket, ttl time.Duration) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, data, ttl).Err()
}

type bucket struct {
	Tokens    float64   `json:"tokens"`
	Timestamp time.Time `json:"timestamp"`
}

type InMemroyBucketStore struct {
	data map[string]*bucket
	mu   sync.Mutex
}

func (m *InMemroyBucketStore) Get(key string) (*bucket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.data[key]
	if !ok {
		return nil, nil
	}
	return b, nil
}

func (m *InMemroyBucketStore) Set(key string, b *bucket, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = b
	return nil
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

func joinURLPath(a, b *url.URL) (path, rawpath string) {
	if a.RawPath == "" && b.RawPath == "" {
		return singleJoiningSlash(a.Path, b.Path), ""
	}
	// Same as singleJoiningSlash, but uses EscapedPath to determine
	// whether a slash should be added
	apath := a.EscapedPath()
	bpath := b.EscapedPath()

	aslash := strings.HasSuffix(apath, "/")
	bslash := strings.HasPrefix(bpath, "/")

	switch {
	case aslash && bslash:
		return a.Path + b.Path[1:], apath + bpath[1:]
	case !aslash && !bslash:
		return a.Path + "/" + b.Path, apath + "/" + bpath
	}
	return a.Path + b.Path, apath + bpath
}

var (
	currentIdx = 0
	ports      = []string{"8081", "8082", "8083"}
)

func rewriteRequestURL(req *http.Request, target *url.URL) {
	targetQuery := target.RawQuery
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host + ports[currentIdx]
	currentIdx = (currentIdx + 1) % len(ports)
	req.URL.Path, req.URL.RawPath = joinURLPath(target, req.URL)
	if targetQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = targetQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
	}
}

func main() {
	// Use httputil.ReverseProxy to forward requests from client to process / server
	target := &url.URL{
		Scheme: "http",
		Host:   "localhost:",
	}
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if _, err := client.Ping(ctx).Result(); err != nil {
		panic(err)
	}
	redisStore := &RedisBucketStore{client: client}
	limiter := NewLimiter(redisStore, 10, 1)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		rewriteRequestURL(req, target)
	}

	// balancer := NewBalancer([]string{"8081", "8082", "8083"})

	// Create a handler that wraps the proxy
	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			host = req.RemoteAddr
		}
		if !limiter.Allow(host, time.Now()) {
			http.Error(rw, "Request limit reached", http.StatusTooManyRequests)
			return
		}

		// Original proxy's ServeHTTP method
		proxy.ServeHTTP(rw, req)
	})

	if err := http.ListenAndServe("localhost:8080", handler); err != nil {
		return
	}
}
