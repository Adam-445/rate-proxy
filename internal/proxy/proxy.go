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

var hopByHop = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

type ReverseProxy struct {
	targetURL *url.URL
	client    *http.Client
	logger    *slog.Logger
}

func NewReverseProxy(target string, logger *slog.Logger) (*ReverseProxy, error) {
	if logger == nil {
		logger = slog.Default()
	}

	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		// reuse default transport to enable keep-alives
		Transport: http.DefaultTransport,
	}
	return &ReverseProxy{targetURL: u, client: client, logger: logger}, nil
}

func (rp *ReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	partialURL := &url.URL{Path: req.URL.Path, RawQuery: req.URL.RawQuery}
	finalURL := rp.targetURL.ResolveReference(partialURL)

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
	stripHopByHopHeaders(proxyReq.Header, req.Header.Get("Connection"))

	// perseve host
	proxyReq.Host = req.Host

	// X-Forwarded-For
	clientIP, _, _ := net.SplitHostPort(req.RemoteAddr)
	if clientIP == "" {
		clientIP = req.RemoteAddr
	}
	proxyReq.Header.Add("X-Forwarded-For", clientIP)

	// Via header (per RFC: "1.1 proxyname")
	proxyReq.Header.Add("Via", fmt.Sprintf("%d.%d %s", req.ProtoMajor, req.ProtoMinor, "rate-proxy"))

	resp, err := rp.client.Do(proxyReq)
	if err != nil {
		http.Error(rw, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			rp.logger.Warn("Couldnt close response body", "error", err)
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
	stripHopByHopHeaders(rw.Header(), resp.Header.Get("Connection"))

	rw.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(rw, resp.Body); err != nil {
		rp.logger.Error("Error while copying response stream", "error", err)
		return
	}
}

func stripHopByHopHeaders(h http.Header, connectionHeader string) {
	for _, hk := range hopByHop {
		h.Del(hk)
	}

	if connectionHeader == "" {
		return
	}
	for _, name := range strings.Split(connectionHeader, ",") {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}
		h.Del(trimmedName)
	}
}
