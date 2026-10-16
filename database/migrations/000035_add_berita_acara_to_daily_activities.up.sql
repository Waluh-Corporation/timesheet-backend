-- 000036_add_berita_acara_to_daily_activities.up.sql
ALTER TABLE daily_activities ADD COLUMN IF NOT EXISTS is_berita_acara BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE daily_activities ADD COLUMN IF NOT EXISTS witness_id BIGINT REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_daily_activities_is_berita_acara ON daily_activities(is_berita_acara);
CREATE INDEX IF NOT EXISTS idx_daily_activities_witness_id ON daily_activities(witness_id);

-- Seed standard BA activity status if not present
INSERT INTO activity_statuses (code, name, description, is_working_day, sort_order)
VALUES ('BA', 'Berita Acara', 'Berita Acara Kehadiran', true, 7)
ON CONFLICT (code) DO NOTHING;
