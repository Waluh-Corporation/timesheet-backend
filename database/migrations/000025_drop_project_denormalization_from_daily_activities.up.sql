-- 000025_drop_project_denormalization_from_daily_activities.up.sql
-- Pure Relational 3NF: Drop denormalized project_id and project_name columns from daily_activities.
-- Canonical source of truth is projects table resolved via project_ref_id foreign key.

-- 1. Ensure any remaining records with NULL project_ref_id are backfilled from project_id/project_name if possible
UPDATE daily_activities 
SET project_ref_id = projects.id
FROM projects
WHERE daily_activities.project_ref_id IS NULL 
  AND daily_activities.project_id IS NOT NULL 
  AND daily_activities.project_id != ''
  AND projects.code = daily_activities.project_id;

UPDATE daily_activities 
SET project_ref_id = projects.id
FROM projects
WHERE daily_activities.project_ref_id IS NULL 
  AND daily_activities.project_name IS NOT NULL 
  AND daily_activities.project_name != ''
  AND LOWER(projects.name) = LOWER(daily_activities.project_name);

-- 2. Drop the redundant denormalized columns
ALTER TABLE daily_activities DROP COLUMN IF EXISTS project_id;
ALTER TABLE daily_activities DROP COLUMN IF EXISTS project_name;
