-- 000008_create_approvers_table.down.sql
-- Revert approvers table

-- 1. Drop Foreign Key constraints pointing to approvers(id)
ALTER TABLE overtime_entries DROP CONSTRAINT IF EXISTS fk_overtime_entries_team_leader;
ALTER TABLE overtime_entries DROP CONSTRAINT IF EXISTS fk_overtime_entries_department_head;

-- 2. Drop approvers table
DROP TABLE IF EXISTS approvers CASCADE;
