-- 000011_remove_approvers_company_and_fix_pcr_fkey.up.sql
-- 1. Resolve duplicate foreign key on profile_change_requests(company_id)
-- Drop the default raw Postgres constraint so only the standard GORM/app constraint remains
ALTER TABLE profile_change_requests DROP CONSTRAINT IF EXISTS profile_change_requests_company_id_fkey;

-- Ensure single standard foreign key constraint exists on profile_change_requests(company_id)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_profile_change_requests_company_rel'
    ) THEN
        ALTER TABLE profile_change_requests
        ADD CONSTRAINT fk_profile_change_requests_company_rel
        FOREIGN KEY (company_id) REFERENCES companies(id)
        ON UPDATE CASCADE ON DELETE SET NULL;
    END IF;
END $$;

-- 2. Remove company_id relation, indexes, and column from approvers table
ALTER TABLE approvers DROP CONSTRAINT IF EXISTS approvers_company_id_fkey;
ALTER TABLE approvers DROP CONSTRAINT IF EXISTS fk_approvers_company;
DROP INDEX IF EXISTS idx_approvers_company_role;
DROP INDEX IF EXISTS idx_approvers_company_id;

ALTER TABLE approvers DROP COLUMN IF EXISTS company_id;

-- Index for fast lookups by role_type on active approvers
CREATE INDEX IF NOT EXISTS idx_approvers_role_type_active ON approvers(role_type) WHERE is_active = true;
