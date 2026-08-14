# API Sandbox Orchestration Platform

An experimental, single-host orchestration platform for provisioning sandboxed environments. This project allows users to deploy and manage containerized GitHub repositories, utilizing Nixpacks for dynamic build plans. 

Suitable as a research/prototype platform; not a drop-in commercial PaaS.

## ⚠️ Known Limitations & Security Caveats

**This system is an experimental prototype and is NOT a security boundary for hostile multi-tenant public internet traffic without further hardening.**

1. **Single host** — The platform relies on one Docker daemon and has no multi-node scheduler or federation capabilities.
2. **Docker socket = host root equivalent** — The control plane (Go backend) mounts `/var/run/docker.sock` to spin up environments, granting it host-level root privileges. 
3. **No multi-tenant production hardening claim** — It provides best-effort container isolation (via networks and capabilities), but does not use hypervisor isolation (e.g., Firecracker).
4. **Editor is non-live** — The web editor is not bound to a live container. Edits are local and must go through a GitHub commit/sync + rebuild cycle to run.
5. **Redis is a hard dependency** — The platform enforces fail-closed rate limits; without Redis, the API will not function.
6. **Email verification blocks onboarding** — The system requires an SMTP provider to send verification emails, which blocks new user onboarding if unconfigured.
7. **Not public-PaaS-safe** — It is not intended for multi-tenant production hosting of fully untrusted workloads.

## Architecture

The system uses a Go backend, `asynq` worker, Traefik reverse proxy, and dynamically provisioned Docker bridge networks per Organization. For a full breakdown of the architecture, trust boundaries, and environment lifecycle, see [ARCHITECTURE.md](docs/ARCHITECTURE.md).

## 🏃 Quick Start

### 1. Start Infrastructure Services
The project uses Docker Compose to run PostgreSQL, Redis, and the Traefik proxy.
```bash
docker compose up -d
```

### 2. Run the Next.js Frontend
```bash
cd frontend
npm run dev
```

### 3. Run the Go Backend & Worker
Ensure you have the required environment variables (`JWT_SECRET`, `DOMAIN`, etc.) set in `.env`.
```bash
cd backend
go build -o server .
./server
```

Open `http://localhost:3000` in your browser.

## 📚 Documentation Index
- [Architecture & Trust Boundaries](docs/ARCHITECTURE.md)
- [Deployment Guide](docs/DEPLOYMENT.md)
- [Evaluation Plan](docs/EVALUATION.md)
- [OpenAPI Specification](docs/openapi.yaml)

## License
MIT License. See [LICENSE](LICENSE) for details.
