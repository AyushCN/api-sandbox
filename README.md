# API Sandbox Orchestration Platform

A powerful, full-stack environment provisioning and sandboxing platform. This project allows users to deploy and manage Dockerized GitHub repositories on-demand through a sleek web dashboard, orchestrating the container lifecycle behind the scenes.

## 🚀 Key Features Achieved

### Security & Multi-Tenancy (New!)
*   **User Authentication:** Complete JWT-based registration and login system with securely hashed passwords using `bcrypt`.
*   **Data Isolation:** Robust multi-tenant architecture where environments, logs, and metrics are strictly filtered and isolated by `UserID`.
*   **Frontend Auth Context:** Secure Next.js routing with an `AuthProvider` that handles token lifecycles and automatically attaches Bearer tokens to all outbound `fetchWithAuth` requests.

### Public URL Routing (New!)
*   **Traefik Reverse Proxy:** Automatically scans new Docker containers for labels and instantly maps wildcard subdomains (`http://[env-id].localhost`) to the dynamically allocated exposed port of the container. No Nginx configuration required!

### Backend Orchestration (Go / Gin / Asynq)
*   **Docker Engine Integration:** Directly interfaces with the Docker Daemon via `fsouza/go-dockerclient` to build images from tarballs and manage containers.
*   **Dynamic Port Mapping:** Automatically binds internal exposed ports to host ports using `PublishAllPorts: true`, supporting any port (5000, 8080, 80) seamlessly.
*   **Asynchronous Task Queue:** Uses `hibiken/asynq` with Redis to handle long-running Docker build tasks in the background without blocking the API.
*   **Robust Container Lifecycle:** Handles everything from `git clone` (with branch fallbacks), tarball creation, image building, starting, and gracefully stopping/removing exited containers.
*   **State Management:** Stores user data, environment metadata, timestamps, and streaming log history securely in PostgreSQL via GORM.

### Frontend Dashboard (Next.js / React / Tailwind)
*   **Sleek Modern UI:** Built with dark mode, glassmorphism elements, and responsive layouts.
*   **Live Terminal Output:** Integrates `xterm.js` to render Docker build logs in a native-feeling terminal window.
*   **Real-time Log Polling:** Bypasses Next.js API proxy buffering bugs by utilizing highly reliable `useSWR` polling for real-time log appending.
*   **Dynamic GitHub Branching:** Automatically fetches and displays available branches for a given GitHub repository using the public GitHub API before deployment (with silent failure fallbacks for API rate limits).
*   **Environment Management:** One-click deployment, status monitoring (Building, Running, Failed, Stopped), URL generation, and container deletion.

## 🛠️ Tech Stack

*   **Go** (Backend API & Worker)
*   **Gin** (HTTP Router & JWT Middleware)
*   **GORM** (PostgreSQL ORM)
*   **Asynq & Redis** (Background Job Queue)
*   **Docker Engine API & Traefik** (Containerization & Reverse Proxy)
*   **Next.js 14 & React** (Frontend Dashboard & Auth Provider)
*   **Tailwind CSS** (Styling)
*   **xterm.js** (In-browser Terminal)

## 🏃 Getting Started

### 1. Start Infrastructure Services
Ensure you have Redis, PostgreSQL, and Traefik running.
```bash
docker start api-sandbox-postgres-1 api-sandbox-redis-1 traefik
```

### 2. Run the Go Backend & Worker
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

## 📝 Recent Fixes & Milestones
*   **Phase 1 Completion:** Fully implemented Priority 1 (Auth, Ownership, and Public Routing) from the Evaluation specs.
*   Fixed container crashing invisibly by exposing Docker runtime exit codes.
*   Fixed the `PublishAllPorts` mapping, completely eliminating hardcoded Port 3000/8080 constraints.
*   Replaced buggy Server-Sent Events (SSE) with robust SWR polling for flawless real-time terminal log rendering.
*   Implemented full cleanup lifecycle for deleted sandboxes to prevent orphaned Docker containers and free up system resources.
