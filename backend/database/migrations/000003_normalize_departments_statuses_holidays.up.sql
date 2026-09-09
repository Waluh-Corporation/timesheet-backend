-- 000003_normalize_departments_statuses_holidays.up.sql
-- Normalization phase 2: departments, activity_statuses, holidays, and FK constraints

-- 1. Departments table
CREATE TABLE IF NOT EXISTS departments (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    company_id BIGINT REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL,
    code VARCHAR(64),
    name VARCHAR(255) NOT NULL,
    division VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_departments_company_id ON departments(company_id);
CREATE INDEX IF NOT EXISTS idx_departments_code ON departments(code);

-- 2. Activity Statuses lookup table
CREATE TABLE IF NOT EXISTS activity_statuses (
    code VARCHAR(8) PRIMARY KEY,
    name VARCHAR(64) NOT NULL,
    description VARCHAR(255),
    is_working_day BOOLEAN NOT NULL DEFAULT true,
    sort_order INT NOT NULL DEFAULT 0
);

-- Seed standard activity statuses
INSERT INTO activity_statuses (code, name, description, is_working_day, sort_order)
VALUES 
    ('P', 'Present', 'Hadir bekerja normal', true, 1),
    ('BT', 'Business Trip', 'Perjalanan dinas', true, 2),
    ('S', 'Sick', 'Sakit', false, 3),
    ('PM', 'Permission', 'Izin', false, 4),
    ('V', 'Leave', 'Cuti / Vacation', false, 5),
    ('X', 'Off', 'Libur / Off', false, 6)
ON CONFLICT (code) DO UPDATE 
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    is_working_day = EXCLUDED.is_working_day,
    sort_order = EXCLUDED.sort_order;

-- 3. Holidays relational table
CREATE TABLE IF NOT EXISTS holidays (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    date DATE NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL,
    company_id BIGINT REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL,
    is_joint_leave BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_holidays_date ON holidays(date);
CREATE INDEX IF NOT EXISTS idx_holidays_company_id ON holidays(company_id);

-- 4. Alter users to add department_id
ALTER TABLE users ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id) ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_users_department_id ON users(department_id);

-- 5. Alter profile_change_requests to add company_id and department_id
ALTER TABLE profile_change_requests ADD COLUMN IF NOT EXISTS company_id BIGINT REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL;
ALTER TABLE profile_change_requests ADD COLUMN IF NOT EXISTS department_id BIGINT REFERENCES departments(id) ON UPDATE CASCADE ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_profile_change_requests_company_id ON profile_change_requests(company_id);
CREATE INDEX IF NOT EXISTS idx_profile_change_requests_department_id ON profile_change_requests(department_id);

-- 6. Add FK constraints on templates.created_by and profile_change_requests.reviewed_by if not already present
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_templates_creator'
    ) THEN
        ALTER TABLE templates 
        ADD CONSTRAINT fk_templates_creator 
        FOREIGN KEY (created_by) REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_profile_change_requests_reviewer'
    ) THEN
        ALTER TABLE profile_change_requests 
        ADD CONSTRAINT fk_profile_change_requests_reviewer 
        FOREIGN KEY (reviewed_by) REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL;
    END IF;
END $$;
