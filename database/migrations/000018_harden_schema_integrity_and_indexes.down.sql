-- 000018_harden_schema_integrity_and_indexes.down.sql
-- Revert schema hardening, remove constraints, indexes, and restore idx_holidays_date

-- 1. Drop Performance & Analytical Indexes
DROP INDEX IF EXISTS idx_users_role_active;
DROP INDEX IF EXISTS idx_overtime_entries_date;
DROP INDEX IF EXISTS idx_daily_activities_date;

-- 2. Drop Domain CHECK Constraints
ALTER TABLE overtime_entries DROP CONSTRAINT IF EXISTS chk_overtime_entries_time_format;
ALTER TABLE daily_activities DROP CONSTRAINT IF EXISTS chk_daily_activities_time_format;
ALTER TABLE profile_change_requests DROP CONSTRAINT IF EXISTS chk_profile_change_requests_status;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_role;
ALTER TABLE approvers DROP CONSTRAINT IF EXISTS chk_approvers_role_type;

-- 3. Drop Entity Uniqueness Constraints
ALTER TABLE departments DROP CONSTRAINT IF EXISTS uq_departments_company_name;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS uq_projects_code_name;

-- 4. Restore holidays date index
CREATE INDEX IF NOT EXISTS idx_holidays_date ON holidays(date);

-- 5. Drop FK index on profile_change_requests(reviewed_by)
DROP INDEX IF EXISTS idx_profile_change_requests_reviewed_by;
