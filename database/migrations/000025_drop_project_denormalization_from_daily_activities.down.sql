-- 000025_drop_project_denormalization_from_daily_activities.down.sql
ALTER TABLE daily_activities ADD COLUMN IF NOT EXISTS project_id varchar(64);
ALTER TABLE daily_activities ADD COLUMN IF NOT EXISTS project_name varchar(255);

-- Backfill from referenced project if present
UPDATE daily_activities
SET project_id = projects.code,
    project_name = projects.name
FROM projects
WHERE daily_activities.project_ref_id = projects.id;
