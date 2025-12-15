package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	// Use httputil.ReverseProxy to forward requests from client to process / server
	target := url.URL{
		Scheme: "http",
		Host:   "localhost:8081",
	}
	proxy := httputil.NewSingleHostReverseProxy(&target)

	if err := http.ListenAndServe("localhost:8080", proxy); err != nil {
		return
	}
}
