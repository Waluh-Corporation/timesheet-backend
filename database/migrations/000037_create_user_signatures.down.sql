-- 000037_create_user_signatures.down.sql
-- Rollback user_signatures table and indexes.

DROP INDEX IF EXISTS uq_user_signatures_active_per_user;
DROP INDEX IF EXISTS idx_user_signatures_slug;
DROP INDEX IF EXISTS idx_user_signatures_deleted_at;
DROP INDEX IF EXISTS idx_user_signatures_user_id;
DROP TABLE IF EXISTS user_signatures;
