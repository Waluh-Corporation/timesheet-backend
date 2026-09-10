-- 000009_relate_daily_activities_user_id.down.sql
ALTER TABLE daily_activities DROP CONSTRAINT IF EXISTS fk_daily_activities_user;
DROP INDEX IF EXISTS idx_daily_activities_user_id;
