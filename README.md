# API Sandbox

A single-host tool that clones a GitHub repo onto a Linux host, runs it in a Docker container with the code bind-mounted, optionally provisions a database sidecar, and gives users a browser IDE and preview URL via Traefik.

**What it is not:** a secure multi-tenant cloud, production hosting, or a platform with guaranteed latency under all conditions.

**How isolation works:** Docker cgroups, capability drops, and per-org bridge networks — best-effort. The control plane mounts `/var/run/docker.sock`, which is equivalent to host root. If the Go backend is compromised, the host is compromised.

Designed for trusted users on a dedicated single host. Do not expose to untrusted users or arbitrary public repos.

---

## What It Does

1. **GitHub OAuth** — sign in, clone a repo, push changes back to GitHub from the browser.
2. **Live file editing** — code is bind-mounted into a language runtime container; process watchers restart on save.
3. **Sidecar databases** — PostgreSQL, MySQL, Redis provisioned per environment on demand.
4. **Browser IDE** — Monaco editor + Xterm.js terminal.
5. **Preview URLs** — Traefik routes `<env-id>.domain` to the running container.
6. **Role-based access** — `OWNER`, `COLLABORATOR`, `VIEWER` enforced across all APIs.

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
| Isolation | Best-effort containers (cgroups, CapDrop, per-org networks) |
| Hostile multi-tenant | **Not supported** |
| Control plane | **Docker socket = host root equivalent** |
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
- [Performance Evaluation](docs/EVALUATION.md)
- [Monitoring](docs/MONITORING.md)

## License

MIT License — Copyright (c) 2026 API Sandbox. See [LICENSE](LICENSE).
