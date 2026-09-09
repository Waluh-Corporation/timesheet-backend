-- 000008_create_approvers_table.down.sql
-- Revert approvers table and re-point overtime_entries foreign keys back to users(id)

-- 1. Remap foreign key values in overtime_entries back to user_id
UPDATE overtime_entries oe
SET team_leader_id = a.user_id
FROM approvers a
WHERE oe.team_leader_id = a.id;

UPDATE overtime_entries oe
SET department_head_id = a.user_id
FROM approvers a
WHERE oe.department_head_id = a.id;

-- 2. Restore Foreign Key constraints pointing to users(id)
ALTER TABLE overtime_entries DROP CONSTRAINT IF EXISTS fk_overtime_entries_team_leader;
ALTER TABLE overtime_entries DROP CONSTRAINT IF EXISTS fk_overtime_entries_department_head;

ALTER TABLE overtime_entries
ADD CONSTRAINT fk_overtime_entries_team_leader
FOREIGN KEY (team_leader_id) REFERENCES users(id)
ON UPDATE CASCADE ON DELETE SET NULL;

ALTER TABLE overtime_entries
ADD CONSTRAINT fk_overtime_entries_department_head
FOREIGN KEY (department_head_id) REFERENCES users(id)
ON UPDATE CASCADE ON DELETE SET NULL;

-- 3. Drop approvers table
DROP TABLE IF EXISTS approvers CASCADE;
