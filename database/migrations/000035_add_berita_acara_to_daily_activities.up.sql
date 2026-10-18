-- 000035_add_berita_acara_to_daily_activities.up.sql
-- Add Berita Acara support to daily activities

ALTER TABLE daily_activities
    ADD COLUMN IF NOT EXISTS is_berita_acara BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS witness_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS berita_acara_notes TEXT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_daily_activities_witness'
    ) THEN
        ALTER TABLE daily_activities
            ADD CONSTRAINT fk_daily_activities_witness
            FOREIGN KEY (witness_id) REFERENCES users(id)
            ON UPDATE CASCADE ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_daily_activities_berita_acara
    ON daily_activities(user_id, is_berita_acara)
    WHERE is_berita_acara = true;
