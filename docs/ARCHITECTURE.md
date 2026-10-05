# Architecture

This document describes the design, lifecycle, and trust boundaries of the API Sandbox.

## What It Is

A single-host orchestrator. One Linux machine runs Docker, Traefik, a Go backend/Asynq worker, a Next.js frontend, PostgreSQL, and Redis. The backend process and worker run in the same service and use the Docker socket to provision ephemeral runtime containers.

## System Diagram

```mermaid
graph TD
    Client[Web Client]
    subgraph Single Host
        Traefik[Traefik Proxy]
        Backend[Go API + Asynq worker - RW docker.sock]
        PostgreSQL[(PostgreSQL)]
        Redis[(Redis)]

        subgraph Organization A Bridge
            EnvA[Sandbox Container A]
            DB_A[(Postgres Sidecar)]
            EnvA --- DB_A
        end

        subgraph Organization B Bridge
            EnvB[Sandbox Container B]
        end
    end

    Client -->|HTTP / WSS| Traefik
    Traefik -->|/api/*| Backend
    Traefik -->|Host: envId.domain| EnvA
    Traefik -->|Host: envId.domain| EnvB

    Backend --> PostgreSQL
    Backend --> Redis
    Backend -->|Enqueues and provisions| EnvA
    Backend -->|Enqueues and provisions| EnvB
```

## Project and Workspace Hierarchy

The fundamental architecture uses a hierarchical approach:

```mermaid
graph TD
    Project[Canonical Project]
    WorkspaceA[Workspace: Owner / Canonical]
    WorkspaceB[Workspace: Editor 1 / Fork]
    WorkspaceC[Workspace: Editor 2 / Fork]
    EnvA[Environment A]
    EnvB[Environment B]
    EnvC[Environment C]

    Project --> WorkspaceA
    Project --> WorkspaceB
    Project --> WorkspaceC
    WorkspaceA --> EnvA
    WorkspaceB --> EnvB
    WorkspaceC --> EnvC
```

*   **Project**: Represents the overarching logical application, managing members and settings.
*   **Workspace**: Represents an isolated git working tree. `OWNER` modifies the canonical workspace. Each `EDITOR` receives a unique isolated `FORK` workspace.
*   **Environment**: The actual running Docker container connected to a Workspace.

## Environment Lifecycle

The environment is the container abstraction attached to a Workspace. Lifecycle state:

```mermaid
stateDiagram-v2
    [*] --> IDLE : Created
    IDLE --> BUILDING : Start requested
    BUILDING --> RUNNING : Base image pulled, volume mounted, health passes
    BUILDING --> FAILED : Clone or health failed
    RUNNING --> STOPPED : User stop
    STOPPED --> BUILDING : Restart
    FAILED --> BUILDING : Retry
    RUNNING --> [*] : Deleted
    STOPPED --> [*] : Deleted
    FAILED --> [*] : Deleted
    IDLE --> [*] : Deleted
```

**"RUNNING" means:** the base image was pulled, code is bind-mounted to `/app`, a process watcher is running, and the current readiness poll did not receive HTTP 502. A 404 can therefore be accepted as ready; this does not prove the application is serving successfully. It does not mean the application is fully built or ready — only that provisioning and the current readiness condition completed.

Every provisioning attempt creates a fresh runtime container. The warm pool pulls and records image references only; it does not hand a running container to an environment. The worker attaches the runtime to its organization bridge and assigns its environment-specific writable mounts.

### Runtime mounts and limits

| Host source | Runtime destination | Mode | Purpose |
|---|---|---|---|
| `${HOST_WORKSPACES_DIR}/<environment-id>` | `/app` | Read/write | That environment's source tree |
| `${HOST_WORKSPACES_DIR}/.cache/<environment-id>/npm` | `/root/.npm` | Read/write | npm cache |
| `${HOST_WORKSPACES_DIR}/.cache/<environment-id>/pnpm` | `/root/.local/share/pnpm/store` | Read/write | pnpm cache |
| `${HOST_WORKSPACES_DIR}/.cache/<environment-id>/pip` | `/root/.cache/pip` | Read/write | pip cache |
| `${HOST_WORKSPACES_DIR}/.cache/<environment-id>/go` | `/go/pkg/mod` | Read/write | Go module cache |

