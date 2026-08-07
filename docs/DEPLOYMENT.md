# API Sandbox Deployment Guide

This guide details how to run the API Sandbox in a production environment.

## 1. Prerequisites

- A Linux server (e.g. Linode, DigitalOcean, AWS EC2)
- Docker and Docker Compose installed
- A domain name pointing to your server's IP address (e.g., `api.yourdomain.com` and `*.yourdomain.com` for environments)

## 2. Environment Variables

Create `.env.production` files in both `frontend` and `backend` directories.

**Backend `.env`:**
```env
# Database & Cache
DATABASE_URL=postgresql://postgres:postgres@postgres:5432/api_sandbox?sslmode=disable
REDIS_URL=redis://redis:6379

# Auth
JWT_SECRET=your_super_secure_jwt_secret_here

# App URL & Traefik Routing
APP_URL=https://api.yourdomain.com
DOMAIN=yourdomain.com

# Email Configuration (SendGrid Recommended)
SENDGRID_API_KEY=SG.your_key_here
# SMTP Fallback (If SendGrid not used)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your_email@gmail.com
SMTP_PASS=your_app_password
SMTP_FROM=no-reply@yourdomain.com
```

## 3. Infrastructure (docker-compose)

The infrastructure services (Traefik, PostgreSQL, Redis) are defined in `frontend/docker-compose.yml`.

To enable HTTPS with Let's Encrypt:
1. Ensure `traefik` is configured with ACME challenges in `docker-compose.yml`.
2. Map a volume for `acme.json` to persist certificates.

Run the infrastructure:
```bash
cd frontend
docker compose up -d
```

## 4. Run the Backend

The backend is compiled via Go and run on the host (or inside another Docker container).
For host-based deployments (as the orchestrator connects to the Docker socket):

```bash
cd backend
go build -o server .
# Run with GIN_MODE=release to enforce Secure cookies
GIN_MODE=release ./server
```

## 5. Run the Frontend

Create a `.env.production` file in the `frontend` directory:

```env
# Required for the Next.js API proxy to route requests to your backend
BACKEND_URL=http://localhost:8080
```

Build and start the Next.js app:
```bash
cd frontend
npm run build
npm start
```

## 6. Backups

A backup script is provided in `scripts/backup.sh`.
To run it daily, add it to your crontab:

```bash
crontab -e
# Add the following line to run at 2 AM daily
0 2 * * * /path/to/api-sandbox/scripts/backup.sh
```
Ensure you have the AWS CLI configured if you wish to upload backups to S3.
