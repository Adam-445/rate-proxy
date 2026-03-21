package proxy

import (
	"net/http"
	"strings"
)

var hopByHop = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func stripHopByHopHeaders(h http.Header) {
	// Iterate through all connection headers
	for _, connHeader := range h.Values("Connection") {
		// Split each individual header by comma
		for _, name := range strings.Split(connHeader, ",") {
			if trimmed := strings.TrimSpace(name); trimmed != "" {
				h.Del(trimmed)
			}
		}
	}

	// Strip the standard hop-by-hop headers
	for _, hk := range hopByHop {
		h.Del(hk)
	}
}
