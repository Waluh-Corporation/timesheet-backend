#!/usr/bin/env bash
# ==============================================================================
# Database Administrator (DBA) Maintenance & Health Check Script
# Timesheet Automation Portal - PostgreSQL
# Focus: Performance Tuning, Buffer Pool, Statistics, and Storage Optimization
# ==============================================================================

set -euo pipefail

CONTAINER_NAME="${PG_CONTAINER:-timesheet_postgres}"
DB_USER="${POSTGRES_USER:-timesheet}"
DB_NAME="${POSTGRES_DB:-timesheet}"

echo "=========================================================="
echo "PostgreSQL DBA Health & Performance Report"
echo "Database  : ${DB_NAME}"
echo "Container : ${CONTAINER_NAME}"
echo "Timestamp : $(date)"
echo "=========================================================="

echo ""
echo "--- 1. Database & Table Sizes ---"
docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "
SELECT
    relname AS table_name,
    pg_size_pretty(pg_total_relation_size(relid)) AS total_size,
    pg_size_pretty(pg_relation_size(relid)) AS data_size,
    pg_size_pretty(pg_total_relation_size(relid) - pg_relation_size(relid)) AS index_size
FROM pg_catalog.pg_statio_user_tables
ORDER BY pg_total_relation_size(relid) DESC;
"

echo ""
echo "--- 2. Active Connections & States ---"
docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "
SELECT 
    state, 
    count(*) AS connection_count 
FROM pg_stat_activity 
GROUP BY state;
"

echo ""
echo "--- 3. Index Usage Statistics (Scan Efficiency) ---"
docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "
SELECT
    relname AS table_name,
    seq_scan,
    seq_tup_read,
    idx_scan,
    idx_tup_fetch
FROM pg_stat_user_tables
ORDER BY seq_scan DESC;
"

echo ""
echo "--- 4. Cache Hit Ratio (Target: > 99%) ---"
docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "
SELECT
    sum(heap_blks_read) as heap_read,
    sum(heap_blks_hit)  as heap_hit,
    round(sum(heap_blks_hit) / nullif(sum(heap_blks_hit) + sum(heap_blks_read), 0)::numeric * 100, 2) as cache_hit_pct
FROM pg_statio_user_tables;
"

echo ""
echo "--- 5. Running VACUUM ANALYZE to update statistics & reclaim dead tuples ---"
docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "VACUUM ANALYZE;"
echo "VACUUM ANALYZE completed successfully."
