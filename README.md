# API Sandbox Orchestration Platform

A powerful, full-stack environment provisioning and sandboxing platform. This project allows users to deploy and manage zero-config GitHub repositories on-demand through a sleek web dashboard, dynamically orchestrating the container lifecycle, networking, and reverse proxying behind the scenes.

## 🚀 Key Features

### 📦 Zero-Config Deployments (Nixpacks)
*   **No Dockerfile Required:** Automatically detects the language (Node.js, Python, Go, Rust, etc.) and generates an optimized, cached build plan using **Nixpacks**.
*   **Deep Subdirectory Support:** Users can deploy specific folders inside monorepos directly (e.g. `https://github.com/org/repo/tree/main/examples/api`).

### 🔒 Enterprise-Grade Security
*   **Strict Container Isolation:** Each user is assigned a dynamically generated, dedicated Docker bridge network (`api-sandbox-net-<userId>`). Containers are bound exclusively to this network, preventing lateral movement and inter-tenant communication.
*   **Resource Limits:** Hard caps on memory (512MB), CPU quotas, and PIDs (max 256) are strictly enforced at the container level to protect host stability from fork bombs or memory leaks.
*   **Path Traversal & SSRF Prevention:** Strict bounds-checking on subdirectory cloning and explicit enforcement of `https://github.com/` URLs.
*   **Hardened Authentication:** JWT-based system enforcing 12-character complex passwords (symbols, numbers, cases). Includes active checks against missing `JWT_SECRET` keys.
*   **Brute-Force Protection:** IP-based rate limiting on `/auth/register` and `/auth/login` endpoints powered by Redis.
*   **Resource Quotas:** Database-enforced deployment quotas (e.g., max 5 running sandboxes, max 10 builds per hour per user) to prevent platform abuse.
*   **Strict Multi-Tenancy:** Robust data isolation where environments, logs, and metrics are strictly filtered and isolated by `UserID`.

### 🌐 Dynamic Reverse Proxy (Traefik)
*   **Instant Routing:** Traefik natively hooks into the Docker socket to map wildcard subdomains instantly as containers spin up. Zero Nginx configuration needed!
*   **Production Domain Support:** Set the `DOMAIN` environment variable (e.g. `sandbox.yourcompany.com`) and the platform will dynamically provision `https://[env-id].sandbox.yourcompany.com` out of the box.

### ⚙️ Backend Orchestration (Go / Gin / Asynq)
*   **BuildKit & Docker Engine API:** Interacts directly with the Docker Daemon via `fsouza/go-dockerclient` (API v1.41) to enable advanced BuildKit context generation.
*   **Dynamic Port Mapping:** Automatically binds internal exposed ports to host ports using `PublishAllPorts: true`, supporting any port seamlessly.
*   **Asynchronous Task Queue:** Uses `hibiken/asynq` with Redis to handle long-running git clones and Nixpack builds in the background.
*   **Resilient Database Layer:** Built-in connection pooling (Max Open, Max Idle, Max Lifetime) to gracefully handle high concurrency without exhausting PostgreSQL connections.
*   **Robust Error Handling:** Features exponential backoff retries (3 attempts) on transient database failures and strict limit-based pagination on heavy database queries.

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
*   **Security Hardening Check:** Successfully validated and patched vectors for Path Traversal, SSRF, and Login Brute-forcing. Deployment quotas are fully active.
