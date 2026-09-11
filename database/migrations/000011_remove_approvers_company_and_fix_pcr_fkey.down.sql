-- 000011_remove_approvers_company_and_fix_pcr_fkey.down.sql
-- Revert 000011 migration

-- 1. Restore company_id on approvers
ALTER TABLE approvers ADD COLUMN IF NOT EXISTS company_id BIGINT;

ALTER TABLE approvers
ADD CONSTRAINT fk_approvers_company
FOREIGN KEY (company_id) REFERENCES companies(id)
ON UPDATE CASCADE ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_approvers_company_role ON approvers(company_id, role_type) WHERE is_active = true;
DROP INDEX IF EXISTS idx_approvers_role_type_active;
