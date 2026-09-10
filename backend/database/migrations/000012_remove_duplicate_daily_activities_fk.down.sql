-- 000012_remove_duplicate_daily_activities_fk.down.sql
-- Re-add legacy constraint if rolling back
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'daily_activities_user_id_fkey'
    ) THEN
        ALTER TABLE daily_activities
        ADD CONSTRAINT daily_activities_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON UPDATE CASCADE ON DELETE CASCADE;
    END IF;
END $$;
