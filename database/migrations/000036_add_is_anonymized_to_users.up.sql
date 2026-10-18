-- 000036_add_is_anonymized_to_users.up.sql
-- Add is_anonymized boolean column and index to users table

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_anonymized BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_users_is_anonymized
    ON users(is_anonymized);

-- Backfill any previously anonymized accounts
UPDATE users
SET is_anonymized = true
WHERE username LIKE 'deleted_user_%' OR name LIKE 'ANONYMIZED_USER_%';
