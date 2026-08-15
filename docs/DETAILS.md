# API Sandbox - Comprehensive Architecture & Implementation Details

This document covers **everything** happening inside the `api-sandbox` directory. It explains the philosophy, the working architecture, and the importance and responsibility of every single component and file in the system. No hidden details are left out.

---

## 1. Core Philosophy & Product Identity

The API Sandbox is a **Development & Testing Sandbox for Backend Developers**. 
It is not an immutable PaaS deployment engine like Heroku. Instead, it solves a specific problem: giving backend developers an instant, cloud-based, ephemeral workspace where they can test their code with live hot-reloading and dynamically provisioned databases before pushing to production.

### The "Sandbox" Approach
- **Dev Runtimes**: Instead of building immutable OCI images via Nixpacks, the platform intelligently detects the project's language (Node, Python, Go, PHP, etc.), launches a pre-warmed language-specific base container, and **bind-mounts** the user's code directly into it.
- **GitHub as the Source of Truth**: The sandbox acts as a temporary mirror of the user's GitHub repository. Authenticated via GitHub OAuth, developers can clone, edit, commit, and securely push directly back to GitHub from the browser.
- **Zero-Config Sidecar Databases**: If the platform detects a database dependency (e.g., Prisma schema, Rails database.yml), it automatically provisions a secure sidecar database (PostgreSQL, MySQL, Redis) strictly tied to that sandbox's lifecycle.
- **Bare-Metal Orchestrator**: To solve the "sandbox-in-sandbox" inception problem, the Go backend runs directly on the Host VM rather than inside a Docker container. This allows the backend to bind-mount directories from the host filesystem into the isolated sandbox containers flawlessly.

---

## 2. Working Architecture (Mental Model)

1. **Client (Browser)** -> Traefik / Next.js Frontend.
2. **Frontend** -> Go Backend API (REST over HTTP, Logs over WebSockets).
3. **Go Backend** -> Controls PostgreSQL (Metadata), Redis (Queues & Rate Limiting), and the Host Docker Daemon via `/var/run/docker.sock`.
4. **Asynq Worker** -> Handles heavy background operations (Cloning repos, provisioning containers) off the main API thread.
5. **Docker Engine (Host)** -> Runs isolated user containers.
    - **User Sandbox**: Language-specific base image, dropped capabilities, no `docker.sock` access, bind-mounted code directory.
    - **Sidecar DB**: Auto-provisioned PostgreSQL/MySQL/Redis container attached to the same private docker network as the User Sandbox.

---

## 3. Backend Implementation (Go)

The backend is built in Go using the **Gin** HTTP framework, **GORM** for database operations, and **Asynq** (Redis) for background jobs.

### Root Level
- `main.go`: The entry point. It initializes the database (`db.InitDB()`), starts the idle cleanup cron job, initializes the queue and Docker engine, and spins up the HTTP API server and/or the Asynq worker based on environment variables.

### `backend/api/` (API Endpoints & Controllers)
This package handles HTTP requests, WebSockets, and Authentication.
- `handlers.go`: Defines the Gin router, applies rate-limiting middlewares, and maps URLs to handler functions.
- `auth.go` & `oauth.go`: Handles identity. `oauth.go` implements the core GitHub OAuth flow. It exchanges the OAuth code for a GitHub token, provisions the user, and securely injects the token into the database.
- `crypto.go`: Implements AES-256 stream cipher encryption. Used specifically to encrypt `GithubToken` at rest in the database so plaintext tokens are never exposed.
- `git_team.go`: Handles `Commit`, `Sync` (Pull), and `Push`. It executes local git commands in the cloned workspaces. Crucially, it temporarily injects the decrypted GitHub token into the local git config (`git config --local url."https://x-access-token:<token>@github.com".insteadOf "https://github.com/"`) to perform secure pushes without leaking the token into process arguments or logs.
- `ide.go`: Handles the file explorer. Contains endpoints to read, write, create, and delete files directly on the host filesystem (`/workspaces/<id>`).
- `ws.go`: Manages the WebSocket Hub. It allows the frontend to stream real-time build and application logs from the backend.
- `metrics.go`: Exposes a Prometheus-compatible `/metrics` endpoint for observability.

### `backend/provider/` (Orchestration & Docker Interaction)
This is the core engine that talks to Docker and makes sandboxes work.
- `docker_engine.go`: The primary interface to the host Docker daemon.
    - `ProvisionDevSandbox()`: Takes a locally cloned repository and starts a container. It explicitly drops all Linux capabilities (`CapDrop: []string{"ALL"}`), sets strict limits, configures private Docker networks, and bind-mounts the host directory into the container.
    - `CloneOrFetch()`: Securely clones a GitHub repository locally. It utilizes the injected GitHub OAuth token to clone private repositories.
