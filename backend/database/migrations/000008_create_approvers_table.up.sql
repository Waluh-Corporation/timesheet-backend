-- 000008_create_approvers_table.up.sql
-- Create dedicated approvers table to decouple Team Leaders and Department Heads from users table

-- 1. Create approvers table
CREATE TABLE IF NOT EXISTS approvers (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    company_id BIGINT REFERENCES companies(id) ON UPDATE CASCADE ON DELETE SET NULL,
    department_id BIGINT REFERENCES departments(id) ON UPDATE CASCADE ON DELETE SET NULL,
    user_id BIGINT REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    employee_id VARCHAR(64),
    role_type VARCHAR(32) NOT NULL, -- 'team_leader', 'department_head'
    title VARCHAR(128),
    email VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT true
);

-- 2. Indexes for performance
CREATE INDEX IF NOT EXISTS idx_approvers_company_role ON approvers(company_id, role_type) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_approvers_department_id ON approvers(department_id);
CREATE INDEX IF NOT EXISTS idx_approvers_user_id ON approvers(user_id);

-- 3. Backfill any existing approvers from users referenced in overtime_entries
DO $$
BEGIN
    -- Backfill team leaders
    INSERT INTO approvers (user_id, company_id, department_id, name, employee_id, role_type, email, is_active)
    SELECT DISTINCT u.id, u.company_id, u.department_id, COALESCE(NULLIF(u.name, ''), u.username), u.employee_id, 'team_leader', u.email, true
    FROM overtime_entries oe
    JOIN users u ON oe.team_leader_id = u.id
    WHERE oe.team_leader_id IS NOT NULL;

    -- Backfill department heads
    INSERT INTO approvers (user_id, company_id, department_id, name, employee_id, role_type, email, is_active)
    SELECT DISTINCT u.id, u.company_id, u.department_id, COALESCE(NULLIF(u.name, ''), u.username), u.employee_id, 'department_head', u.email, true
    FROM overtime_entries oe
    JOIN users u ON oe.department_head_id = u.id
    WHERE oe.department_head_id IS NOT NULL;

    -- Remap foreign key values in overtime_entries from user.id to approver.id
    UPDATE overtime_entries oe
    SET team_leader_id = a.id
    FROM approvers a
    WHERE a.user_id = oe.team_leader_id AND a.role_type = 'team_leader';

    UPDATE overtime_entries oe
    SET department_head_id = a.id
    FROM approvers a
    WHERE a.user_id = oe.department_head_id AND a.role_type = 'department_head';
END $$;

-- 4. Re-point Foreign Key constraints to approvers(id)
ALTER TABLE overtime_entries DROP CONSTRAINT IF EXISTS fk_overtime_entries_team_leader;
ALTER TABLE overtime_entries DROP CONSTRAINT IF EXISTS fk_overtime_entries_department_head;

ALTER TABLE overtime_entries
ADD CONSTRAINT fk_overtime_entries_team_leader
FOREIGN KEY (team_leader_id) REFERENCES approvers(id)
ON UPDATE CASCADE ON DELETE SET NULL;

ALTER TABLE overtime_entries
ADD CONSTRAINT fk_overtime_entries_department_head
FOREIGN KEY (department_head_id) REFERENCES approvers(id)
ON UPDATE CASCADE ON DELETE SET NULL;
