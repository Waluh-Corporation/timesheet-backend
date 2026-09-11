#!/usr/bin/env bash
# ==============================================================================
# Database Administrator (DBA) Disaster Recovery / Restore Script
# Timesheet Automation Portal - PostgreSQL
# SLA Compliance: RTO < 1 hour
# ==============================================================================

set -euo pipefail

CONTAINER_NAME="${PG_CONTAINER:-timesheet_postgres}"
DB_USER="${POSTGRES_USER:-timesheet}"
DB_NAME="${POSTGRES_DB:-timesheet}"

if [ $# -lt 1 ]; then
    echo "Usage: $0 <path_to_backup.dump> [target_db_name]"
    echo "Example: $0 ./backups/timesheet_backup_20260909_080000.dump timesheet"
    exit 1
fi

BACKUP_FILE="$1"
TARGET_DB="${2:-$DB_NAME}"

if [ ! -f "${BACKUP_FILE}" ]; then
    echo "ERROR: Backup file '${BACKUP_FILE}' does not exist!"
    exit 1
fi

echo "=========================================================="
echo "DISASTER RECOVERY / DATABASE RESTORE"
echo "Backup File : ${BACKUP_FILE}"
echo "Target DB   : ${TARGET_DB}"
echo "Container   : ${CONTAINER_NAME}"
echo "Time        : $(date)"
echo "=========================================================="

# 1. Verify archive integrity before attempting restore
echo "Verifying backup catalog..."
if ! docker exec -i "${CONTAINER_NAME}" pg_restore --list < "${BACKUP_FILE}" > /dev/null; then
    echo "FATAL: Archive file is corrupted or not a valid PostgreSQL custom dump!"
    exit 2
fi
echo "Archive integrity verified."

# 2. Perform restoration
echo "Restoring database into '${TARGET_DB}'..."
# --clean: drop database objects before recreating them
# --if-exists: avoid errors if objects don't exist
docker exec -i "${CONTAINER_NAME}" pg_restore \
    -U "${DB_USER}" \
    -d "${TARGET_DB}" \
    --clean \
    --if-exists \
    --no-owner \
    --no-privileges \
    < "${BACKUP_FILE}" || true

# 3. Post-restore smoke validation
echo "Executing post-recovery smoke test..."
TABLE_COUNT=$(docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${TARGET_DB}" -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';")
USER_COUNT=$(docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${TARGET_DB}" -t -c "SELECT COUNT(*) FROM users;")

echo "Post-recovery status:"
echo " - Tables restored: $(echo "${TABLE_COUNT}" | tr -d '[:space:]')"
echo " - Users restored : $(echo "${USER_COUNT}" | tr -d '[:space:]')"

echo "Database restoration completed successfully."
