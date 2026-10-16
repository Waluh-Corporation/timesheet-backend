-- 000035_add_berita_acara_to_daily_activities.down.sql
-- Revert Berita Acara support from daily activities

DROP INDEX IF EXISTS idx_daily_activities_berita_acara;

ALTER TABLE daily_activities
    DROP CONSTRAINT IF EXISTS fk_daily_activities_witness;

ALTER TABLE daily_activities
    DROP COLUMN IF EXISTS berita_acara_notes,
    DROP COLUMN IF EXISTS witness_id,
    DROP COLUMN IF EXISTS is_berita_acara;
