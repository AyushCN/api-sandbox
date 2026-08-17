# Deployment Guide

The API Sandbox platform orchestrates an entire ecosystem of user-defined containers. Because the primary backend system mounts `/var/run/docker.sock` to accomplish this, **the platform must be deployed to a single, dedicated, secure host**. 

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
*Note: If you are running Docker Rootless, adjust ownership to match your remapped IDs.*

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
```

## 3. Starting the Stack

The system leverages a unified `docker-compose.yml` file that orchestrates the core infrastructure: Traefik, PostgreSQL, Redis, the Go Backend API, and the Next.js Frontend.

```bash
# Build and deploy detached
docker compose up -d --build
```

### Verifying the Deployment

Run `docker compose ps` to verify all five core services are running smoothly. 

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
