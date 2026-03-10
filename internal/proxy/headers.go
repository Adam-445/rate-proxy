package proxy

import (
	"net/http"
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

func stripHopByHopHeaders(h http.Header) {
	if connHeader := h.Get("Connection"); connHeader != "" {
		for _, name := range strings.Split(connHeader, ",") {
			if trimmed := strings.TrimSpace(name); trimmed != "" {
				h.Del(trimmed)
			}
		}
	}

	for _, hk := range hopByHop {
		h.Del(hk)
	}
}
