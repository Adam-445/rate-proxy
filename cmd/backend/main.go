package main

import (
	"encoding/json"
	"net/http"
)

func handleRequest(w http.ResponseWriter, r *http.Request) {
	// Echo back request info
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"method":  r.Method,
		"path":    r.URL.Path,
		"headers": r.Header,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/", handleRequest)
	if err := http.ListenAndServe(":8081", nil); err != nil {
		return
	}
}
