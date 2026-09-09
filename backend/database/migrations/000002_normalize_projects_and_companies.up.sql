-- 000002_normalize_projects_and_companies.up.sql
-- Normalization phase 1: companies and projects tables with foreign key constraints

CREATE TABLE IF NOT EXISTS companies (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    app_impacted VARCHAR(255),
    company_id BIGINT REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_projects_code ON projects(code);
CREATE INDEX IF NOT EXISTS idx_projects_company_id ON projects(company_id);

-- Alter users table to add company_id FK
ALTER TABLE users ADD COLUMN IF NOT EXISTS company_id BIGINT REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_users_company_id ON users(company_id);

-- Alter templates table to add company_id FK
ALTER TABLE templates ADD COLUMN IF NOT EXISTS company_id BIGINT REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_templates_company_id ON templates(company_id);

-- Alter daily_activities table to add project_ref_id FK
ALTER TABLE daily_activities ADD COLUMN IF NOT EXISTS project_ref_id BIGINT REFERENCES projects(id) ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_daily_activities_project_ref_id ON daily_activities(project_ref_id);
