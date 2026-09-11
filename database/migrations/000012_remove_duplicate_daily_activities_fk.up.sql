-- 000012_remove_duplicate_daily_activities_fk.up.sql
-- Resolve duplicate foreign key on daily_activities(user_id)
-- Drop the default legacy Postgres constraint so only canonical fk_daily_activities_user remains
ALTER TABLE daily_activities DROP CONSTRAINT IF EXISTS daily_activities_user_id_fkey;

-- Ensure canonical foreign key constraint exists on daily_activities(user_id) -> users(id)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_daily_activities_user'
    ) THEN
        ALTER TABLE daily_activities
        ADD CONSTRAINT fk_daily_activities_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON UPDATE CASCADE ON DELETE CASCADE;
    END IF;
END $$;
