-- 000014_drop_holidays_company_id.up.sql
-- Remove company_id column and foreign key constraint from holidays table

-- 1. Drop foreign key constraint if it exists
ALTER TABLE holidays DROP CONSTRAINT IF EXISTS holidays_company_id_fkey;

-- 2. Drop index if it exists
DROP INDEX IF EXISTS idx_holidays_company_id;

-- 3. Drop column
ALTER TABLE holidays DROP COLUMN IF EXISTS company_id;
