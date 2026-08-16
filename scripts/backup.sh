#!/bin/bash
set -euo pipefail

# Directory to store backups on the host
BACKUP_DIR="${BACKUP_DIR:-./backups}"
# PostgreSQL Docker container name
DB_CONTAINER="${DB_CONTAINER:-api-sandbox-postgres-1}"
# Database name inside the container
DB_NAME="${DB_NAME:-api_sandbox}"
# Database user
DB_USER="${DB_USER:-postgres}"

mkdir -p "$BACKUP_DIR"

TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="$BACKUP_DIR/db_backup_$TIMESTAMP.sql.gz"

echo "Creating database backup: $BACKUP_FILE"

# Execute pg_dump inside the running postgres container
docker exec "$DB_CONTAINER" pg_dump -U "$DB_USER" -d "$DB_NAME" --clean | gzip > "$BACKUP_FILE"

echo "Backup created successfully."

# Rotate backups (keep last 7 days)
echo "Cleaning up backups older than 7 days..."
find "$BACKUP_DIR" -type f -name "db_backup_*.sql.gz" -mtime +7 -exec rm {} \;
echo "Done."
