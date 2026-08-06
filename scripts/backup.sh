#!/bin/bash
# Backup script for API Sandbox Database

set -e

# Configuration
DB_CONTAINER=${DB_CONTAINER:-"frontend-postgres-1"}
DB_USER=${DB_USER:-"postgres"}
DB_NAME=${DB_NAME:-"api_sandbox"}
BACKUP_DIR=${BACKUP_DIR:-"$HOME/backups/api_sandbox"}
DATE=$(date +%Y%m%d_%H%M%S)
S3_BUCKET=${S3_BACKUP_BUCKET:-"s3://api-sandbox-backups"}
RETENTION_DAYS=30

echo "Starting database backup at $DATE..."

# Ensure backup directory exists
mkdir -p $BACKUP_DIR

# Create backup using pg_dump inside the postgres container
BACKUP_FILE="$BACKUP_DIR/db-$DATE.sql.gz"
echo "Running pg_dump..."
docker exec -t $DB_CONTAINER pg_dump -U $DB_USER -d $DB_NAME | gzip > $BACKUP_FILE

echo "Backup saved to $BACKUP_FILE"

# Upload to S3 if AWS CLI is configured
if command -v aws &> /dev/null; then
    echo "Uploading to S3 ($S3_BUCKET)..."
    aws s3 cp $BACKUP_FILE $S3_BUCKET/db-$DATE.sql.gz
    echo "Upload complete."
else
    echo "WARNING: AWS CLI not found. Skipping S3 upload."
    echo "To enable off-site backups, install aws-cli and configure credentials."
fi

# Clean up old backups locally
echo "Cleaning up backups older than $RETENTION_DAYS days..."
find $BACKUP_DIR -name "db-*.sql.gz" -type f -mtime +$RETENTION_DAYS -delete

echo "Backup process finished successfully."
