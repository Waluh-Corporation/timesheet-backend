-- 000026_security_hardening_and_db_optimizations.up.sql
-- Hardening Keamanan & Identitas, Covering Index, Functional Indexes, pg_trgm Trigrams, and Partial Index Alignment

-- 1. Create refresh_tokens table for Dual-Token Architecture and Token Rotation
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) UNIQUE NOT NULL,
    family_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_ip VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens(family_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

-- 2. Covering index for monthly activity range queries (Index-Only Scan)
CREATE INDEX IF NOT EXISTS idx_daily_activities_range_covering 
ON daily_activities (user_id, date) 
INCLUDE (status, project_ref_id, start_time, end_time) 
WHERE is_active = true;

-- 3. Functional indexes on code columns to eliminate full table scans
CREATE INDEX IF NOT EXISTS idx_companies_lower_code ON companies (LOWER(code));
CREATE INDEX IF NOT EXISTS idx_departments_lower_code ON departments (LOWER(code));
CREATE INDEX IF NOT EXISTS idx_projects_lower_code ON projects (LOWER(code));

-- 4. Enable pg_trgm extension and GIN trigram indexes for master entity name searches
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_companies_name_trgm ON companies USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_departments_name_trgm ON departments USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_projects_name_trgm ON projects USING gin (name gin_trgm_ops);

-- 5. Drop standalone unselective boolean indexes on is_active in favor of composite/partial indexes
DROP INDEX IF EXISTS idx_companies_is_active;
DROP INDEX IF EXISTS idx_departments_is_active;
DROP INDEX IF EXISTS idx_sites_active;
DROP INDEX IF EXISTS idx_divisions_active;
DROP INDEX IF EXISTS idx_daily_activities_is_active;
DROP INDEX IF EXISTS idx_overtime_entries_active;
DROP INDEX IF EXISTS idx_approvers_is_active;
DROP INDEX IF EXISTS idx_projects_is_active;
