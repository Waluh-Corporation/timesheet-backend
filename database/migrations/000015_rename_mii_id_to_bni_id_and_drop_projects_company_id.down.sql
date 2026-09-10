-- 000015_rename_mii_id_to_bni_id_and_drop_projects_company_id.down.sql
-- 1. Re-add company_id to projects table
ALTER TABLE projects ADD COLUMN IF NOT EXISTS company_id BIGINT REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_projects_company_id ON projects(company_id);

-- 2. Rename bni_id back to mii_id on users table
COMMENT ON COLUMN users.bni_id IS NULL;
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'users' AND column_name = 'bni_id'
    ) THEN
        ALTER TABLE users RENAME COLUMN bni_id TO mii_id;
    END IF;
END $$;

-- 3. Rename bni_id back to mii_id on profile_change_requests table
COMMENT ON COLUMN profile_change_requests.bni_id IS NULL;
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'profile_change_requests' AND column_name = 'bni_id'
    ) THEN
        ALTER TABLE profile_change_requests RENAME COLUMN bni_id TO mii_id;
    END IF;
END $$;
