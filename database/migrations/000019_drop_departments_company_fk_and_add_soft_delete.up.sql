-- 000019_decouple_department_company_and_admin_company.up.sql
-- 1. Drop foreign key constraints linking departments.company_id to companies.id
ALTER TABLE departments DROP CONSTRAINT IF EXISTS departments_company_id_fkey;
ALTER TABLE departments DROP CONSTRAINT IF EXISTS fk_departments_company;
ALTER TABLE departments DROP CONSTRAINT IF EXISTS fk_companies_departments;

-- 2. Ensure existing admin users have no company association
UPDATE users 
SET company_id = NULL, company = NULL 
WHERE role = 'admin';

-- 3. Enforce that admin accounts do not relate to any company
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_users_admin_no_company'
    ) THEN
        ALTER TABLE users 
        ADD CONSTRAINT chk_users_admin_no_company 
        CHECK (role != 'admin' OR company_id IS NULL);
    END IF;
END $$;
