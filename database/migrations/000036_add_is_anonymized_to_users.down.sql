-- 000036_add_is_anonymized_to_users.down.sql
-- Revert is_anonymized column from users table

DROP INDEX IF EXISTS idx_users_is_anonymized;

ALTER TABLE users
    DROP COLUMN IF EXISTS is_anonymized;
