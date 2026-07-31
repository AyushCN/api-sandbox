# API Sandbox - Frontend

This directory contains the Next.js frontend dashboard for the API Sandbox platform.

**Note:** The architecture of this platform has evolved. The backend, orchestration worker, and database interactions have been fully migrated to a highly performant **Go backend** (located in the `/backend` directory). 

Please see the root `README.md` for complete, up-to-date documentation, architecture details, and local development instructions.

## 🏃 Local Development (Frontend Only)

To run the Next.js frontend dashboard locally:

1. Ensure the Go backend and infrastructure (PostgreSQL, Redis, Traefik) are running (see root `README.md`).
2. Install dependencies:
   ```bash
   npm install
   ```
3. Start the development server:
   ```bash
   npm run dev
   ```
4. Open [http://localhost:3000](http://localhost:3000) with your browser.
