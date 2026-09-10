-- 000001_create_initial_schema.up.sql
-- Base initial schema for timesheet generator portal

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    username VARCHAR(64) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255),
    role VARCHAR(16) NOT NULL DEFAULT 'user',
    is_active BOOLEAN NOT NULL DEFAULT true,
    name VARCHAR(255),
    mii_id VARCHAR(64),
    employee_id VARCHAR(64),
    division VARCHAR(255),
    department VARCHAR(255),
    group_name VARCHAR(255),
    position VARCHAR(128),
    site VARCHAR(128),
    company VARCHAR(64)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

CREATE TABLE IF NOT EXISTS web_authn_credentials (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    credential_id BYTEA NOT NULL UNIQUE,
    public_key BYTEA NOT NULL,
    attestation_type VARCHAR(64),
    aaguid BYTEA,
    sign_count BIGINT NOT NULL DEFAULT 0,
    clone_warning BOOLEAN NOT NULL DEFAULT false,
    backup_eligible BOOLEAN NOT NULL DEFAULT false,
    backup_state BOOLEAN NOT NULL DEFAULT false,
    transports JSONB,
    friendly_name VARCHAR(128)
);

CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_user_id ON web_authn_credentials(user_id);

CREATE TABLE IF NOT EXISTS templates (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(512),
    company VARCHAR(64),
    sheet_name VARCHAR(128) NOT NULL DEFAULT 'Sheet1',
    file_data BYTEA,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_by BIGINT,
    builtin VARCHAR(32)
);

CREATE INDEX IF NOT EXISTS idx_templates_deleted_at ON templates(deleted_at);

CREATE TABLE IF NOT EXISTS cell_mappings (
    id BIGSERIAL PRIMARY KEY,
    template_id BIGINT NOT NULL REFERENCES templates(id) ON UPDATE CASCADE ON DELETE CASCADE,
    field VARCHAR(32) NOT NULL,
    scope VARCHAR(16) NOT NULL DEFAULT 'cell',
    cell_ref VARCHAR(16),
    "column" VARCHAR(4),
    start_row INT NOT NULL DEFAULT 0,
    fillable BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_cell_mappings_template_id ON cell_mappings(template_id);

CREATE TABLE IF NOT EXISTS daily_activities (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    date DATE NOT NULL,
    start_time VARCHAR(8),
    end_time VARCHAR(8),
    status VARCHAR(8),
    activity TEXT,
    project_name VARCHAR(255),
    project_id VARCHAR(64),
    app_impacted VARCHAR(255)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_date ON daily_activities(user_id, date);

CREATE TABLE IF NOT EXISTS overtime_entries (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    daily_activity_id BIGINT REFERENCES daily_activities(id) ON UPDATE CASCADE ON DELETE SET NULL,
    date DATE NOT NULL,
    start_time VARCHAR(8),
    end_time VARCHAR(8),
    task_description TEXT NOT NULL,
    team_leader VARCHAR(128),
    department_head VARCHAR(128)
);

CREATE INDEX IF NOT EXISTS idx_overtimes_user_id ON overtime_entries(user_id);
CREATE INDEX IF NOT EXISTS idx_overtimes_daily_activity_id ON overtime_entries(daily_activity_id);

CREATE TABLE IF NOT EXISTS push_subscriptions (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    endpoint VARCHAR(512) NOT NULL UNIQUE,
    p256dh VARCHAR(255) NOT NULL,
    auth VARCHAR(255) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user_id ON push_subscriptions(user_id);

CREATE TABLE IF NOT EXISTS profile_change_requests (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    name VARCHAR(255),
    mii_id VARCHAR(64),
    employee_id VARCHAR(64),
    division VARCHAR(255),
    department VARCHAR(255),
    group_name VARCHAR(255),
    position VARCHAR(128),
    site VARCHAR(128),
    reviewed_by BIGINT,
    reviewed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_profile_change_requests_user_id ON profile_change_requests(user_id);

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
