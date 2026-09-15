-- 000018_harden_schema_integrity_and_indexes.up.sql
-- Harden relational integrity, add missing FK indexes, domain CHECK constraints, and entity uniqueness

-- 1. Add missing Foreign Key index on profile_change_requests(reviewed_by)
CREATE INDEX IF NOT EXISTS idx_profile_change_requests_reviewed_by 
ON profile_change_requests(reviewed_by);

-- 2. Drop redundant duplicate index on holidays(date) since holidays_date_key UNIQUE index already covers it
DROP INDEX IF EXISTS idx_holidays_date;

-- 3. Add Entity Uniqueness Constraints
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'uq_projects_code_name'
    ) THEN
        ALTER TABLE projects ADD CONSTRAINT uq_projects_code_name UNIQUE (code, name);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'uq_departments_company_name'
    ) THEN
        ALTER TABLE departments ADD CONSTRAINT uq_departments_company_name UNIQUE (company_id, name);
    END IF;
END $$;

-- 4. Add Domain CHECK Constraints
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_approvers_role_type'
    ) THEN
        ALTER TABLE approvers ADD CONSTRAINT chk_approvers_role_type CHECK (role_type IN ('team_leader', 'department_head'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_users_role'
    ) THEN
        ALTER TABLE users ADD CONSTRAINT chk_users_role CHECK (role IN ('admin', 'user'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_profile_change_requests_status'
    ) THEN
        ALTER TABLE profile_change_requests ADD CONSTRAINT chk_profile_change_requests_status CHECK (status IN ('pending', 'approved', 'rejected'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_daily_activities_time_format'
    ) THEN
        ALTER TABLE daily_activities ADD CONSTRAINT chk_daily_activities_time_format 
        CHECK (
            (start_time IS NULL OR start_time = '' OR start_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$') AND
            (end_time IS NULL OR end_time = '' OR end_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$')
        );
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_overtime_entries_time_format'
    ) THEN
        ALTER TABLE overtime_entries ADD CONSTRAINT chk_overtime_entries_time_format 
        CHECK (
            (start_time IS NULL OR start_time = '' OR start_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$') AND
            (end_time IS NULL OR end_time = '' OR end_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$')
        );
    END IF;
END $$;

-- 5. Add Performance & Analytical Indexes
CREATE INDEX IF NOT EXISTS idx_daily_activities_date ON daily_activities(date);
CREATE INDEX IF NOT EXISTS idx_overtime_entries_date ON overtime_entries(date);
CREATE INDEX IF NOT EXISTS idx_users_role_active ON users(role, is_active) WHERE deleted_at IS NULL;
