-- 000017_drop_app_impacted_from_daily_activities.down.sql
-- Rollback: Re-add column and backfill from canonical projects table.

ALTER TABLE daily_activities ADD COLUMN IF NOT EXISTS app_impacted VARCHAR(255);

UPDATE daily_activities
SET app_impacted = projects.app_impacted
FROM projects
WHERE daily_activities.project_ref_id = projects.id;
