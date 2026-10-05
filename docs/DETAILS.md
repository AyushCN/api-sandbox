# Technical Details & API Subsystems

This document explores the lower-level mechanics of the API Sandbox.

## WebSocket Broadcasting Casing

The orchestrator relies heavily on WebSockets to push live state to the frontend (e.g., `file_changed`, `reload_ready`, `status_update`). 

**Technical Detail**: The `BroadcastMessage` struct in the Go Backend does *not* utilize `json:""` tags. 
```go
type BroadcastMessage struct {
	Type      string
	EnvID     string
	UserID    string
	UserName  string
	Data      interface{}
	Timestamp time.Time
}
```
Because of this, the Go JSON serializer defaults to exactly capitalizing the struct fields when emitting data. The frontend and benchmarking scripts **must** expect capitalized keys at the root of the event (e.g., `event["Type"]`), while nested metadata inside the `Data` map remains exactly as it was formulated natively.

## The Orphan Reaper Daemon

Because environments are Docker containers, a process or database failure can leave orphaned containers consuming host resources.

To mitigate this, the backend schedules an Asynq reconciliation task every five minutes:
1. It lists Docker containers with the application labels.
2. It cross-references runtime container names/IDs with environment records in PostgreSQL.
3. It removes orphan containers, records failure for a `RUNNING` environment whose container is missing, and marks stale `BUILDING` records failed.

This provides periodic reconciliation; it does not guarantee immediate recovery if Docker, Redis, or PostgreSQL is unavailable. Failures are logged for diagnosis.

## Database Sidecar Orchestration

Sidecar databases (PostgreSQL, MySQL, MongoDB, Redis) are generated dynamically based on the repository's needs.
1. The backend provisions the primary application container.
2. It detects required database types from the repository.
3. It starts a secondary database container on the environment's organization-specific bridge network (`api-sandbox-net-<organization-id>`).
4. The database credentials are injected into the primary sandbox via environment variables (e.g., `DATABASE_URL=postgres://user:pass@<db-alias>:5432/db`).

Runtime containers are freshly created per environment. The warm pool downloads images and stores image references; it does not reuse running containers. Runtime workspace and package-cache mounts are scoped to the environment. The runtime has no Docker socket and is isolated from Compose's shared `traefik-net`, though outbound internet access is currently available.

## Docker Socket Authority

The backend/worker service has a read/write Docker socket because it performs image pulls and runtime/sidecar container and network lifecycle operations. This is broad, host-root-equivalent authority. Traefik mounts the socket `:ro` for its Docker provider to discover Compose labels; this does not restrict the Docker API to read-only calls. Replacing either access path would require a meaningful change to the current Docker orchestration or routing responsibilities, so deployment must remain limited to a trusted single host.

## Preventing HTTP Socket Leaks

During the rapid iteration of the HTTP Proxy Readiness Polling loop, the Go backend fires thousands of requests to Traefik per second. 
To prevent exhausting the OS socket pool via `TIME_WAIT` states, the backend explicitly:
1. Avoids `defer resp.Body.Close()` inside the `for` loop, opting to close it manually within the loop block.
2. Copies the body to `io.Discard` before closing to ensure the underlying TCP connection can be reliably returned to the HTTP keep-alive connection pool.
