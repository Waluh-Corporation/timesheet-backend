-- 000004_enforce_referential_integrity_and_indexes.down.sql
-- Rollback for normalization phase 3

ALTER TABLE daily_activities DROP CONSTRAINT IF EXISTS fk_daily_activities_status;
ALTER TABLE overtime_entries DROP CONSTRAINT IF EXISTS fk_overtimes_daily_activity;

DROP INDEX IF EXISTS idx_overtime_entries_user_date;
DROP INDEX IF EXISTS idx_departments_company_active;
DROP INDEX IF EXISTS idx_projects_company_active;
DROP INDEX IF EXISTS idx_daily_activities_status;
