-- 000006_drop_templates_and_cell_mappings.down.sql
-- Recreate templates and cell_mappings tables if rolled back

CREATE TABLE IF NOT EXISTS templates (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(512),
    company VARCHAR(64),
    company_id BIGINT,
    sheet_name VARCHAR(128) NOT NULL DEFAULT 'Sheet1',
    file_data BYTEA,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_by BIGINT REFERENCES users(id) ON UPDATE CASCADE ON DELETE SET NULL,
    builtin VARCHAR(32)
);

CREATE TABLE IF NOT EXISTS cell_mappings (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES templates(id) ON UPDATE CASCADE ON DELETE CASCADE,
    field VARCHAR(32) NOT NULL,
    scope VARCHAR(16) NOT NULL DEFAULT 'cell',
    cell_ref VARCHAR(16),
    column_letter VARCHAR(4),
    start_row INT,
    fillable BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_cell_mappings_template_id ON cell_mappings(template_id);
