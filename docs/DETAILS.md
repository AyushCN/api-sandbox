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

Because sandboxes are physical Docker containers, a crash in the Go backend could result in orphaned containers running infinitely and exhausting host resources. 

To mitigate this, `backend/worker/worker.go` runs a continuous daemon loop:
1. It queries the local Docker socket for all containers labeled with `api-sandbox`.
2. It cross-references the container IDs against the active `environments` table in PostgreSQL.
3. If an environment is marked as `deleted` in the database, or missing entirely, the reaper force-kills and removes the container.

This guarantees that the system state eventually converges with the PostgreSQL source of truth.

## Database Sidecar Orchestration

Sidecar databases (PostgreSQL, MySQL, MongoDB, Redis) are generated dynamically based on the repository's needs.
1. The backend provisions the primary application container.
2. It detects required database types (e.g., scanning `requirements.txt` for `psycopg2`).
3. It spins up a secondary database container connected directly to the specific Organization's isolated Docker network (`api-sandbox-net-<org_id>`).
4. The database credentials are injected into the primary sandbox via environment variables (e.g., `DATABASE_URL=postgres://user:pass@<db-alias>:5432/db`).

## Preventing HTTP Socket Leaks

During the rapid iteration of the HTTP Proxy Readiness Polling loop, the Go backend fires thousands of requests to Traefik per second. 
To prevent exhausting the OS socket pool via `TIME_WAIT` states, the backend explicitly:
1. Avoids `defer resp.Body.Close()` inside the `for` loop, opting to close it manually within the loop block.
2. Copies the body to `io.Discard` before closing to ensure the underlying TCP connection can be reliably returned to the HTTP keep-alive connection pool.
