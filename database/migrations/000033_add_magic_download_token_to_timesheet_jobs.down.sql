-- 000033_add_magic_download_token_to_timesheet_jobs.down.sql
DROP INDEX IF EXISTS idx_timesheet_jobs_download_token;

ALTER TABLE timesheet_jobs
    DROP COLUMN IF EXISTS download_token,
    DROP COLUMN IF EXISTS download_count,
    DROP COLUMN IF EXISTS max_downloads,
    DROP COLUMN IF EXISTS token_expires_at;
