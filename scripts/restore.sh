#!/bin/bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <backup_file.sql.gz>"
    exit 1
fi

BACKUP_FILE=$1

if [ ! -f "$BACKUP_FILE" ]; then
    echo "Error: Backup file not found: $BACKUP_FILE"
    exit 1
fi

# PostgreSQL Docker container name
DB_CONTAINER="${DB_CONTAINER:-api-sandbox-postgres-1}"
# Database name inside the container
DB_NAME="${DB_NAME:-api_sandbox}"
# Database user
DB_USER="${DB_USER:-postgres}"

echo "========================================================================="
echo "WARNING: This will overwrite the current database."
echo "Please ensure the backend API and worker containers are STOPPED"
echo "to prevent active connections from blocking the restore."
echo "Run: docker compose stop backend"
echo "========================================================================="
read -p "Are you sure you want to proceed? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Restore aborted."
    exit 1
fi

echo "Restoring database from: $BACKUP_FILE"

# Uncompress and pipe to psql
gunzip -c "$BACKUP_FILE" | docker exec -i "$DB_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME"

echo "Restore completed successfully."
