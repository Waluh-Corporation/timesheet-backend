-- 000009_relate_daily_activities_user_id.up.sql
-- Relate user_id in daily_activities table to id in users table with ON UPDATE CASCADE ON DELETE CASCADE

-- 1. Clean up any orphaned daily_activities records with invalid user_id
DELETE FROM daily_activities
WHERE user_id NOT IN (SELECT id FROM users);

-- 2. Add foreign key constraint if it doesn't already exist
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

-- 3. Ensure index on daily_activities(user_id) exists for fast lookups and joins
CREATE INDEX IF NOT EXISTS idx_daily_activities_user_id ON daily_activities(user_id);
