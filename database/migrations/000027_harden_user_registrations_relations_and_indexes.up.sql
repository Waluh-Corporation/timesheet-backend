-- 000027_harden_user_registrations_relations_and_indexes.up.sql
-- Enforce explicit foreign key constraints and add missing relational indexes for user_registrations.

-- 1. Standardize foreign key constraints with explicit names
ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS user_registrations_user_id_fkey;
ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS fk_user_registrations_user_id;
ALTER TABLE user_registrations ADD CONSTRAINT fk_user_registrations_user_id 
    FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE;

ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS user_registrations_division_id_fkey;
ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS fk_user_registrations_division_id;
ALTER TABLE user_registrations ADD CONSTRAINT fk_user_registrations_division_id 
    FOREIGN KEY (division_id) REFERENCES divisions(id) ON UPDATE CASCADE ON DELETE SET NULL;

ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS user_registrations_department_id_fkey;
ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS fk_user_registrations_department_id;
ALTER TABLE user_registrations ADD CONSTRAINT fk_user_registrations_department_id 
    FOREIGN KEY (department_id) REFERENCES departments(id) ON UPDATE CASCADE ON DELETE SET NULL;

ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS user_registrations_site_id_fkey;
ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS fk_user_registrations_site_id;
ALTER TABLE user_registrations ADD CONSTRAINT fk_user_registrations_site_id 
    FOREIGN KEY (site_id) REFERENCES sites(id) ON UPDATE CASCADE ON DELETE SET NULL;

ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS user_registrations_company_id_fkey;
ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS fk_user_registrations_company_id;
ALTER TABLE user_registrations ADD CONSTRAINT fk_user_registrations_company_id 
    FOREIGN KEY (company_id) REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL;

ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS user_registrations_reviewed_by_fkey;
ALTER TABLE user_registrations DROP CONSTRAINT IF EXISTS fk_user_registrations_reviewed_by;
ALTER TABLE user_registrations ADD CONSTRAINT fk_user_registrations_reviewed_by 
    FOREIGN KEY (reviewed_by) REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL;

-- 2. Create indexes on all relational foreign key columns
CREATE INDEX IF NOT EXISTS idx_user_registrations_user_id ON user_registrations(user_id);
CREATE INDEX IF NOT EXISTS idx_user_registrations_division_id ON user_registrations(division_id);
CREATE INDEX IF NOT EXISTS idx_user_registrations_department_id ON user_registrations(department_id);
CREATE INDEX IF NOT EXISTS idx_user_registrations_site_id ON user_registrations(site_id);
CREATE INDEX IF NOT EXISTS idx_user_registrations_company_id ON user_registrations(company_id);
CREATE INDEX IF NOT EXISTS idx_user_registrations_reviewed_by ON user_registrations(reviewed_by);

-- 3. Filtered and composite indexes for administrative review
CREATE INDEX IF NOT EXISTS idx_user_registrations_status ON user_registrations(status);
CREATE INDEX IF NOT EXISTS idx_user_registrations_status_created_at ON user_registrations(status, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_registrations_pending_user ON user_registrations(user_id) WHERE status = 'pending';
