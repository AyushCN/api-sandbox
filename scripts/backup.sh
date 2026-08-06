#!/bin/bash
# scripts/backup.sh

BACKUP_DIR="/backups"
mkdir -p $BACKUP_DIR

# Backup database
pg_dump api_sandbox | gzip > $BACKUP_DIR/db-$(date +%Y%m%d-%H%M%S).sql.gz

# Upload to S3 (if configured)
if [ ! -z "$AWS_ACCESS_KEY_ID" ]; then
  aws s3 cp $BACKUP_DIR/ s3://my-backups/ --recursive
fi

# Keep only 30 days of backups
find $BACKUP_DIR -mtime +30 -delete

echo "Backup completed at $(date)" >> /var/log/backups.log
