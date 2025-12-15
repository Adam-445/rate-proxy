package main

import (
	"fmt"
	"net/http"
	"time"
)

func greet(w http.ResponseWriter, r *http.Request) {
	if _, err := fmt.Fprintf(w, "Hello World! %s", time.Now()); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/", greet)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		return
	}
}