The runtime does not mount the shared `${HOST_WORKSPACES_DIR}` root at `/workspaces`, and does not receive a Docker socket. The trusted backend separately mounts the workspace root at `/app/workspaces` to manage files.

Runtime image defaults currently run as UID 0. The actual Docker configuration sets 512 MiB memory and memory+swap, one CPU using `CPUQuota=100000` and `CPUPeriod=100000`, and a 256 PID limit. It drops all Linux capabilities and enables `no-new-privileges`; it does not use privileged mode, host PID/network/IPC namespaces, or device mappings.

### Docker networks

Compose infrastructure services (backend, PostgreSQL, Redis, Traefik, frontend) share `traefik-net`. Runtime containers use an organization-specific bridge and are not attached to `traefik-net`. A runtime shares its organization network with its database sidecar and Traefik, but not with backend or shared Redis/PostgreSQL. Local Docker verification confirmed outbound HTTP works; bounded probes to host gateway port 80 and shared Redis failed. Internet egress remains enabled for Git/repository access and dependency downloads.

## How the Dev Loop Works

1. User saves a file in the browser IDE
2. Go backend writes the file to the host path `/var/lib/api-sandbox/workspaces/<env-id>/`
3. The sandbox container has this directory bind-mounted to `/app`
4. The process watcher inside the container detects the change and restarts the application
5. The backend polls Traefik with the environment's virtual host header until it receives a non-502 response
6. A `reload_ready` WebSocket event is broadcast to all connected clients

Warm reload latency (Node.js): ~250ms. Cold starts are substantially longer.

## Trust Boundaries

| Boundary | Mechanism | What it does not protect |
|----------|-----------|--------------------------|
| API auth | JWT cookie | Compromised API process |
| Tenant network | Bridge per organization | Host if socket is abused; outbound egress is allowed |
| Container | CapDrop, no-new-privs, mem/PID limits | Kernel exploits, socket mount |
| Path validation | Workspace root checks | Host FS via RCE in Go backend |

## Critical Limitation: Docker Socket

The backend/worker container mounts `/var/run/docker.sock` read/write. It uses Docker operations for image pulls, container create/start/stop/remove/inspect/exec/logs, and network list/create/inspect/connect. This is functionally equivalent to host-root authority. The API handler and worker are in the same backend service, so the current deployment does not grant them separate socket capabilities.

Traefik uses Docker provider discovery to read Compose labels and discover backend/frontend routes; the socket is mounted with `:ro`. That protects the socket path from filesystem writes only; it does not make Docker API access read-only. Removing Docker discovery would require replacing the current dynamic label-based routing configuration. Runtime containers have no socket mount.

| Component | Socket mount | Why / operations | Can it be removed in the current design? |
|---|---|---|---|
| Backend + worker | Read/write | Pulls images; creates, starts, inspects, execs, logs, stops, restarts, and removes containers; creates/lists/connects/inspects networks | No, not while this process provisions and supervises runtime containers directly |
| Traefik (production and dev Compose) | `:ro` filesystem mount | Docker provider discovers labeled Compose services and their endpoints | Not without replacing Docker provider discovery and current label-driven routing |
| Runtime containers | None | User project execution has no Docker control-plane need | No socket is required |
| Frontend, PostgreSQL, Redis | None | No Docker control-plane operations | No socket is required |

The Go API handlers and Asynq worker share the backend service, so they cannot have separate socket permissions in the current deployment. A socket mount with `:ro` is not an API authorization boundary. The Docker socket is not passed to tests/scripts as a container mount; the real-Docker test commands run from the developer/CI process that invokes the Docker CLI or Go client.

**This is a deliberate design tradeoff for a single-host, trusted-user tool.** Do not use this as a multi-tenant public platform. Local inspection also found the runtime preview returned HTTP 404 while readiness accepted any status except 502; treat that as a separate preview/readiness defect, not evidence that runtime networking is healthy.

## Editor Strategy

Code is **not** baked into images. Raw source is mapped from the host into Alpine-based language runtimes via bind mounts. This enables live editing but means the container and host share the same codebase at all times.

Process watchers per runtime:
- **Node.js**: `npx nodemon`
- **Python**: `uvicorn --reload --reload-dir .`
- **Go**: `air` (pinned to v1.52.3, compatible with `golang:alpine`)