- `dev_runtime.go`: The **intelligence** of the system. `DetectDevRuntime()` scans a repository's contents (looking for `package.json`, `go.mod`, `requirements.txt`, etc.) and determines the correct Docker base image (e.g., `node:20-alpine`, `golang:1.22`). It dynamically generates a `sandbox-start.sh` script that is injected into the container to install dependencies (e.g., `npm install`) and start a hot-reloading watcher (e.g., `nodemon -L`, `air`, `watchmedo`).
- `db_requirements.go` & `sidecar_db.go`: Scans the repository for database hints (Prisma, Laravel, Rails, Django). If it finds them, `StartSidecarDatabase()` provisions a dedicated, ephemeral Postgres/MySQL/Redis container. It dynamically generates random credentials and injects them as `DATABASE_URL` environment variables into the main User Sandbox container.

### `backend/worker/` (Asynchronous Job Processing)
- `worker.go`: Powered by Asynq/Redis. When a user creates a sandbox, the API enqueues a job rather than blocking the HTTP request. The worker picks up the job, executes the clone operation (`provider.CloneOrFetch`), runs the runtime detection (`provider.DetectDevRuntime`), provisions sidecar databases (`provider.StartSidecarDatabase`), and finally starts the sandbox container (`provider.ProvisionDevSandbox`). 
- **Idempotency**: It handles retries gracefully by cleaning up existing containers if a job fails halfway.

### `backend/models/` (Database Schema)
- `models.go`: Defines the relational structure for GORM. Includes `User`, `Project`, `Environment` (Sandboxes), `Organization`, and `Log`. Critically, the `User` model includes the `GithubID` and `GithubToken` (marked securely to never leak in JSON serialization).

### `backend/cron/` (Scheduled Tasks)
- `cleanup.go`: A periodic job that queries the database for environments in a `RUNNING` state whose `UpdatedAt` timestamp is older than `IDLE_TIMEOUT_HOURS`. It gracefully shuts down these containers to save server resources, reinforcing the "ephemeral" nature of the sandboxes.

---

## 4. Frontend Implementation (Next.js App Router)

The frontend provides the UI for authentication, project management, and the IDE. It uses Tailwind CSS for styling and `react-hot-toast` for notifications.

### `frontend/app/(main)/environments/[id]/page.tsx` (The Web IDE)
This is the most complex page in the application.
- **File Explorer**: A tree-view component that fetches the directory structure via `/api/environments/:id/git-tree` and allows users to click files to view them.
- **Code Editor**: Allows local edits to files (saved via `/api/environments/:id/files/content`).
- **Live Logs**: Connects to the backend WebSocket (`/api/ws/environments/:id`) to stream standard output from the running Docker container in real-time.
- **Git Controls**: Provides "Commit Changes" and "Push to GitHub" buttons. 
- **Safety Mechanism**: If a user tries to delete a sandbox with uncommitted local changes, the page forcefully interrupts them with a warning that the changes will be permanently lost unless pushed.

### `frontend/app/login/page.tsx` & `frontend/app/register/page.tsx`
- Traditional email/password forms alongside the new, primary **"Continue with GitHub"** buttons that redirect to the backend OAuth endpoints.

---

## 5. Infrastructure & Configuration

### `docker-compose.yml`
- In the original design, this file ran the Backend and Frontend. 
- **Now**, it *strictly* runs the infrastructure dependencies: `postgres` (Main database), `redis` (Queue & Rate limiting), and `traefik` (Reverse Proxy).
- The Backend and Frontend run directly on the host (Bare Metal) to avoid Docker volume mapping complexities (sandbox inception).

### `.env.example`
- Contains the required variables for the system.
- `JWT_SECRET`: For signing user sessions.
- `GITHUB_CLIENT_ID` & `GITHUB_CLIENT_SECRET`: For OAuth integration.
- `TOKEN_ENCRYPTION_KEY`: A 32-byte AES key used strictly for encrypting the GitHub tokens at rest.
- `DATABASE_URL` & `REDIS_URL`: Pointers to the local infrastructure.

---

## Summary of the Lifecycle Flow

1. **Auth**: User logs in with GitHub. Backend encrypts their token.
2. **Create Sandbox**: User clicks "New Environment" and selects a GitHub repo.
3. **Queue**: Backend inserts an `Environment` row (status: `BUILDING`) and enqueues an Asynq task.
4. **Worker Execution**:
   - Decrypts user's GitHub token.
   - Securely clones the repo to `/home/user/api-sandbox/backend/workspaces/<env-id>`.
   - Scans for sidecar database dependencies (e.g., PostgreSQL). Starts the sidecar DB.
   - Detects the language runtime (e.g., Node.js).
   - Generates `sandbox-start.sh` (e.g., `npm install && npx nodemon -L index.js`).
   - Provisions a `node:alpine` Docker container.
   - Bind-mounts `/home/user/api-sandbox/backend/workspaces/<env-id>` to `/app` inside the container.
   - Connects it to the sidecar database network.
5. **Live Dev**: The frontend streams the logs. The user edits code in the browser IDE. The Go backend writes the edits to the host disk. The container's `nodemon -L` sees the host disk change and instantly restarts the app.
6. **Persistence**: The user clicks "Commit & Push". The Go backend securely injects the OAuth token into the local git config, commits the code, and pushes it directly to the user's GitHub repository.
