-- 000020_drop_departments_company_id_and_standardize_soft_deletes.down.sql
-- 1. Restore company_id on departments
ALTER TABLE departments ADD COLUMN IF NOT EXISTS company_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_departments_company_id ON departments(company_id);
DROP INDEX IF EXISTS idx_departments_name;

-- 2. Restore idx_user_date unique index on daily_activities
DROP INDEX IF EXISTS idx_daily_activities_user_date_active;
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_date ON daily_activities(user_id, date);
