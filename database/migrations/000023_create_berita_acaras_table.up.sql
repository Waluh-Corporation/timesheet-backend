-- 000023_create_berita_acaras_table.up.sql
-- Create berita_acaras table with referential integrity, indexes, and active-only partial unique index

CREATE TABLE IF NOT EXISTS berita_acaras (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    daily_activity_id BIGINT REFERENCES daily_activities(id) ON UPDATE CASCADE ON DELETE SET NULL,
    date DATE NOT NULL,
    day VARCHAR(32),
    start_time VARCHAR(8),
    end_time VARCHAR(8),
    keterangan TEXT NOT NULL DEFAULT '',
    team_leader_id BIGINT REFERENCES approvers(id) ON UPDATE CASCADE ON DELETE SET NULL,
    department_head_id BIGINT REFERENCES approvers(id) ON UPDATE CASCADE ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_berita_acaras_user_id ON berita_acaras(user_id);
CREATE INDEX IF NOT EXISTS idx_berita_acaras_date ON berita_acaras(date);
CREATE INDEX IF NOT EXISTS idx_berita_acaras_user_date ON berita_acaras(user_id, date);
CREATE INDEX IF NOT EXISTS idx_berita_acaras_daily_activity_id ON berita_acaras(daily_activity_id);
CREATE INDEX IF NOT EXISTS idx_berita_acaras_team_leader_id ON berita_acaras(team_leader_id);
CREATE INDEX IF NOT EXISTS idx_berita_acaras_department_head_id ON berita_acaras(department_head_id);
CREATE INDEX IF NOT EXISTS idx_berita_acaras_is_active ON berita_acaras(is_active);
CREATE UNIQUE INDEX IF NOT EXISTS idx_berita_acaras_user_date_active ON berita_acaras(user_id, date) WHERE is_active = true;

ANALYZE berita_acaras;
