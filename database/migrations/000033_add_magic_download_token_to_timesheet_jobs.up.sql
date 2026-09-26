-- 000033_add_magic_download_token_to_timesheet_jobs.up.sql
ALTER TABLE timesheet_jobs
    ADD COLUMN IF NOT EXISTS download_token VARCHAR(64) UNIQUE,
    ADD COLUMN IF NOT EXISTS download_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_downloads INT NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS token_expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_timesheet_jobs_download_token ON timesheet_jobs(download_token) WHERE download_token IS NOT NULL;
