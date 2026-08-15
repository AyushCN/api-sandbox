# API Sandbox Deployment Guide

This guide covers deploying the API Sandbox platform to a single production host.

> [!WARNING]
> **CRITICAL SECURITY WARNING: Docker Socket Access**
>
> The `backend` container requires the Docker socket (`/var/run/docker.sock`) mounted as a volume. This is inherent to the architecture because the backend must orchestrate isolated user environments on the host machine. 
> 
> **Mounting the Docker socket gives the backend container host-level root control.** If the API backend is compromised, the attacker essentially gains root access to the host server. 
> 
> You should deploy this platform on an isolated, dedicated host/VM. Do not run other sensitive workloads on the same machine.

## Prerequisites

- A dedicated Linux VM/Host
- Docker and Docker Compose (v2) installed
- (Optional but recommended) A domain name pointing to your server's IP address (e.g. `sandbox.yourdomain.com` and a wildcard `*.sandbox.yourdomain.com`).

## 1. Initial Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/api-sandbox/backend.git api-sandbox
   cd api-sandbox
   ```

2. **Configure Environment Variables:**
   Copy the example config to `.env`:
   ```bash
   cp .env.example .env
   ```
   Open `.env` in a text editor and fill in the required values:
   - `JWT_SECRET`: Must be a long, secure random string. (e.g. generate via `openssl rand -base64 32`)
   - `DOMAIN`: Your root domain (e.g. `sandbox.example.com`). Environments will be assigned subdomains of this.
   - `APP_URL`: The URL where the frontend is reachable (e.g. `https://sandbox.example.com`).

## 2. First Boot & Email Configuration

User registration requires email verification by default. You have two options for the first boot:

**Option A (Production): Configure SMTP or SendGrid**
In your `.env` file, configure `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, and `SMTP_PASS` (or `SENDGRID_API_KEY`).

**Option B (Testing/Local): Disable Verification**
If you don't have SMTP credentials yet, you can temporarily disable email verification so you can register your admin account immediately:
Set `SKIP_EMAIL_VERIFICATION=true` in `.env`.

## 3. Starting the Services

Once your `.env` is configured, start the stack:

```bash
docker compose up -d
```

This will automatically pull base images, build the Go backend and Next.js frontend, and initialize the PostgreSQL and Redis containers alongside the Traefik proxy.

### Verify Health

You can check if the core dependencies (DB and Redis) successfully initialized:
```bash
curl http://localhost:8080/health
```
You should see a `{"status":"ok", "db":"ok", "redis":"ok"}` response.

## 4. First User Registration

1. Navigate to your `APP_URL` in a browser.
2. Register a new account.
3. If `SKIP_EMAIL_VERIFICATION` was true, you can log in immediately. Otherwise, check your email for the verification link.
4. Try creating your first Sandbox Environment to verify the backend can successfully orchestrate Docker containers via the socket.

## 5. Enable HTTPS (Let's Encrypt / ACME)

Traefik handles TLS automatically, but it is disabled by default for local development. To enable it on a public server:

1. Ensure your server is accessible on ports 80 and 443.
2. Edit `.env` and set:
   ```env
   ENABLE_TLS=true
   ACME_EMAIL=your-email@example.com
   TRAEFIK_ENTRYPOINT=websecure
   ```
3. Restart the stack:
   ```bash
   docker compose down
   docker compose up -d
   ```

> [!NOTE]
> The Traefik dashboard is insecure and disabled by default. Do not enable it (`TRAEFIK_DASHBOARD=true`) on a public IP without adding basic auth middleware to the compose file.

## 6. Observability & Alerting

The API Sandbox backend exposes a Prometheus-compatible metrics endpoint at `/metrics`.

**Scraping Requirements:**
- **Private Network Only**: Do NOT expose `/metrics` to the public internet. It contains internal operational data.
- Configure your Prometheus scraper to target the backend container internally (e.g., `http://backend:8080/metrics` from within the Docker network, or bound to `localhost` on the host).

**Key Metrics to Monitor:**
- `asynq_queue_size`: To monitor build queue backlog.
- API HTTP response times and `429 Too Many Requests` frequency.
- Go runtime memory/GC stats.

## Architecture Limitations

Before deploying to production, please be aware of the following architectural limits:

- **Single Host:** The orchestration design is tightly coupled to the local Docker socket. Multi-node clusters (Swarm/K8s) are not supported by this compose setup.
- **Socket Trust:** As warned above, the backend assumes total trust over the host Docker daemon.
- **Live Editor:** The in-app editor is fully live! Changes you make in the editor are immediately synced to the container via host volume bind-mounts, triggering hot-reloading (via nodemon, watchdog, or air) depending on your language runtime.
- **Redis Required:** The Asynq worker queue relies heavily on Redis for job scheduling and state consistency. Do not attempt to strip Redis from the stack.
