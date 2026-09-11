-- 000015_rename_mii_id_to_bni_id_and_drop_projects_company_id.up.sql
-- 1. Rename mii_id to bni_id and set comment 'NPP BNI' on users table
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'users' AND column_name = 'mii_id'
    ) THEN
        ALTER TABLE users RENAME COLUMN mii_id TO bni_id;
    END IF;
END $$;

COMMENT ON COLUMN users.bni_id IS 'NPP BNI';

-- 2. Rename mii_id to bni_id on profile_change_requests table
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'profile_change_requests' AND column_name = 'mii_id'
    ) THEN
        ALTER TABLE profile_change_requests RENAME COLUMN mii_id TO bni_id;
        COMMENT ON COLUMN profile_change_requests.bni_id IS 'NPP BNI';
    END IF;
END $$;

-- 3. Drop foreign key constraint, index, and company_id column from projects table
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_company_id_fkey;
DROP INDEX IF EXISTS idx_projects_company_id;
ALTER TABLE projects DROP COLUMN IF EXISTS company_id;
