# rate-proxy

A reverse proxy with per-client token bucket rate limiting and active health-checked load balancing.
 
---

## Architecture

```mermaid
%%{init: {"theme": "base", "themeVariables": {
  "primaryColor":       "#E6F1FB",
  "primaryTextColor":   "#0C447C",
  "primaryBorderColor": "#185FA5",
  "lineColor":          "#888780",
  "clusterBkg":         "#FFFBF0",
  "clusterBorder":      "#BA7517"
}}}%%
flowchart LR
  classDef gray fill:#F1EFE8,stroke:#5F5E5A,color:#444441
  classDef blue fill:#E6F1FB,stroke:#185FA5,color:#0C447C
  classDef teal fill:#E1F5EE,stroke:#0F6E56,color:#085041

  Client:::gray

  Client --> LOG["Logging<br/>time &amp; status"]:::blue
  LOG    --> RL["Rate Limit<br/>token bucket"]:::blue
  RL     --> PH["Proxy Handler<br/>header rewrite"]:::blue

  subgraph pool["backend pool  ·  ↺ health probe"]
    BE1[":8081"]:::gray
    BE2[":8082"]:::gray
    BEN["· · ·"]:::gray
  end

  PH --> BE1 & BE2 & BEN

  RL -. "Consume()" .-> BS["BucketStore<br/>in-memory · redis"]:::teal

  style pool stroke-dasharray:5 5,color:#6B4C0B
```

A request enters the logging middleware, which starts a timer and records the final status code. It passes to the rate limiter, which checks the client's token bucket by IP and either allows or rejects the request before any backend is touched. Allowed requests reach `ProxyHandler`, which asks the balancer for a healthy backend, retrieves or creates a cached `ReverseProxy` for that address, rewrites the request headers, and streams the response back. Independently, one goroutine per backend runs a continuous health check loop with exponential backoff, marking backends up or down via an atomic bool.

## How to Run
 
**Prerequisites:** Go 1.21+. For Redis-backed rate limiting: Docker.
 
```bash
# Clone and run with in-memory store (default)
git clone https://github.com/Adam-445/rate-proxy
cd rate-proxy
 
# Start three backends in separate terminals
PORT=8081 go run ./cmd/backend &
PORT=8082 go run ./cmd/backend &
PORT=8083 go run ./cmd/backend &
 
# Start the proxy
go run ./cmd/proxy --config config.json
 
# Test it
curl http://localhost:8080/hello
 
# For Redis-backed rate limiting across multiple proxy instances:
docker compose up -d
# then set storage.type = "redis" in config.json
```
 
Run the tests:
```bash
go test ./...
```

