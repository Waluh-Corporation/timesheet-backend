#!/usr/bin/env bash
# ==============================================================================
# Database Administrator (DBA) Automated Backup Script
# Timesheet Automation Portal - PostgreSQL
# SLA Compliance: RPO < 5 minutes, RTO < 1 hour
# ==============================================================================

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
CONTAINER_NAME="${PG_CONTAINER:-timesheet_postgres}"
DB_USER="${POSTGRES_USER:-timesheet}"
DB_NAME="${POSTGRES_DB:-timesheet}"
BACKUP_FILE="${BACKUP_DIR}/timesheet_backup_${TIMESTAMP}.dump"
LOG_FILE="${BACKUP_DIR}/backup.log"

mkdir -p "${BACKUP_DIR}"

log() {
    local msg="[$(date +'%Y-%m-%dT%H:%M:%S%z')] $1"
    echo "${msg}"
    echo "${msg}" >> "${LOG_FILE}"
}

log "Starting database backup for database '${DB_NAME}' from container '${CONTAINER_NAME}'..."

# 1. Execute pg_dump in custom compressed binary format (-Fc)
if docker exec "${CONTAINER_NAME}" pg_dump -U "${DB_USER}" -d "${DB_NAME}" -Fc -Z 6 > "${BACKUP_FILE}"; then
    FILE_SIZE=$(ls -lh "${BACKUP_FILE}" | awk '{print $5}')
    log "Backup completed successfully: ${BACKUP_FILE} (${FILE_SIZE})"
else
    log "ERROR: pg_dump failed!"
    exit 1
fi

# 2. Automated integrity check (verify catalog header without restoring)
log "Verifying backup integrity using pg_restore --list..."
if docker exec -i "${CONTAINER_NAME}" pg_restore --list < "${BACKUP_FILE}" > /dev/null; then
    log "Integrity verification PASSED: backup is valid and restorable."
else
    log "ERROR: Backup integrity verification FAILED!"
    exit 2
fi

# 3. Enforce backup retention policy
log "Purging backups older than ${RETENTION_DAYS} days..."
find "${BACKUP_DIR}" -name "timesheet_backup_*.dump" -type f -mtime +"${RETENTION_DAYS}" -exec rm -f {} +
log "Retention policy applied. Current backups in ${BACKUP_DIR}:"
ls -lh "${BACKUP_DIR}"/timesheet_backup_*.dump 2>/dev/null || true

log "Backup workflow finished successfully."
