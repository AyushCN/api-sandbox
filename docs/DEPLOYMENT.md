# Deployment Guide

The API Sandbox platform orchestrates user-defined Docker containers. The backend and worker share read/write access to `/var/run/docker.sock`, which grants effectively host-root Docker authority. Traefik also mounts the socket read-only for Docker provider discovery; `:ro` does not restrict Docker API operations. **Deploy only to a dedicated, secure host for trusted users.**

## Prerequisites

- A dedicated Linux Host (e.g., Ubuntu 24.04).
- Root access or a user with `sudo` and `docker` group privileges.
- Docker version >= 24.0.0.
- Docker Compose plugin.
- Port 80 and 443 open.

## 1. Directory Structure

The system requires specific host paths to exist for workspace bind-mounting.

```bash
sudo mkdir -p /var/lib/api-sandbox/workspaces
sudo chown -R $USER:$USER /var/lib/api-sandbox/workspaces
```
The backend bind-mounts this root at `/app/workspaces`. Each runtime gets only its own `<environment-id>` directory at `/app`; package caches live under `.cache/<environment-id>/`. Do not mount the shared root into runtime containers.

## 2. Environment Configuration

Clone the repository and prepare the `.env` file at the root.

```bash
git clone <repo-url> api-sandbox
cd api-sandbox
cp .env.example .env
```

### Essential `.env` Variables:

```env
# Networking
DOMAIN=localhost      # Set this to your actual wildcard domain in production (e.g., my-sandbox.io)
FRONTEND_URL=http://localhost
APP_URL=http://localhost
WS_URL=ws://localhost

# Security Secrets
JWT_SECRET=super-secret-32-byte-string-here
TOKEN_ENCRYPTION_KEY=super-secret-16-byte-exactly

# Authentication (GitHub OAuth)
GITHUB_CLIENT_ID=your_oauth_client_id
GITHUB_CLIENT_SECRET=your_oauth_client_secret

# Infrastructure
HOST_WORKSPACES_DIR=/var/lib/api-sandbox/workspaces
# For Compose, Redis must be addressed by its service name, not localhost:
REDIS_URL=redis://redis:6379
```

`localhost` inside the backend container is the backend itself. `redis://localhost:6379` is suitable only when running the backend directly on a host with Redis bound locally.

## 3. Starting the Stack

The system leverages a unified `docker-compose.yml` file that orchestrates the core infrastructure: Traefik, PostgreSQL, Redis, the Go Backend API, and the Next.js Frontend.

These infrastructure services share `traefik-net`. Runtime containers are created separately on `api-sandbox-net-<user-id>` bridges; environments and sidecars belonging to the same user share a bridge, and runtime containers are not joined to the Compose network. User bridges permit outbound access, which supports Git operations and dependency downloads. Runtime images currently default to root and have Docker-enforced memory, CPU, PID, and capability restrictions; see [Architecture](ARCHITECTURE.md) for exact settings and mounts.

```bash
# Build and deploy detached
docker compose up -d --build
```

### Verifying the Deployment

Run `docker compose ps` to verify the five core services are running and healthy where health checks are configured.

```bash
$ docker compose ps
NAME                     STATUS    PORTS
api-sandbox-backend      Up        8080/tcp
api-sandbox-frontend     Up        3000/tcp
api-sandbox-postgres-1   Up        5432/tcp
api-sandbox-redis-1      Up        6379/tcp
api-sandbox-traefik      Up        0.0.0.0:80->80/tcp
```

## 4. Production Considerations

If deploying to production, consider the following enhancements:

1. **HTTPS / TLS**: Update the Traefik labels in `docker-compose.yml` to utilize Let's Encrypt ACME resolvers for automated TLS certificate provisioning on your `DOMAIN`.
2. **Wildcard DNS**: Ensure your domain provider has an A record pointing `*.your-domain.com` to the host IP. This is mandatory for the ephemeral sandboxes to receive unique subdomains dynamically.
3. **Storage Quotas**: Since workspaces accumulate Git histories and `node_modules`, you should enforce LVM storage quotas or periodically prune idle environments using the backend Garbage Collection daemon.
