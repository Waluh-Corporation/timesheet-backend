-- 000026_security_hardening_and_db_optimizations.down.sql
-- Revert 000026 changes

-- 1. Restore standalone indexes on is_active if needed
CREATE INDEX IF NOT EXISTS idx_companies_is_active ON companies(is_active);
CREATE INDEX IF NOT EXISTS idx_departments_is_active ON departments(is_active);
CREATE INDEX IF NOT EXISTS idx_sites_active ON sites(is_active);
CREATE INDEX IF NOT EXISTS idx_divisions_active ON divisions(is_active);
CREATE INDEX IF NOT EXISTS idx_daily_activities_is_active ON daily_activities(is_active);
CREATE INDEX IF NOT EXISTS idx_overtime_entries_active ON overtime_entries(is_active);
CREATE INDEX IF NOT EXISTS idx_approvers_is_active ON approvers(is_active);
CREATE INDEX IF NOT EXISTS idx_projects_is_active ON projects(is_active);

-- 2. Drop trigram indexes
DROP INDEX IF EXISTS idx_projects_name_trgm;
DROP INDEX IF EXISTS idx_departments_name_trgm;
DROP INDEX IF EXISTS idx_companies_name_trgm;

-- 3. Drop functional indexes
DROP INDEX IF EXISTS idx_projects_lower_code;
DROP INDEX IF EXISTS idx_departments_lower_code;
DROP INDEX IF EXISTS idx_companies_lower_code;

-- 4. Drop covering index
DROP INDEX IF EXISTS idx_daily_activities_range_covering;

-- 5. Drop refresh_tokens table
DROP TABLE IF EXISTS refresh_tokens;
