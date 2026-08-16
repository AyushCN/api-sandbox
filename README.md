# Live Testing Sandbox Platform

A high-utility orchestration platform for ephemeral, high-speed development sandboxes. This project provides backend developers with instant, safe, and disposable cloud-based development environments to test server-side applications with live hot-reloading and automated database provisioning.

Instead of deploying static images, this platform mounts your code into language-specific development runtimes with sidecar databases, enabling live code edits via an integrated web IDE synced directly to your GitHub repository.

## ✨ Core Features

1. **Ephemeral Dev Runtimes**: Instant orchestration of hot-reloading containers via host-level bind mounts. Natively supports **Node.js, Python, Go, Ruby, PHP, Rust, .NET (C#), Java, C, and C++** with sub-2-second edit-to-impact latency.
2. **GitHub-First Source of Truth**: End-to-end GitHub OAuth integration. The sandbox acts as a temporary mirror. You can commit and push directly to GitHub from the browser.
3. **Zero-Config Databases**: Automatic provisioning of isolated sidecar databases (PostgreSQL, MySQL, Redis) strictly tied to the lifecycle of the ephemeral sandbox. Fine-grained connection variables (e.g., `DB_HOST`, `DB_PORT`, `DB_USER`) are automatically injected into the sandbox container.
4. **Browser IDE & Terminal**: Integrated file editing and terminal access to instantly test backend APIs before pushing.
5. **Team Collaboration & RBAC**: Real-time user search for inviting teammates. Strict Role-Based Access Control ensures `VIEWER` roles have true read-only access (enforced at both UI and API levels), while `OWNER` and `COLLABORATOR` roles can edit, commit, and push changes.
6. **Isolated Workspaces**: Sandboxes can be launched in isolated, shared workspaces. A private "Default Workspace" is automatically created for all new users.
7. **Fork Sandbox**: Seamlessly clone any environment you have access to. Forking duplicates the entire container and sidecar context into your own isolated sandbox to avoid team conflict.
8. **Strict Invitation Security**: Pending invitations grant zero access to sandboxes or code until the user explicitly accepts the invitation.
9. **Crash Visibility**: When a sandbox container fails to start, the exact application traceback is surfaced directly in the App Output panel, replacing the silent "Container is provisioning" message.
10. **Smart Boot Polling**: The health checker waits up to 120 seconds for slow runtimes (e.g., Python `pip install`, Node.js `npm install`) before declaring a container failed, eliminating false crash-loop cycles on cold images.

## 📦 Changelog

### v1.3.4 — 2026-08-16 (Extended Language Support & Dev Loop Optimization)

- **Languages**: Added native hot-reloading heuristics for `.NET (C#)`, `Java`, `C`, and `C++` using minimal Alpine base images (`dotnet/sdk:8.0-alpine`, `eclipse-temurin:21-jdk-alpine`, and `alpine:3.19` with injected `build-base`/`cmake`).
- **Dev Loop**: Refactored the `POST /files/content` API to execute Git staging and DB writes asynchronously, achieving a true < 2s hot-reloading loop.
- **Resilience**: Upgraded Node.js runtimes to use native `node --watch` instead of `nodemon` to eliminate download overhead.
- **Resilience**: Injected Traefik `retry.attempts=10` middleware to seamlessly mask `502 Bad Gateway` errors while sandbox containers reboot.
- **Security Audit**: Successfully executed Experiment 5, mathematically proving 100% IDOR resistance across all operational APIs.

### v1.3.3 — 2026-08-16 (Host Security & Cleanup)

- **Security**: Removed exposed Postgres and Redis ports from docker-compose; locked down `/metrics` with token auth.
- **Operations**: Added robust daemon-level orphan reaper and disk workspace garbage collection for idle sandboxes.
- **Cleanup**: Fixed file watcher CWD path bugs; removed redundant `/sync` git endpoints; eliminated stale `install.sh` install scripts.
- **Testing**: Added end-to-end testing script via session cookie (`e2e_session.sh`).

### v1.3.2 — 2026-08-16 (Unified Deployment & Build Fixes)

- **fix**: Next.js frontend now correctly injects `BACKEND_URL` at build time to prevent `localhost:8080` proxy loops inside Docker.
- **fix**: WebSocket upgrader strictly binds origins to `APP_URL` instead of the undocumented `FRONTEND_URL`.
- **fix**: Unified `docker-compose.yml` to automatically orchestrate Traefik, Postgres, Redis, Frontend, and Backend as a single cohesive stack.
- **docs**: Corrected encryption key documentation; `TOKEN_ENCRYPTION_KEY` strictly requires `openssl rand -hex 16` to fulfill the 32-byte exact length constraint.

### v1.3.1 — 2026-08-16 (Sandbox Lifecycle Stability)

- **fix**: `GetDockerLogs` now falls back to the DB crash log when the container is absent (fixes silent "provisioning" spinner)
- **fix**: Corrected `ORDER BY timestamp` column name in the DB-fallback log query (was incorrectly `created_at`, causing a 42703 SQL error)
- **fix**: `TouchFileInContainer` now uses the correct `api-sandbox-env-` name prefix, matching the provisioning subsystem
- **fix**: Replaced the 3-second single health check with a 120-second polling loop so slow dependency installs (`pip`, `npm`, `go mod`) no longer cause false crash detection
- **fix**: Boot log message "Waiting for sandbox to initialize…" is now written to the DB so the App Output stays informative during the install phase

## ⚠️ Known Limitations & Security Caveats

**This system is an experimental prototype and is NOT a security boundary for hostile multi-tenant public internet traffic without further hardening.**

1. **Single host** — The platform relies on one Docker daemon and has no multi-node scheduler or federation capabilities.
2. **Best-Effort Container Isolation** — Environments run with dropped capabilities and no `docker.sock` access, but do not use hypervisor-level isolation (e.g., Firecracker).
3. **GitHub OAuth Requirement** — You must set up a GitHub OAuth App to use the platform as pushing and pulling depend entirely on GitHub as the source of truth.

## 🏃 Quick Start

### 1. Set up GitHub OAuth
1. Go to your GitHub Developer Settings -> OAuth Apps.
2. Create a new app with the callback URL: `http://localhost/api/auth/github/callback` (or your domain).
3. Generate a Client ID and Secret.

### 2. Configure Environment Variables
```bash
cp .env.example .env
```
Open `.env` and fill in the required variables:
- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`
- `TOKEN_ENCRYPTION_KEY` (use `openssl rand -hex 16`)
- `JWT_SECRET` (use `openssl rand -hex 32`)

### 3. Start the Unified Stack
The project uses Docker Compose to automatically orchestrate PostgreSQL, Redis, the Traefik reverse proxy, the Next.js Frontend, and the Go Backend.
```bash
# Create the workspaces directory (required for bind mounts)
sudo mkdir -p /var/lib/api-sandbox/workspaces
sudo chown -R $USER:$USER /var/lib/api-sandbox/workspaces

# Build and start all services
docker compose up -d --build
```

Open `http://localhost` in your browser. You're ready to go!

## 📚 Documentation Index
- [Architecture & Trust Boundaries](docs/ARCHITECTURE.md)
- [Deployment Guide](docs/DEPLOYMENT.md)
- [OpenAPI Specification](docs/openapi.yaml)

## License
MIT License. See [LICENSE](LICENSE) for details.
