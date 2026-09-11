-- 000005_normalize_overtime_approvers.down.sql
-- Revert normalization of overtime approvers

-- 1. Re-add string columns
ALTER TABLE overtime_entries
ADD COLUMN IF NOT EXISTS team_leader VARCHAR(128),
ADD COLUMN IF NOT EXISTS department_head VARCHAR(128);

-- 2. Restore string values from referenced users
UPDATE overtime_entries oe
SET team_leader = u.name
FROM users u
WHERE oe.team_leader_id = u.id;

UPDATE overtime_entries oe
SET department_head = u.name
FROM users u
WHERE oe.department_head_id = u.id;

-- 3. Drop foreign keys and indexes
ALTER TABLE overtime_entries DROP CONSTRAINT IF EXISTS fk_overtime_entries_team_leader;
ALTER TABLE overtime_entries DROP CONSTRAINT IF EXISTS fk_overtime_entries_department_head;

DROP INDEX IF EXISTS idx_overtimes_team_leader_id;
DROP INDEX IF EXISTS idx_overtimes_department_head_id;

-- 4. Drop foreign key columns
ALTER TABLE overtime_entries
DROP COLUMN IF EXISTS team_leader_id,
DROP COLUMN IF EXISTS department_head_id;
