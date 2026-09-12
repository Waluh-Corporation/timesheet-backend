-- 000020_drop_departments_company_id_and_standardize_soft_deletes.up.sql
-- 1. Decouple departments from companies: drop foreign key, unique constraint, index, and company_id column
ALTER TABLE departments DROP CONSTRAINT IF EXISTS departments_company_id_fkey;
ALTER TABLE departments DROP CONSTRAINT IF EXISTS fk_departments_company;
ALTER TABLE departments DROP CONSTRAINT IF EXISTS fk_companies_departments;
ALTER TABLE departments DROP CONSTRAINT IF EXISTS uq_departments_company_name;
DROP INDEX IF EXISTS idx_departments_company_id;
ALTER TABLE departments DROP COLUMN IF EXISTS company_id;

-- 2. Ensure departments has is_active boolean column (migrate is_delete if present)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'departments' AND column_name = 'is_delete'
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'departments' AND column_name = 'is_active'
        ) THEN
            ALTER TABLE departments ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT true;
            UPDATE departments SET is_active = NOT is_delete;
        END IF;
        ALTER TABLE departments DROP COLUMN is_delete;
    END IF;
END $$;

ALTER TABLE departments ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;
CREATE INDEX IF NOT EXISTS idx_departments_is_active ON departments(is_active);
CREATE INDEX IF NOT EXISTS idx_departments_name ON departments(name);

-- 3. Add is_active column to companies
ALTER TABLE companies ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;
CREATE INDEX IF NOT EXISTS idx_companies_is_active ON companies(is_active);

-- 4. Add is_active column to overtime_entries
ALTER TABLE overtime_entries ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;
CREATE INDEX IF NOT EXISTS idx_overtime_entries_user_date_active ON overtime_entries(user_id, date, is_active);

-- 5. Add is_active column and partial unique index to daily_activities
ALTER TABLE daily_activities ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;
CREATE INDEX IF NOT EXISTS idx_daily_activities_is_active ON daily_activities(is_active);

-- Drop obsolete full unique index on (user_id, date) so re-entry on soft-deleted date creates a new record
DROP INDEX IF EXISTS idx_user_date;
CREATE UNIQUE INDEX IF NOT EXISTS idx_daily_activities_user_date_active ON daily_activities(user_id, date) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_daily_activities_user_date ON daily_activities(user_id, date);

-- 6. Guarantee initial master data with ID 1
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM companies WHERE id = 1) THEN
        IF EXISTS (SELECT 1 FROM companies WHERE LOWER(code) = 'mii') THEN
            UPDATE companies SET id = 1 WHERE LOWER(code) = 'mii';
        ELSE
            INSERT INTO companies (id, code, name, is_active)
            VALUES (1, 'mii', 'PT Mitra Integrasi Informatika', true)
            ON CONFLICT (id) DO NOTHING;
        END IF;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM departments WHERE id = 1) THEN
        IF EXISTS (SELECT 1 FROM departments WHERE LOWER(code) = 'wdl') THEN
            UPDATE departments SET id = 1 WHERE LOWER(code) = 'wdl';
        ELSE
            INSERT INTO departments (id, code, name, division, is_active)
            VALUES (1, 'WDL', 'Wholesale Channel and Service Delivery', 'Wholesale Digital Delivery', true)
            ON CONFLICT (id) DO NOTHING;
        END IF;
    END IF;
END $$;

-- Synchronize sequences for companies and departments
SELECT setval(
    pg_get_serial_sequence('companies', 'id'), 
    COALESCE((SELECT GREATEST(MAX(id), 1) FROM companies), 1)
);
SELECT setval(
    pg_get_serial_sequence('departments', 'id'), 
    COALESCE((SELECT GREATEST(MAX(id), 1) FROM departments), 1)
);

-- 7. Synchronize existing users data with master data
UPDATE users u
SET company = c.name
FROM companies c
WHERE u.company_id = c.id
  AND (u.company IS NULL OR u.company != c.name);

UPDATE users u
SET department = d.name,
    division = COALESCE(NULLIF(u.division, ''), d.division)
FROM departments d
WHERE u.department_id = d.id
  AND (u.department IS NULL OR u.department != d.name);

-- Ensure admin accounts have no company association
UPDATE users 
SET company_id = NULL, company = ''
WHERE role = 'admin' 
  AND (company_id IS NOT NULL OR (company IS NOT NULL AND company != ''));
