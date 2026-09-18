-- 000026_create_user_registrations_table.up.sql
-- Create user_registrations table with complete relational integrity, foreign keys, and indexes.

CREATE TABLE IF NOT EXISTS user_registrations (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id BIGINT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    name VARCHAR(255),
    bni_id VARCHAR(64),
    employee_id VARCHAR(64),
    division VARCHAR(255),
    division_id BIGINT,
    department VARCHAR(255),
    department_id BIGINT,
    site VARCHAR(128),
    site_id BIGINT,
    company VARCHAR(64),
    company_id BIGINT,
    position VARCHAR(128),
    group_name VARCHAR(255),
    admin_notes TEXT,
    reviewed_by BIGINT,
    reviewed_at TIMESTAMPTZ,

    CONSTRAINT chk_user_registrations_status CHECK (status IN ('pending', 'approved', 'rejected')),
    CONSTRAINT fk_user_registrations_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    CONSTRAINT fk_user_registrations_division_id FOREIGN KEY (division_id) REFERENCES divisions(id) ON UPDATE CASCADE ON DELETE SET NULL,
    CONSTRAINT fk_user_registrations_department_id FOREIGN KEY (department_id) REFERENCES departments(id) ON UPDATE CASCADE ON DELETE SET NULL,
    CONSTRAINT fk_user_registrations_site_id FOREIGN KEY (site_id) REFERENCES sites(id) ON UPDATE CASCADE ON DELETE SET NULL,
    CONSTRAINT fk_user_registrations_company_id FOREIGN KEY (company_id) REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL,
    CONSTRAINT fk_user_registrations_reviewed_by FOREIGN KEY (reviewed_by) REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL
);

-- Index foreign keys for optimal join performance and preventing table locks on parent updates/deletions
CREATE INDEX IF NOT EXISTS idx_user_registrations_user_id ON user_registrations(user_id);
CREATE INDEX IF NOT EXISTS idx_user_registrations_division_id ON user_registrations(division_id);
CREATE INDEX IF NOT EXISTS idx_user_registrations_department_id ON user_registrations(department_id);
CREATE INDEX IF NOT EXISTS idx_user_registrations_site_id ON user_registrations(site_id);
CREATE INDEX IF NOT EXISTS idx_user_registrations_company_id ON user_registrations(company_id);
CREATE INDEX IF NOT EXISTS idx_user_registrations_reviewed_by ON user_registrations(reviewed_by);

-- Index for status filtering and ordered pagination
CREATE INDEX IF NOT EXISTS idx_user_registrations_status ON user_registrations(status);
CREATE INDEX IF NOT EXISTS idx_user_registrations_status_created_at ON user_registrations(status, created_at DESC);

-- Partial unique index: ensure a user cannot have more than one pending registration concurrently
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_registrations_pending_user ON user_registrations(user_id) WHERE status = 'pending';
