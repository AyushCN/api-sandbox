# Monitoring & Observability

Because the API Sandbox orchestrates dynamic containers on a single host, standard monitoring tools are essential for tracing errors and identifying misbehaving user sandboxes.

## Accessing Logs

### 1. Go Backend (Orchestrator)
The backend manages the database, the Docker socket, and the WebSocket hub. It uses structured `slog` JSON logging.
```bash
docker logs -f api-sandbox-backend
```
Look out for `"level":"WARN"` entries related to `"touch-on-save failed"`, which indicate a sandbox container has crashed or `Air` / `nodemon` has died.

### 2. Traefik Proxy (Routing)
Traefik handles all ingress for both the API and the user sandboxes.
```bash
docker logs -f api-sandbox-traefik
```
You can view the live Traefik dashboard by exposing port 8080 (if enabled in `docker-compose.yml`) to visualize the dynamic routers being created and destroyed as sandboxes spin up and down.

### 3. Ephemeral Sandboxes (User Environments)
User sandboxes are prefixed with `api-sandbox-env-`.
```bash
docker ps | grep api-sandbox-env-
docker logs -f <container_id>
```

## System Telemetry & Metrics

### The `measure_loop` Tool
You can run the built-in benchmark locally at any time to verify the latency of your host machine's I/O and process watchers.
```bash
export JWT_SECRET=$(grep JWT_SECRET .env | cut -d= -f2)
cd backend
go run scripts/measure_loop/main.go
```

### PostgreSQL Status Sync
The orchestrator maintains a tight loop syncing Docker socket states with the `environments` table in PostgreSQL. If an environment is listed as `RUNNING` in the database but `docker ps` shows it is absent, the backend Orphan Reaper will attempt to reconcile the state. 

You can manually inspect the state by checking the DB:
```sql
SELECT id, status, port FROM environments;
```

## Known Bottlenecks
1. **Inotify Limits**: The platform uses bind mounts heavily. If you provision hundreds of node environments, the host may exhaust its inotify watchers. 
   - *Fix:* `sysctl fs.inotify.max_user_watches=524288`
2. **Socket Exhaustion**: If the HTTP Readiness polling loop is misconfigured to leak response bodies, the Go backend will exhaust host TCP ports. Ensure `resp.Body.Close()` is aggressively utilized.
