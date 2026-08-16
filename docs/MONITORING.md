# Monitoring & Operations

This document covers baseline operations, monitoring, and database management for a self-hosted Sandbox VM.

## 1. System Monitoring

The Sandbox platform relies on Docker Compose. You can monitor the health of the system via standard Docker metrics and API health checks.

### API Health Polling
Configure your monitoring system (Prometheus, UptimeRobot, Datadog) to poll the health endpoint:
```bash
curl https://<YOUR_DOMAIN>/api/health
```
**Expected Response:** `200 OK` `{"status":"ok","db":"ok","redis":"ok"}`

### Host Resource Constraints
The platform spins up ephemeral containers. Keep an eye on Host Memory and Swap.
- **View Container Stats:** `docker stats`
- **View Host Disk Space:** `df -h`
- **View Inodes Usage:** `df -i` (heavy Node.js environments create many tiny files, which can exhaust inodes before disk space).

## 2. Garbage Collection

The system features two layers of automated garbage collection:
1. **Idle GC:** Environments are automatically stopped, their DB sidecars destroyed, and host disk workspaces wiped after `IDLE_TIMEOUT_HOURS` (default: 6 hours) of inactivity.
2. **Orphan Reaper:** An hourly cron job strictly scans the host Docker daemon for `api-sandbox-env-*` or `api-sandbox-db-*` containers that crashed, failed to delete cleanly, or no longer match an active database row, and forcefully removes them to prevent silent host exhaustion.

## 3. Backups

The platform includes automated scripts to backup the control-plane PostgreSQL database.

> [!WARNING]
> **Scope of Backups:** The database backup *only* captures metadata (users, sandbox URLs, settings). It **does not** backup the live `workspaces/` files on disk. Users are expected to sync and push their workspace changes to GitHub.

### Manual Backup
Run the backup script on the host VM:
```bash
./scripts/backup.sh
```
This drops a `.sql.gz` dump into `./backups/` and automatically prunes backups older than 7 days.

### Restoring a Backup
> [!CAUTION]
> **Downtime Required:** Restoring drops the database connections. You MUST stop the backend application before restoring, otherwise active connections will block the restore.

1. Stop the application services:
   ```bash
   docker compose stop backend
   ```
2. Run the restore script:
   ```bash
   ./scripts/restore.sh ./backups/db_backup_20260816_120000.sql.gz
   ```
3. Restart the application:
   ```bash
   docker compose start backend
   ```

## 4. Troubleshooting Orphan Containers
If you suspect the automated Reaper is missing a stalled container, you can manually prune the host:
```bash
# Stop and remove ALL sandboxes (Warning: disruptive to active users)
docker rm -f $(docker ps -a -q --filter name=api-sandbox-env-)
docker rm -f $(docker ps -a -q --filter name=api-sandbox-db-)

# Wipe all workspaces
sudo rm -rf /var/lib/api-sandbox/workspaces/*
```
