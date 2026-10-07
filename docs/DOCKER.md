# Docker Operations and Verification

This document describes the Docker behavior implemented by the backend and the local verification performed for it. The platform is a trusted-user, single-host application; it is not a hostile multi-tenant security boundary.

## Socket authority

| Component | Socket access | Operations / reason | Assessment |
|---|---|---|---|
| Backend and Asynq worker | Read/write `/var/run/docker.sock` | Pull images; create/start/inspect/exec/log/stop/restart/remove runtime and database containers; list/create/inspect/connect/remove networks | Required by the current single-process Docker provisioner; effectively host-root authority |
| Traefik in `docker-compose.yml` and `docker-compose.dev.yml` | Socket mounted `:ro` | Docker provider discovers Compose services and labels for routing | `:ro` is filesystem-only; Docker API authority remains broad. Removing it requires replacing Docker provider discovery and label-driven routing |
| Runtime containers | None | No control-plane Docker operations | Socket is not mounted |
| Frontend, PostgreSQL, Redis | None | No Docker control-plane operations | Socket is not mounted |

API and worker responsibilities cannot currently be separated by socket permissions because they run in the same backend service and both need Docker lifecycle work. Introducing a Docker API proxy would add another security-sensitive service; the implementation does not claim that a read-only socket or simple mount option limits API authority.

## Runtime container model

For environment `E`, Docker receives these bind mounts:

| Daemon-host source | Container destination | Mode | Purpose |
|---|---|---|---|
| `${HOST_WORKSPACES_DIR}/E` | `/app` | Read/write | Environment source tree |
| `${HOST_WORKSPACES_DIR}/.cache/E/npm` | `/root/.npm` | Read/write | npm cache |
| `${HOST_WORKSPACES_DIR}/.cache/E/pnpm` | `/root/.local/share/pnpm/store` | Read/write | pnpm cache |
| `${HOST_WORKSPACES_DIR}/.cache/E/pip` | `/root/.cache/pip` | Read/write | pip cache |
| `${HOST_WORKSPACES_DIR}/.cache/E/go` | `/go/pkg/mod` | Read/write | Go module cache |

These are the runtime bind mounts; there is no shared workspace mount, Docker socket, or runtime Docker volume. The backend itself mounts the host workspace root at `/app/workspaces` for trusted control-plane operations. When `HOST_CACHE_DIR` is explicitly configured, provisioning uses that configured cache root instead of the default under `HOST_WORKSPACES_DIR`.

Runtime defaults are `node:20-alpine`, `python:3.11-slim`, and `golang:alpine` (repositories can select custom images). The current images default to UID 0; the backend does not force a non-root user. No compatibility validation has established that arbitrary repositories can install packages and run with a forced non-root UID, so the deployment preserves the image default.

Docker host configuration for production runtime containers:

| Setting | Value |
|---|---|
| Memory / memory+swap | 512 MiB / 512 MiB |
| CPU | 1 CPU (`CPUQuota=100000`, `CPUPeriod=100000`) |
| PIDs | 256 |
| Capabilities | `CapDrop: ALL` |
| Security option | `no-new-privileges:true` |
| Privileged, host PID/network/IPC, devices | Disabled / unset / none |
| Network attachment | User-scoped bridge (`api-sandbox-net-<user-id>`) |

The direct Docker shell integration script uses `NanoCpus=1000000000` for its one-CPU fixture. Production container creation expresses the same CPU allowance using quota/period, so `NanoCpus` is zero in inspect output for production containers; inspect `CpuQuota` and `CpuPeriod` there.

## Network behavior

