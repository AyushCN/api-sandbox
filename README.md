# API Sandbox: Sub-Second Ephemeral Dev Environments

A high-performance orchestration platform for instant, ephemeral backend development environments. This platform mounts your host code directly into containerized language runtimes with sidecar databases, enabling live hot-reloading loops with guaranteed p95 latencies of **< 1 second**.

## ✨ Core Features

1. **Sub-Second Hot Reloading**: Powered by direct host bind-mounts and native process watchers (`nodemon`, `air`, `uvicorn`), yielding development loop latencies under 500ms.
2. **HTTP Readiness Probes**: Advanced Traefik-routed HTTP polling accurately determines exactly when a container has successfully restarted, blocking until the application is genuinely listening for traffic.
3. **Automatic Sidecar Databases**: Every environment receives completely isolated, zero-config PostgreSQL, MySQL, and Redis instances.
4. **GitHub Source of Truth**: End-to-end OAuth integration allows users to commit and push their web IDE changes directly back to the active branch without leaving the browser.
5. **Role-Based Access Control**: Granular `OWNER`, `COLLABORATOR`, and `VIEWER` roles enforced mathematically across all APIs.
6. **Fork & Play**: Instantly fork any active environment to create your own isolated sandbox clone without interrupting the main team.
7. **Resilient Orphan Garbage Collection**: Background daemons actively sweep the Docker socket to reap disconnected, idle, or stalled sandbox containers automatically.

## 🚀 Benchmark Validated Dev Loop

The core promise of this platform is **immediate feedback**. Using our internal `measure_loop` telemetry tools, we have rigorously measured the end-to-end latency of a code save propagating to a fully restarted container serving HTTP:

- **Node.js (Express)**: ~250ms
- **Python (FastAPI)**: ~520ms
- **Go (Air)**: ~400ms

*(Benchmarks validated natively via automated HTTP Proxy polling & WebSockets)*

## ⚠️ Security Posture & Capabilities

**This system is an experimental internal development tool. It is NOT a security boundary for hostile multi-tenant public internet traffic.**

| Capability | Reality |
|------------|---------|
| Container Networks | Isolated bridge per organization |
| Multi-tenant hosting | **Not supported** (Best effort isolation only) |
| Architecture Base | **Host Docker socket mount (root-equivalent)** |

Because the Backend container mounts `/var/run/docker.sock` to orchestrate sandbox lifecycles, compromising the Go API effectively grants host-level root access. Do not expose this platform publicly to untrusted users.

## 🏁 Quick Start

### 1. Configure GitHub OAuth
1. Create a GitHub OAuth App.
2. Set the callback URL to `http://localhost/api/auth/github/callback`.
3. Save the Client ID and Secret.

### 2. Prepare Environment
```bash
cp .env.example .env
```
Fill in the `.env` file:
- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`
- `TOKEN_ENCRYPTION_KEY` (Generate via `openssl rand -hex 16`)
- `JWT_SECRET` (Generate via `openssl rand -hex 32`)

### 3. Launch Unified Stack
We use Docker Compose to run PostgreSQL, Redis, Traefik, the Next.js Frontend, and the Go Backend in a unified mesh.
```bash
# Create required bind mount host directory
sudo mkdir -p /var/lib/api-sandbox/workspaces
sudo chown -R $USER:$USER /var/lib/api-sandbox/workspaces

# Build and deploy
docker compose up -d --build
```
Navigate to `http://localhost` to begin provisioning environments.

## 📚 Documentation Index
- [System Architecture](docs/ARCHITECTURE.md)
- [Deployment Guide](docs/DEPLOYMENT.md)
- [Technical Details & APIs](docs/DETAILS.md)
- [Performance Evaluation](docs/EVALUATION.md)
- [Monitoring & Observability](docs/MONITORING.md)
