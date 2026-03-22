// Package proxy provides a transparent HTTP reverse proxy
package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
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

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid proxy target URL scheme %q: only http and https are supported", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid proxy target URL %q: host is empty", target)
	}

	if logger == nil {
		logger = slog.Default()
	}

	if transport == nil {
		transport = http.DefaultTransport
	}
	return &ReverseProxy{target: u, transport: transport, logger: logger}, nil
}

func buildBackendURL(target *url.URL, req *http.Request) url.URL {
	// Create a copy of the target URL so we don't mutate the original
	finalURL := *target

	// We manually join target.Path and request path instead of using ResolveReference here because
	// ResolveReference replaces the base path when the request path starts with '/',
	// which would drop any prefix configured on the backend (eg. "/api/v1").
	targetPath := strings.TrimSuffix(finalURL.Path, "/")
	reqPath := strings.TrimPrefix(req.URL.Path, "/")
	finalURL.Path = targetPath + "/" + reqPath

	// Merge query parameters if necessary
	if finalURL.RawQuery == "" || req.URL.RawQuery == "" {
		finalURL.RawQuery = finalURL.RawQuery + req.URL.RawQuery
	} else {
		finalURL.RawQuery = finalURL.RawQuery + "&" + req.URL.RawQuery
	}

	return finalURL
}

func (rp *ReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	finalURL := buildBackendURL(rp.target, req)
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

	// preserve original host in X-Forwarded-Host; use backend host for request Host
	proxyReq.Header.Set("X-Forwarded-Host", req.Host)

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
