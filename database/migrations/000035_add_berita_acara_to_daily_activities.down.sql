-- 000036_add_berita_acara_to_daily_activities.down.sql
DROP INDEX IF EXISTS idx_daily_activities_witness_id;
DROP INDEX IF EXISTS idx_daily_activities_is_berita_acara;

ALTER TABLE daily_activities DROP COLUMN IF EXISTS witness_id;
ALTER TABLE daily_activities DROP COLUMN IF EXISTS is_berita_acara;
