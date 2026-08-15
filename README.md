# Live Testing Sandbox Platform

A high-utility orchestration platform for ephemeral, high-speed development sandboxes. This project provides backend developers with instant, safe, and disposable cloud-based development environments to test server-side applications with live hot-reloading and automated database provisioning.

Instead of deploying static images, this platform mounts your code into language-specific development runtimes with sidecar databases, enabling live code edits via an integrated web IDE synced directly to your GitHub repository.

## ✨ Core Features

1. **Ephemeral Dev Runtimes**: Instant orchestration of hot-reloading containers (Node.js, Python, Go, etc.) via host-level bind mounts.
2. **GitHub-First Source of Truth**: End-to-end GitHub OAuth integration. The sandbox acts as a temporary mirror. You can commit and push directly to GitHub from the browser.
3. **Zero-Config Databases**: Automatic provisioning of isolated sidecar databases (PostgreSQL, MySQL, Redis) strictly tied to the lifecycle of the ephemeral sandbox. Fine-grained connection variables (e.g., `DB_HOST`, `DB_PORT`, `DB_USER`) are automatically injected into the sandbox container.
4. **Browser IDE & Terminal**: Integrated file editing and terminal access to instantly test backend APIs before pushing.
5. **Team Collaboration & RBAC**: Real-time user search for inviting teammates. Strict Role-Based Access Control ensures `VIEWER` roles have true read-only access (enforced at both UI and API levels), while `OWNER` and `COLLABORATOR` roles can edit, commit, and push changes.

## ⚠️ Known Limitations & Security Caveats

**This system is an experimental prototype and is NOT a security boundary for hostile multi-tenant public internet traffic without further hardening.**

1. **Single host** — The platform relies on one Docker daemon and has no multi-node scheduler or federation capabilities.
2. **Best-Effort Container Isolation** — Environments run with dropped capabilities and no `docker.sock` access, but do not use hypervisor-level isolation (e.g., Firecracker).
3. **GitHub OAuth Requirement** — You must set up a GitHub OAuth App to use the platform as pushing and pulling depend entirely on GitHub as the source of truth.

## 🏃 Quick Start

### 1. Set up GitHub OAuth
1. Go to your GitHub Developer Settings -> OAuth Apps.
2. Create a new app with the callback URL: `http://localhost:8080/api/auth/github/callback`
3. Add the Client ID and Secret to your `backend/.env` file.

### 2. Start Infrastructure Services
The project uses Docker Compose to run PostgreSQL, Redis, and the Traefik proxy.
```bash
docker compose up -d
```

### 3. Run the Next.js Frontend
```bash
cd frontend
npm install
npm run dev
```

### 4. Run the Go Backend & Worker
Ensure you have the required environment variables (`GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `TOKEN_ENCRYPTION_KEY`, `DOMAIN`, etc.) set in `backend/.env`.
```bash
cd backend
go build -o server .
sudo ./server # Sudo may be required to configure bind-mount directories properly
```

Open `http://localhost:3000` in your browser.

## 📚 Documentation Index
- [Architecture & Trust Boundaries](docs/ARCHITECTURE.md)
- [Deployment Guide](docs/DEPLOYMENT.md)
- [OpenAPI Specification](docs/openapi.yaml)

## License
MIT License. See [LICENSE](LICENSE) for details.
