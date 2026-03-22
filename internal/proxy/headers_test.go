package proxy

import (
	"net/http"
	"testing"
)

func TestStripHopByHopHeaders(t *testing.T) {
	h := http.Header{}

	// Standard hop-by-hop headers
	h.Add("Keep-Alive", "timeout=5, max=100")
	h.Add("Upgrade", "websocket")

	// A standard header (we want to keep this one)
	h.Add("Content-Type", "application/json")

	// A custom header listed in Connection must be stripped
	h.Add("Connection", "Upgrade, X-Internal-Routing")
	h.Add("X-Internal-Routing", "node-55")

	stripHopByHopHeaders(h)

	// Check standard hop-by-hop headers removal
	if h.Get("Keep-Alive") != "" {
		t.Error("Keep-Alive was not stripped")
	}
	if h.Get("Upgrade") != "" {
		t.Error("Upgrade was not stripped")
	}

	// Check dynamic Connection-based removals
	if h.Get("X-Internal-Routing") != "" {
		t.Error("Custom header defined in Connection was not stripped")
	}
	if h.Get("Connection") != "" {
		t.Error("Connection header itself was not stripped")
	}

	// Check preservation of safe headers
	if h.Get("Content-Type") != "application/json" {
		t.Error("Content-Type was improperly stripped or modified")
	}
}
