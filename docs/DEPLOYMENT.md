# Deployment Guide

This guide walks you through deploying the API Sandbox platform on a clean Virtual Machine (VM) using Docker Compose.

> [!CAUTION]
> **Security Warning: Docker Socket Mount**
> This platform mounts `/var/run/docker.sock` into the backend container so it can provision sandboxes on the host machine. **This effectively grants the backend container root access to the host machine.** Do not run this platform on a host containing sensitive data or other critical workloads unless you explicitly trust all authenticated users.

## 1. Prerequisites

- A Linux VM with **Docker** and **Docker Compose (v2)** installed.
- At least 4GB of RAM (8GB+ recommended depending on sandbox quota).
- A domain name pointing to the VM (e.g., `sandbox.example.com`).
- A wildcard DNS record pointing to the VM (e.g., `*.sandbox.example.com`) for routing traffic to individual ephemeral sandboxes.

## 2. Configuration

1. Clone the repository to your VM:
   ```bash
   git clone https://github.com/your-org/api-sandbox.git
   cd api-sandbox
   ```

2. Create the workspaces directory on your host:
   ```bash
   # This must be an absolute path
   sudo mkdir -p /var/lib/api-sandbox/workspaces
   sudo chown -R $USER:$USER /var/lib/api-sandbox/workspaces
   ```

3. Configure your environment variables:
   ```bash
   cp .env.example .env
   ```
   Open `.env` in your editor and fill out the required secrets:
   - `JWT_SECRET`: Generate with `openssl rand -hex 32`
   - `TOKEN_ENCRYPTION_KEY`: Generate with `openssl rand -hex 16` (Must be exactly 32 chars)
   - `GITHUB_CLIENT_ID` & `GITHUB_CLIENT_SECRET`: See next step.
   - `HOST_WORKSPACES_DIR`: Set to `/var/lib/api-sandbox/workspaces` (must be absolute).
   - `DOMAIN`: Set to your base domain (e.g., `sandbox.example.com`).
   - `APP_URL`: Set to `https://sandbox.example.com`.

## 3. GitHub OAuth Setup

1. Go to your GitHub Developer Settings -> OAuth Apps.
2. Create a new app.
3. Set the **Authorization callback URL** to: `https://<YOUR_DOMAIN>/api/auth/github/callback`
4. Copy the Client ID and Client Secret into your `.env` file.

## 4. Start the Platform

Build the images and start the services in the background:
```bash
docker compose up -d --build
```

## 5. Verification

1. **Check Backend Health**:
   Run `curl https://<YOUR_DOMAIN>/api/health`
   You should receive a `200 OK` response with `{"status":"ok","db":"ok","redis":"ok"}`.

2. **Access the Frontend**:
   Open `https://<YOUR_DOMAIN>` in your browser. You should see the login page.

3. **End-to-End Test**:
   - Log in with GitHub.
   - Create a new sandbox.
   - Verify that the container spins up and files appear in `/var/lib/api-sandbox/workspaces/<env_id>` on your host.
   - Make a change to a file in the browser IDE and verify that the "Live" / "Saved" indicator updates successfully (verifying the reload signal).
   - Check that the preview URL `https://<env_id>.<YOUR_DOMAIN>` resolves.

## 6. Capacity Planning

Each active sandbox consumes approximately 1GB of total memory footprint (512MB RAM + 512MB Swap). Additionally:
- Concurrent **BUILDING** states (npm install, pip install) will create temporary spikes in CPU and disk I/O.
- The control plane baseline (Traefik, Postgres, Redis, Backend, Frontend) consumes roughly 2GB of RAM.

Use this starting heuristic to calculate the maximum number of concurrent running sandboxes for your host VM:
`max_running ≈ (host_RAM - 2GB) / 1GB`

*Example: An 8GB VM can comfortably host roughly 6 concurrent active user sandboxes. Measure real usage with `docker stats`.*

## Troubleshooting

### Workspace Path Mismatch
If sandboxes fail to start or the preview shows an empty directory, verify that `HOST_WORKSPACES_DIR` in `.env` is an exact absolute path matching a real directory on your host machine.

### Traefik Network Attach Fails
If the backend logs show an error about attaching Traefik to the isolated sandbox networks, ensure the Traefik container name in `docker-compose.yml` exactly matches `api-sandbox-traefik`.

### OAuth Callback Fails
If logging in results in a redirect mismatch error from GitHub, double-check that the callback URL configured in GitHub exactly matches `https://<YOUR_DOMAIN>/api/auth/github/callback` (including HTTP vs HTTPS).