Compose infrastructure services share `traefik-net`: backend/worker, PostgreSQL, Redis, Traefik, and frontend. Each runtime container attaches to a user-scoped bridge with its optional database sidecar and Traefik. Environments belonging to the same user share that bridge and can reach each other's network endpoints. Runtime is not attached to `traefik-net` and has no shared Redis/PostgreSQL service route. Local real-container probes confirmed shared Redis DNS/connectivity and host-gateway port 80 were unreachable, while outbound HTTP succeeded. Internet egress supports GitHub/repository operations, package installation, dependency downloads, and projects that call external APIs; it remains enabled.

The local verification did not establish connectivity to every arbitrary backend/host port or every external API. Network claims should be scoped to the specific tested addresses and ports.

## Image warm pool and lifecycle

The image pool pulls runtime images and stores image references in Redis. Provisioning may consume a matching reference, but always creates a fresh runtime container with new environment mounts. Startup drains old `api-sandbox-warm-*` containers and clears old Redis warm-container IDs. Environment deletion removes the runtime and sidecar containers, workspace, and scoped cache; cleanup errors are returned/logged instead of reporting successful deletion.

An Asynq task reconciles Docker state every minute. It removes containers whose environment is absent or terminal; marks a `RUNNING` environment failed when its runtime is missing or stopped; and marks `BUILDING` records failed after 60 minutes without a successful build. Staleness is based on database `UpdatedAt`, refreshed when a worker claims a build, rather than container creation time. Reconciliation errors are returned and logged. Docker removal-failure injection and every database/Docker race have not been exercised live.

`healthCheckType: "http"` (the web-runtime default) requires a 2xx preview response; redirects are not followed and 3xx/4xx/5xx are not ready. `"tcp"` requires a successful connection to the runtime container's configured port. `"none"` skips both probes but still requires a live runtime process. Startup readiness is bounded to 10 minutes per candidate, the overall build task to 45 minutes, and image pulls to 10 minutes. A process exit stops PID 1 and Docker marks the container exited; the one-minute reaper transitions a `RUNNING` record to `FAILED`.

GitHub credential rewrites are supplied via process-scoped Git config, not written to `.git/config`. Existing stale token rewrites are removed before Git operations.

## Verification commands

Run Go tests from `backend`:

```bash
GOCACHE=/tmp/api-sandbox-gocache go test ./...
API_SANDBOX_DOCKER_INTEGRATION=1 GOCACHE=/tmp/api-sandbox-gocache go test -run TestDockerRuntimeContainerIsolation -count=1 -v ./provider
```

Run the Docker CLI isolation fixture from the repository root:

```bash
bash backend/scripts/test-docker-workspace-isolation.sh
```

The integration test creates two real containers through the production container helper, inspects their mounts, resource/security settings and network attachments, checks cross-environment file visibility, verifies the Docker socket is absent, and verifies that a script running as PID 1 exits the container and that failed startup resources are removed. Unit tests cover HTTP 200 readiness, rejection of 404/403/500 and redirects, transport failure, process exit, and the `none` policy. The shell fixture independently checks workspace isolation, actual UID, capabilities, namespaces, devices, memory/swap, CPU, PIDs, and stop/remove cleanup.

The final Compose run exercised environment creation through the API, PostgreSQL, Redis/Asynq, the worker, Docker provisioning, a Node application running as the container's main process, 2xx readiness through Traefik, environment/log/file/Git-status reads, restart to a distinct runtime container, and API deletion cleanup. Temporary user/project records were required because normal project creation still fails: its API/model path omits the database-required `owner_organization_id`. The initial live run exposed that preview routes were stored as Redis hashes, which Traefik's KV provider did not load; provisioning now writes individual Redis keys in the documented layout. The corrected preview returned 200 with the fixture body. All temporary records, containers, workspace files, Git config, and the test user's bridge network were removed.

## Local inspection commands

```bash
docker inspect api-sandbox-env-<environment-id> --format '{{json .HostConfig}}'
docker inspect api-sandbox-env-<environment-id> --format '{{json .Mounts}}'
docker inspect api-sandbox-env-<environment-id> --format '{{json .NetworkSettings.Networks}}'
```
