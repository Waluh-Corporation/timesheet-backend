-- 000004_enforce_referential_integrity_and_indexes.up.sql
-- Normalization phase 3: Enforce referential integrity on activity status, overtime relations, and add performance indexes

-- 1. Clean up and standardize daily_activities.status values before applying foreign key constraint
UPDATE daily_activities
SET status = 'P'
WHERE status IS NULL OR TRIM(status) = '';

UPDATE daily_activities
SET status = UPPER(TRIM(status))
WHERE status IS NOT NULL;

-- Default any unmapped status to 'P' (Present)
UPDATE daily_activities
SET status = 'P'
WHERE status NOT IN (SELECT code FROM activity_statuses);

-- 2. Enforce foreign key constraint on daily_activities(status) -> activity_statuses(code)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_daily_activities_status'
    ) THEN
        ALTER TABLE daily_activities
        ADD CONSTRAINT fk_daily_activities_status
        FOREIGN KEY (status) REFERENCES activity_statuses(code)
        ON UPDATE CASCADE ON DELETE RESTRICT;
    END IF;
END $$;

-- 3. Ensure foreign key constraints on overtime_entries
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_overtimes_daily_activity'
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.table_constraints 
            WHERE table_name = 'overtime_entries' AND constraint_type = 'FOREIGN KEY' AND constraint_name LIKE '%daily_activity%'
        ) THEN
            ALTER TABLE overtime_entries
            ADD CONSTRAINT fk_overtimes_daily_activity
            FOREIGN KEY (daily_activity_id) REFERENCES daily_activities(id)
            ON UPDATE CASCADE ON DELETE SET NULL;
        END IF;
    END IF;
END $$;

-- 4. Performance indexes for frequent queries, reports, and joins
CREATE INDEX IF NOT EXISTS idx_daily_activities_status ON daily_activities(status);
CREATE INDEX IF NOT EXISTS idx_projects_company_active ON projects(company_id, is_active);
CREATE INDEX IF NOT EXISTS idx_departments_company_active ON departments(company_id, is_active);
CREATE INDEX IF NOT EXISTS idx_overtime_entries_user_date ON overtime_entries(user_id, date);
