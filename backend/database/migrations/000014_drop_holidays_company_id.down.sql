-- 000014_drop_holidays_company_id.down.sql
-- Rollback: Re-add company_id column, foreign key constraint, and index to holidays table

ALTER TABLE holidays ADD COLUMN IF NOT EXISTS company_id BIGINT;
ALTER TABLE holidays ADD CONSTRAINT holidays_company_id_fkey FOREIGN KEY (company_id) REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_holidays_company_id ON holidays(company_id);
