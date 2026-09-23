-- 000032_create_timesheet_jobs.up.sql
CREATE TABLE IF NOT EXISTS timesheet_jobs (
    id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    month INT NOT NULL,
    year INT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'queued',
    file_key VARCHAR(255),
    download_url TEXT,
    expires_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_timesheet_jobs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_timesheet_jobs_user_status ON timesheet_jobs(user_id, status);
CREATE INDEX IF NOT EXISTS idx_timesheet_jobs_expires_at ON timesheet_jobs(expires_at);
