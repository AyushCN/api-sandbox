# API Sandbox Orchestration Platform

A powerful, full-stack environment provisioning and sandboxing platform. This project allows users to deploy and manage zero-config GitHub repositories on-demand through a sleek web dashboard, dynamically orchestrating the container lifecycle, networking, and reverse proxying behind the scenes.

## 🚀 Key Features

### 📦 Zero-Config Deployments (Nixpacks)
*   **No Dockerfile Required:** Automatically detects the language (Node.js, Python, Go, Rust, etc.) and generates an optimized, cached build plan using **Nixpacks**.
*   **Deep Subdirectory Support:** Users can deploy specific folders inside monorepos directly (e.g. `https://github.com/org/repo/tree/main/examples/api`).

### 🔒 Enterprise-Grade Security
*   **Strict Container Isolation:** Each user workspace is assigned a dynamically generated, dedicated Docker bridge network (`api-sandbox-net-<orgId>`). Containers are bound exclusively to this network, preventing lateral movement and inter-tenant communication.
*   **Kernel-Level Hardening:** Sandboxes are deployed with `no-new-privileges:true` and `CapDrop: ALL` to completely neuter privilege escalation and breakout vectors.
*   **Host Network Protection:** Core platform services (PostgreSQL, Redis) are bound strictly to `127.0.0.1` on the host, preventing sandbox containers from exploiting the default Docker gateway to access internal databases.
*   **Resource Limits:** Hard caps on memory (512MB), CPU quotas, and PIDs (max 256) are strictly enforced at the container level to protect host stability from fork bombs or memory leaks.
*   **Path Traversal & SSRF Prevention:** Strict bounds-checking on subdirectory cloning and explicit enforcement of `https://github.com/` URLs.
*   **Hardened Authentication & SMTP:** JWT-based system enforcing 12-character complex passwords. Verification and password resets utilize generic `net/smtp` to send real emails via any provider (SendGrid, SES, Mailgun).
*   **Strict API Rate Limiting:** Powered by Redis, registration/login endpoints, password reset flows, and all authenticated data-fetching endpoints (e.g. `/environments`) are heavily rate-limited (e.g., max 200 reqs/min) to prevent database exhaustion.
*   **Resource Quotas:** Database-enforced deployment quotas (e.g., max 5 running sandboxes, max 10 builds per hour per user) to prevent platform abuse.
*   **Organizations & Teams:** Built-in multi-tenancy grouping. Workspaces are isolated by `OrganizationID`. Teammates can be invited to an Organization, automatically sharing access to the same dashboard, logs, and internal Docker networks for seamless microservice composition.
*   **Comprehensive Audit Logging:** High-impact mutations (Environment Create/Delete/Restart) are immutably logged with `UserID`, `Action`, `Resource`, and `IPAddress` for SOC2 compliance and operational visibility.

### 🌐 Dynamic Reverse Proxy (Traefik)
*   **Instant Routing with Isolation:** Traefik natively hooks into the Docker socket to map wildcard subdomains instantly. Crucially, the orchestrator dynamically attaches the Traefik proxy to each user's isolated network, guaranteeing secure traffic routing without bridging multi-tenant networks.
*   **Production Domain Support:** Set the `DOMAIN` environment variable (e.g. `sandbox.yourcompany.com`) and the platform will dynamically provision `https://[env-id].sandbox.yourcompany.com` out of the box.

### ⚙️ Backend Orchestration & Operability (Go)
*   **BuildKit & Docker Engine API:** Interacts directly with the Docker Daemon via `fsouza/go-dockerclient` (API v1.41) to enable advanced BuildKit context generation.
*   **Asynchronous Task Queue:** Uses `hibiken/asynq` with Redis to handle long-running git clones and Nixpack builds in the background.
*   **Resilient Database Layer:** Built-in connection pooling (Max Open, Max Idle, Max Lifetime) to gracefully handle high concurrency without exhausting PostgreSQL connections.
*   **Robust Error Handling:** Features exponential backoff retries (3 attempts) on transient database failures and strict limit-based pagination on heavy database queries.
*   **Graceful Shutdown:** Implements OS signal trapping (`SIGTERM`/`SIGINT`) to cleanly drain HTTP requests and allow running container builds to finish before exiting, preventing zombie state.
*   **Structured Logging & Metrics:** Uses Go 1.21 `log/slog` for fully parsable JSON structured logging, and exports vital health metrics (DB connections, active containers) via a `/metrics` Prometheus endpoint.
*   **Automated Backups:** Includes a production-ready `backup.sh` cron script to perform `pg_dump`, gzip compression, and rolling 30-day automated uploads to an S3 bucket.

### 🎨 Frontend Dashboard (Next.js / Material Design 3)
*   **Sleek Modern UI:** Built with dark mode, Material Design 3 tokens, glassmorphism elements, and smooth `framer-motion` animations.
*   **Route Groups:** Clean Next.js app router architecture separating public auth pages from protected `/(main)/` dashboards.
*   **Live Terminal Output:** Integrates `xterm.js` to render Docker build logs in a native-feeling terminal window using highly reliable SWR polling.

## 🛠️ Tech Stack

*   **Go** (Backend API & Orchestration Worker)
*   **Gin** (HTTP Router & JWT Middleware)
*   **GORM** (PostgreSQL ORM)
*   **Asynq & Redis** (Background Job Queue & Rate Limiting)
*   **Docker Engine API & Traefik v3** (Containerization & Reverse Proxy)
*   **Nixpacks** (Zero-config Build System)
*   **Next.js 14 & React** (Frontend Dashboard & Auth Provider)
*   **Tailwind CSS & Framer Motion** (Styling & Animation)
*   **xterm.js** (In-browser Terminal)

## 🏃 Getting Started

### 1. Start Infrastructure Services
The project uses Docker Compose to run PostgreSQL, Redis, and the Traefik proxy.
```bash
cd frontend
docker compose up -d
```

### 2. Run the Go Backend & Worker
Ensure you have the `DOMAIN` (optional) and `JWT_SECRET` environment variables set.
```bash
cd backend
go build -o server .
./server
```

### 3. Run the Next.js Frontend
In a new terminal:
```bash
cd frontend
npm run dev
```
Open `http://localhost:3000` in your browser.

## 📝 Recent Milestones
*   **Complete Nixpacks Overhaul:** Repos without Dockerfiles now automatically build! Subdirectory cloning properly scopes the build context to prevent host-level path traversal.
*   **Traefik Integration:** Replaced manual proxy setup with a native Traefik container inside `docker-compose.yml` for seamless, instant URL routing.
*   **Production Readiness Audit Fixes:** Implemented severe missing limits: Container Network Isolation, Container Memory/CPU limits, DB Connection Pooling, Error Handling with Exponential Backoff, and Endpoint Pagination.
*   **Operational Hardening:** Integrated Graceful Shutdown, API-wide Rate Limiting, `log/slog` structured JSON logging, a Prometheus `/metrics` endpoint, native SMTP email support, and an automated PostgreSQL S3 Backup script.
*   **Advanced Security Hardening:** Patched container host access by forcing `127.0.0.1` binds on infrastructure databases, dropped Docker capabilities (`CapDrop`), enforced `no-new-privileges`, introduced strict Password Reset rate-limiting, and established comprehensive Audit Logging.
*   **Organizations & Collaborative Workspaces:** Upgraded from strict single-user architecture to an `OrganizationID`-based tenancy model. Teammates can now share access to sandbox management and communicate seamlessly across shared, isolated Docker networks.
