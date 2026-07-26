# API Sandbox

A production-grade, self-hosted platform that enables developers to deploy backend services in isolated environments and instantly generate shareable public API endpoints.

This project is built as a unified Next.js 16 monorepo containing both the frontend and the backend API routes, interfacing natively with Docker via `dockerode` to spawn sandbox containers.

## 🚀 Architecture & Tech Stack

- **Framework:** Next.js 16 (App Router, React 19, TypeScript)
- **Database & ORM:** PostgreSQL 16 managed via Prisma ORM
- **Queue & Background Jobs:** Redis 7 with Bull (via a standalone Node.js worker)
- **Containerization:** Docker & Dockerode (Node.js Docker API client)
- **Validation:** Zod
- **Styling:** Tailwind CSS (Coming in Phase 3)

## 🛠️ Local Development Setup

To run the API Sandbox locally, you will need **Node.js (v20+)** and **Docker Desktop / Docker Engine** installed on your host machine.

### 1. Clone & Install Dependencies
```bash
git clone <repository-url>
cd api-sandbox
npm install
```

### 2. Start the Infrastructure (Database & Redis)
Ensure the Docker daemon is running on your machine, then spin up the PostgreSQL and Redis containers using Docker Compose:
```bash
docker compose up -d
```

### 3. Environment Variables
Create a `.env.local` file in the root directory (this should already be generated with default connection strings if you ran the initial setup):
```env
DATABASE_URL="postgresql://postgres:postgres@localhost:5432/api_sandbox?schema=public"
REDIS_URL="redis://localhost:6379"
```

### 4. Database Migrations
Sync the Prisma schema with the local PostgreSQL database:
```bash
npx prisma db push
```

### 5. Start the Application
You need to run two separate processes for local development: the Next.js web server and the background queue worker.

**Terminal 1 (Next.js Server):**
```bash
npm run dev
```
*The web server will be available at http://localhost:3000*

**Terminal 2 (Background Worker):**
```bash
npm run worker
```
*The worker process listens to the Redis queue, clones the requested GitHub repositories, builds the Docker images, and spawns the isolated containers.*

## 🔌 API Endpoints

### `GET /api/environments`
Returns a list of all sandbox environments ordered by creation date.

### `POST /api/environments`
Submits a new sandbox environment build request.
- **Payload:**
  ```json
  {
    "name": "my-express-app",
    "gitUrl": "https://github.com/expressjs/express",
    "githubBranch": "master"
  }
  ```
- **Action:** Creates a database record and pushes a job to the Redis queue. The background worker picks it up asynchronously.

### `GET /api/environments/[id]`
Retrieves a specific environment by its ID, including its most recent build logs and resource metrics.

## ⚠️ Important Note on Docker Permissions
The background worker requires direct access to the host machine's Docker daemon to spawn containers. It connects via the default UNIX socket `/var/run/docker.sock`. 
If you encounter a `permission denied` error, ensure your current user is in the `docker` group, or run:
```bash
sudo chmod 666 /var/run/docker.sock
```
*(Only do this for local development!)*
