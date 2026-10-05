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
| Network attachment | Organization-specific user bridge |

The direct Docker shell integration script uses `NanoCpus=1000000000` for its one-CPU fixture. Production container creation expresses the same CPU allowance using quota/period, so `NanoCpus` is zero in inspect output for production containers; inspect `CpuQuota` and `CpuPeriod` there.

## Network behavior

Compose infrastructure services share `traefik-net`: backend/worker, PostgreSQL, Redis, Traefik, and frontend. Each runtime container attaches to an organization bridge with its optional database sidecar and Traefik. It is not attached to `traefik-net` and has no shared Redis/PostgreSQL service route. Local real-container probes confirmed shared Redis DNS/connectivity and host-gateway port 80 were unreachable, while outbound HTTP succeeded. Internet egress supports GitHub/repository operations, package installation, dependency downloads, and projects that call external APIs; it remains enabled.

The local verification did not establish connectivity to every arbitrary backend/host port or every external API. Network claims should be scoped to the specific tested addresses and ports.

## Image warm pool and lifecycle

The image pool pulls runtime images and stores image references in Redis. Provisioning may consume a matching reference, but always creates a fresh runtime container with new environment mounts. Startup drains old `api-sandbox-warm-*` containers and clears old Redis warm-container IDs. Environment deletion removes the runtime and sidecar containers, workspace, and scoped cache; cleanup errors are returned/logged instead of reporting successful deletion.

An Asynq task reconciles Docker state every five minutes. It removes containers whose environment is absent or no longer active; marks a `RUNNING` record failed when its runtime is missing or stopped; and marks `BUILDING` records failed after 30 minutes if no runtime exists or provisioning remains stale. Reconciliation errors are returned and logged. Docker removal-failure injection and every database/Docker race have not been exercised live.

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

The integration test creates two real containers through the production container helper, inspects their mounts, resource/security settings and network attachments, checks cross-environment file visibility, verifies the Docker socket is absent, and forces a missing-command start failure to verify partial-container cleanup. The shell fixture independently checks workspace isolation, actual UID, capabilities, namespaces, devices, memory/swap, CPU, PIDs, and stop/remove cleanup.

The full local Compose route exercised API environment creation, PostgreSQL state, Redis/Asynq, the worker, Docker image/container provisioning, runtime app execution, and API deletion cleanup. The ordinary project-creation API could not be used because it omits the database-required `owner_organization_id`; temporary test-only records were created to reach the Docker flow. That means the normal project-creation path did not pass this end-to-end check. Preview through Traefik returned 404, and the current readiness check treats any status other than 502 as ready; this is tracked as a separate API/preview issue, not a Docker isolation pass.

## Local inspection commands

```bash
docker inspect api-sandbox-env-<environment-id> --format '{{json .HostConfig}}'
docker inspect api-sandbox-env-<environment-id> --format '{{json .Mounts}}'
docker inspect api-sandbox-env-<environment-id> --format '{{json .NetworkSettings.Networks}}'
```
