package balancer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Prober knows how to check whether a single backend is healthy.
// Returning nil means healthy, any non nil error means unhealthy
//
// The context carries the Balancer's cancellation signal, a Prober implementation must
// respect it so that Stop() can interrupt a running probe immediately rather than waiting
// for a network timeout
type Prober interface {
	Probe(ctx context.Context, address string) error
}

// HTTPProber implements Prober with an HTTP GET request.
// A 2xx response code means healthy. Any network error or non 2xx status means unhealthy
type HTTPProber struct {
	client *http.Client // owns the timeout
	path   string       // eg. "/healthz"
}

// NewHTTPProber creates an HTTPProber that sends GET address+path, expecting a 2xx
// response within timeout.
func NewHTTPProber(path string, timeout time.Duration) *HTTPProber {
	return &HTTPProber{
		client: &http.Client{Timeout: timeout},
		path:   path,
	}
}

// Probe sends GET address+path and returns nil if response status is 2xx.
// The response body is always drained and closed so the underlying TCP connection is
// returned to the pool for reuse.
func (p *HTTPProber) Probe(ctx context.Context, address string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address+p.path, nil)
	if err != nil {
		return err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body) // drain so the connection is reusable

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unhealthy: status %d", resp.StatusCode)
	}
	return nil
}
