-- 000007_normalize_password_reset_tokens.up.sql
-- Normalize password_reset_tokens: add token_type, temporal auditability (used_at), IP audit, and performance indexes

-- 1. Add audit and classification columns
ALTER TABLE password_reset_tokens
ADD COLUMN IF NOT EXISTS token_type VARCHAR(32) NOT NULL DEFAULT 'password_reset',
ADD COLUMN IF NOT EXISTS used_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS created_ip VARCHAR(45),
ADD COLUMN IF NOT EXISTS used_ip VARCHAR(45);

-- 2. Backfill existing used boolean data into used_at timestamp
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'password_reset_tokens' AND column_name = 'used'
    ) THEN
        UPDATE password_reset_tokens
        SET used_at = created_at
        WHERE used = true AND used_at IS NULL;

        ALTER TABLE password_reset_tokens DROP COLUMN used;
    END IF;
END $$;

-- 3. High-performance partial index for active (unconsumed) tokens
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_active 
ON password_reset_tokens(user_id) 
WHERE used_at IS NULL;

-- 4. B-Tree index on expires_at for efficient TTL housekeeping / pruning
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at 
ON password_reset_tokens(expires_at);
