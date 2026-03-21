// Package proxy provides a transparent HTTP reverse proxy
package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
)

type ReverseProxy struct {
	target    *url.URL
	transport http.RoundTripper
	logger    *slog.Logger
}

func NewReverseProxy(target string, transport http.RoundTripper, logger *slog.Logger) (*ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = slog.Default()
	}

	if transport == nil {
		transport = http.DefaultTransport
	}
	return &ReverseProxy{target: u, transport: transport, logger: logger}, nil
}

func (rp *ReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	partialURL := &url.URL{Path: req.URL.Path, RawQuery: req.URL.RawQuery}
	finalURL := rp.target.ResolveReference(partialURL)

	proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, finalURL.String(), req.Body)
	if err != nil {
		http.Error(rw, "Failed to forward", http.StatusBadGateway)
		return
	}

	// copy all headers
	for k, vals := range req.Header {
		for _, v := range vals {
			proxyReq.Header.Add(k, v)
		}
	}

	// remove hop-by-hop
	stripHopByHopHeaders(proxyReq.Header)

	// preserve host
	proxyReq.Host = req.Host

	// X-Forwarded-For
	clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		clientIP = req.RemoteAddr
	}

	// Get the original XFF from the incoming request
	if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
		clientIP = prior + ", " + clientIP
	}

	proxyReq.Header.Set("X-Forwarded-For", clientIP)

	// Via header (per RFC: "1.1 proxyname")
	proxyReq.Header.Add("Via", fmt.Sprintf("%d.%d %s", req.ProtoMajor, req.ProtoMinor, "rate-proxy"))

	resp, err := rp.transport.RoundTrip(proxyReq)
	if err != nil {
		rp.logger.Error("Backend request failed", "target", rp.target.String(), "error", err)
		http.Error(rw, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			rp.logger.Warn("Couldn't close response body", "error", err)
			return
		}
	}()

	// copy response headers
	for k, vals := range resp.Header {
		for _, v := range vals {
			rw.Header().Add(k, v)
		}
	}

	// strip hop-by-hop headers from response
	stripHopByHopHeaders(rw.Header())

	rw.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(rw, resp.Body); err != nil {
		rp.logger.Error("Error while copying response stream", "error", err)
		return
	}
}
