-- 000019_decouple_department_company_and_admin_company.down.sql
-- 1. Drop admin company restriction
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_admin_no_company;

-- 2. Restore departments foreign key constraint referencing companies(id)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'departments_company_id_fkey'
    ) THEN
        ALTER TABLE departments
        ADD CONSTRAINT departments_company_id_fkey
        FOREIGN KEY (company_id) REFERENCES companies(id)
        ON UPDATE CASCADE ON DELETE SET NULL;
    END IF;
END $$;
