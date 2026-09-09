-- 000007_normalize_password_reset_tokens.down.sql
-- Revert normalization on password_reset_tokens table

-- 1. Re-add boolean used column
ALTER TABLE password_reset_tokens
ADD COLUMN IF NOT EXISTS used BOOLEAN NOT NULL DEFAULT false;

-- 2. Restore boolean values from used_at
UPDATE password_reset_tokens
SET used = (used_at IS NOT NULL);

-- 3. Drop indexes
DROP INDEX IF EXISTS idx_password_reset_tokens_active;
DROP INDEX IF EXISTS idx_password_reset_tokens_expires_at;

-- 4. Drop added columns
ALTER TABLE password_reset_tokens
DROP COLUMN IF EXISTS token_type,
DROP COLUMN IF EXISTS used_at,
DROP COLUMN IF EXISTS created_ip,
DROP COLUMN IF EXISTS used_ip;
