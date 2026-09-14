-- 000024_create_sites_and_divisions.up.sql
-- Create Master Data tables for sites and divisions, add foreign key relationships to departments, users, and profile_change_requests

-- 1. Create sites table
CREATE TABLE IF NOT EXISTS sites (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    code VARCHAR(64) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_sites_active ON sites(is_active);

-- 2. Create divisions table
CREATE TABLE IF NOT EXISTS divisions (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    code VARCHAR(64) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_divisions_active ON divisions(is_active);

-- 3. Seed default sites
INSERT INTO sites (code, name, is_active)
VALUES 
    ('ctcn', 'Citicon', true),
    ('rdtx', 'RDTX', true),
    ('pjp', 'Pejompongan', true)
ON CONFLICT (code) DO NOTHING;

-- 4. Seed default divisions
INSERT INTO divisions (code, name, is_active)
VALUES 
    ('wdd', 'Wholesale Digital Delivery', true)
ON CONFLICT (code) DO NOTHING;

-- Seed any distinct non-empty divisions from departments if not already present
INSERT INTO divisions (code, name, is_active)
SELECT 
    LOWER(SUBSTRING(REGEXP_REPLACE(division, '[^a-zA-Z0-9]', '', 'g'), 1, 32)) AS code,
    division AS name,
    true AS is_active
FROM departments
WHERE division IS NOT NULL AND TRIM(division) != ''
GROUP BY division
ON CONFLICT (code) DO NOTHING;

-- 5. Add relational columns and foreign keys
ALTER TABLE departments ADD COLUMN IF NOT EXISTS division_id INT REFERENCES divisions(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_departments_division_id ON departments(division_id);

ALTER TABLE users ADD COLUMN IF NOT EXISTS site_id INT REFERENCES sites(id) ON DELETE SET NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS division_id INT REFERENCES divisions(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_users_site_id ON users(site_id);
CREATE INDEX IF NOT EXISTS idx_users_division_id ON users(division_id);

ALTER TABLE profile_change_requests ADD COLUMN IF NOT EXISTS site_id INT REFERENCES sites(id) ON DELETE SET NULL;
ALTER TABLE profile_change_requests ADD COLUMN IF NOT EXISTS division_id INT REFERENCES divisions(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_profile_change_requests_site_id ON profile_change_requests(site_id);
CREATE INDEX IF NOT EXISTS idx_profile_change_requests_division_id ON profile_change_requests(division_id);

-- 6. Backfill existing relational IDs
UPDATE departments d 
SET division_id = div.id 
FROM divisions div 
WHERE d.division_id IS NULL AND (LOWER(d.division) = LOWER(div.name) OR LOWER(d.division) = LOWER(div.code));

UPDATE users u 
SET site_id = s.id 
FROM sites s 
WHERE u.site_id IS NULL AND (LOWER(u.site) = LOWER(s.name) OR LOWER(u.site) = LOWER(s.code));

UPDATE users u 
SET division_id = d.id 
FROM divisions d 
WHERE u.division_id IS NULL AND (LOWER(u.division) = LOWER(d.name) OR LOWER(u.division) = LOWER(d.code));

UPDATE profile_change_requests pcr 
SET site_id = s.id 
FROM sites s 
WHERE pcr.site_id IS NULL AND (LOWER(pcr.site) = LOWER(s.name) OR LOWER(pcr.site) = LOWER(s.code));

UPDATE profile_change_requests pcr 
SET division_id = d.id 
FROM divisions d 
WHERE pcr.division_id IS NULL AND (LOWER(pcr.division) = LOWER(d.name) OR LOWER(pcr.division) = LOWER(d.code));

ANALYZE sites;
ANALYZE divisions;
ANALYZE departments;
ANALYZE users;
