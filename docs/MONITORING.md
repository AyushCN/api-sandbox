# Monitoring & Observability

Because the API Sandbox orchestrates dynamic containers on a single host, standard monitoring tools are essential for tracing errors and identifying misbehaving user sandboxes.

## Accessing Logs

### 1. Go Backend (Orchestrator)
The backend and Asynq worker share the backend service, which manages the database, Docker socket, and WebSocket hub. It uses structured `slog` JSON logging.
```bash
docker logs -f api-sandbox-backend
```
Look for cleanup/provisioning failures and container inspect/start/stop errors. A `touch-on-save failed` warning can indicate the runtime container or its watcher is unavailable.

### 2. Traefik Proxy (Routing)
Traefik handles all ingress for both the API and the user sandboxes.
```bash
docker logs -f api-sandbox-traefik
```
Traefik uses both Docker provider discovery for Compose labels and Redis provider configuration for environment preview routes. Its Docker socket mount is filesystem read-only only; the Docker API itself is not read-only. The dashboard is disabled unless `TRAEFIK_DASHBOARD=true`.

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
An Asynq task runs orphan reconciliation every five minutes. It compares labeled Docker containers with environment records, removes orphan runtimes, and records failure when a database `RUNNING` environment has no container or a `BUILDING` environment is stale. The reaper logs failures; check backend logs before manually removing containers.

You can manually inspect the state by checking the DB:
```sql
SELECT id, status, port FROM environments;
```

Inspect mounts and network attachments for a runtime container with:

```bash
docker inspect api-sandbox-env-<environment-id> --format '{{json .Mounts}}'
docker inspect api-sandbox-env-<environment-id> --format '{{json .NetworkSettings.Networks}}'
```

## Known Bottlenecks
1. **Inotify Limits**: The platform uses bind mounts heavily. If you provision hundreds of node environments, the host may exhaust its inotify watchers. 
   - *Fix:* `sysctl fs.inotify.max_user_watches=524288`
2. **Socket Exhaustion**: If the HTTP Readiness polling loop is misconfigured to leak response bodies, the Go backend will exhaust host TCP ports. Ensure `resp.Body.Close()` is aggressively utilized.
