-- 000022_harden_password_hash_column_and_argon2id.down.sql

DROP INDEX IF EXISTS idx_users_updated_at;
COMMENT ON COLUMN users.password_hash IS NULL;
