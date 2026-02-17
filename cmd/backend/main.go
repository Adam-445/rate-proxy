package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"instance": port,
			"method":   r.Method,
			"path":     r.URL.Path,
			"header":   r.Header,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
		}
	})

	fmt.Printf("Backend listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		return
	}
}
