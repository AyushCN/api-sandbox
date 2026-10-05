# API Sandbox

A single-host tool that clones a GitHub repo onto a Linux host, runs it in a Docker container with the code bind-mounted, optionally provisions a database sidecar, and gives users a browser IDE and preview URL via Traefik.

**What it is not:** a secure multi-tenant cloud, production hosting, or a platform with guaranteed latency under all conditions.

**How isolation works:** Each environment gets a fresh Docker container, its own writable workspace mounted at `/app`, a user-scoped bridge network, resource limits, dropped capabilities, and `no-new-privileges`. This is best-effort isolation with a shared host kernel. Environments owned by the same user share a Docker bridge. The backend and worker share a read/write `/var/run/docker.sock`; Docker API access is effectively host-root authority. Traefik also mounts the socket read-only at the filesystem level, which does not restrict Docker API operations.

Designed for trusted users on a dedicated single host. Do not expose to untrusted users or arbitrary public repos.

---

## What It Does

1. **GitHub OAuth** — sign in, authenticate, push changes back to GitHub.
2. **Project Sharing & Auto-Forking** — owners can share projects with other users. When an `EDITOR` edits a project, an isolated Workspace (`FORK`) is automatically created for them with their own branch and container environment.
3. **Change Requests & Review** — editors submit their workspace changes via Change Requests, which the `OWNER` can review, diff, and merge back into the canonical workspace.
4. **Live file editing** — code is bind-mounted into a language runtime container; process watchers restart on save.
5. **Scoped WebSockets** — real-time presence and updates scoped by Project, Workspace, and Environment.
6. **Sidecar databases** — PostgreSQL, MySQL, Redis provisioned per environment on demand.
7. **Browser IDE** — Monaco editor + Xterm.js terminal.
8. **Preview URLs** — Traefik routes `<env-id>.domain` to the running container.
9. **Role-based access** — `OWNER`, `EDITOR`, `VIEWER` enforced via robust backend middleware.
10. **Warm Image Pool** — Pre-pulls `node`, `python`, and `go` images. Every environment still gets a new container; warm containers are not reused.
11. **Environment-Scoped Dependency Caches** — Package caches are mounted separately for each environment.

## What It Does Not Do

- Hard multi-tenant isolation (shared kernel, privileged control plane)
- Guaranteed sub-second reload under all conditions (cold installs, Python pip, Go compilation take minutes)
- Production hosting or data durability guarantees

## Reload Latency (Lab Measurements)

Warm reload after a file save, measured in lab conditions via `scripts/measure_loop`:

| Runtime | Warm p50 | Warm p95 | Notes |
|---------|----------|----------|-------|
| Node.js (nodemon) | ~250ms | ~251ms | After `npm install` completes |
| Python (uvicorn --reload) | ~520ms | ~521ms | After `pip install` completes |
| Go (air) | ~400ms | — | After initial compilation |

Cold starts (first boot, dependency install) take significantly longer and are not sub-second.
See [PROOF.md](PROOF.md) for raw numbers.

## Capability & Security Matrix

| Concern | Reality |
|---------|---------|
| Isolation | Best-effort containers (cgroups, CapDrop, per-user networks, per-environment workspace mounts) |
| Hostile multi-tenant | **Not supported** |
| Control plane | **Docker socket = host root equivalent** |
| Runtime user | Root by image default; verify image compatibility before changing |
| Public signups | **Do not do this** |
| Trusted friends on a dedicated host | Viable |

## Quick Start

### 1. GitHub OAuth App

Create a GitHub OAuth App. Set callback URL to `http://localhost/api/auth/github/callback`.

### 2. Environment

```bash
cp .env.example .env
```

Required values:
- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`
- `TOKEN_ENCRYPTION_KEY` — exactly 32 hex chars: `openssl rand -hex 16`
- `JWT_SECRET` — `openssl rand -hex 32`

### 3. Host Directory

```bash
sudo mkdir -p /var/lib/api-sandbox/workspaces
sudo chown -R $USER:$USER /var/lib/api-sandbox/workspaces
```

### 4. Start

```bash
docker compose up -d --build
curl -sS http://localhost/api/health
# expect: {"db":"ok","redis":"ok","status":"ok"}
```

## Documentation

- [Architecture & Trust Boundaries](docs/ARCHITECTURE.md)
- [Deployment Guide](docs/DEPLOYMENT.md)
- [Technical Details](docs/DETAILS.md)
- [Docker Operations and Verification](docs/DOCKER.md)
- [Performance Evaluation](docs/EVALUATION.md)
- [Monitoring](docs/MONITORING.md)

## Docker Runtime Notes

- Runtime workspace: `${HOST_WORKSPACES_DIR}/<environment-id>` → `/app` (read/write). Runtime containers do not receive the shared backend `/app/workspaces` mount.
- Runtime cache: `${HOST_WORKSPACES_DIR}/.cache/<environment-id>/{npm,pnpm,pip,go}`. Cache mounts are writable and scoped by environment.
- Runtime limits: 512 MiB memory, 512 MiB memory+swap, one CPU via quota/period, and 256 PIDs. Capabilities are dropped and `no-new-privileges` is enabled; runtime images currently default to UID 0.
- The Compose infrastructure network contains the backend, PostgreSQL, Redis, Traefik, and frontend. Runtime containers use a user-scoped bridge instead; a user's environments share that bridge. Runtime internet egress currently works and is needed for repository access and dependency downloads.
- `healthCheckType: "http"` (default) requires a 2xx preview response, `"tcp"` requires a successful TCP connection to the runtime port, and `"none"` checks only that the runtime process is alive. Readiness is bounded to 10 minutes per candidate; build tasks have a 45-minute deadline. App exits stop the container and are reconciled to `FAILED` within about one minute.
- The normal project-creation endpoint has a schema/model mismatch for `owner_organization_id`. The runtime preview route now uses Traefik's Redis KV key layout and passed live 2xx readiness; normal project creation remains blocked by the schema mismatch. See [Docker Operations](docs/DOCKER.md).

## License

MIT License — Copyright (c) 2026 API Sandbox. See [LICENSE](LICENSE).
