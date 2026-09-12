-- 000017_drop_app_impacted_from_daily_activities.up.sql
-- Pure Relational 3NF: Drop redundant denormalized app_impacted column from daily_activities.
-- The canonical source of truth is projects.app_impacted, resolved via project_ref_id foreign key.

ALTER TABLE daily_activities DROP COLUMN IF EXISTS app_impacted;
