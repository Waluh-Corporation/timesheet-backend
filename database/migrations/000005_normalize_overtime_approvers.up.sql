-- 000005_normalize_overtime_approvers.up.sql
-- Normalization: Convert free-text team_leader and department_head to relational foreign keys to users(id)

-- 1. Add foreign key columns if they do not exist
ALTER TABLE overtime_entries
ADD COLUMN IF NOT EXISTS team_leader_id BIGINT,
ADD COLUMN IF NOT EXISTS department_head_id BIGINT;

-- 2. Data migration / backfill: Match existing string values with users table if possible
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'overtime_entries' AND column_name = 'team_leader'
    ) THEN
        UPDATE overtime_entries oe
        SET team_leader_id = u.id
        FROM users u
        WHERE oe.team_leader_id IS NULL
          AND oe.team_leader IS NOT NULL
          AND TRIM(oe.team_leader) != ''
          AND (LOWER(TRIM(oe.team_leader)) = LOWER(TRIM(u.name)) OR LOWER(TRIM(oe.team_leader)) = LOWER(TRIM(u.username)));
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'overtime_entries' AND column_name = 'department_head'
    ) THEN
        UPDATE overtime_entries oe
        SET department_head_id = u.id
        FROM users u
        WHERE oe.department_head_id IS NULL
          AND oe.department_head IS NOT NULL
          AND TRIM(oe.department_head) != ''
          AND (LOWER(TRIM(oe.department_head)) = LOWER(TRIM(u.name)) OR LOWER(TRIM(oe.department_head)) = LOWER(TRIM(u.username)));
    END IF;
END $$;

-- 3. Add Foreign Key constraints with ON UPDATE CASCADE ON DELETE SET NULL
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_overtime_entries_team_leader'
    ) THEN
        ALTER TABLE overtime_entries
        ADD CONSTRAINT fk_overtime_entries_team_leader
        FOREIGN KEY (team_leader_id) REFERENCES users(id)
        ON UPDATE CASCADE ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_overtime_entries_department_head'
    ) THEN
        ALTER TABLE overtime_entries
        ADD CONSTRAINT fk_overtime_entries_department_head
        FOREIGN KEY (department_head_id) REFERENCES users(id)
        ON UPDATE CASCADE ON DELETE SET NULL;
    END IF;
END $$;

-- 4. Create B-Tree indexes for foreign key joins to ensure sub-second query performance
CREATE INDEX IF NOT EXISTS idx_overtimes_team_leader_id ON overtime_entries(team_leader_id);
CREATE INDEX IF NOT EXISTS idx_overtimes_department_head_id ON overtime_entries(department_head_id);

-- 5. Drop deprecated string columns if they exist
ALTER TABLE overtime_entries
DROP COLUMN IF EXISTS team_leader,
DROP COLUMN IF EXISTS department_head;
