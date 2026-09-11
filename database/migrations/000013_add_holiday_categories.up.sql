-- 000013_add_holiday_categories.up.sql
-- Add civic and religious categorization flags and indexing to holidays table
ALTER TABLE holidays ADD COLUMN IF NOT EXISTS is_civic BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE holidays ADD COLUMN IF NOT EXISTS is_religious BOOLEAN NOT NULL DEFAULT false;

-- Create index for faster holiday lookup and joint leave filtering
CREATE INDEX IF NOT EXISTS idx_holidays_joint_leave ON holidays(is_joint_leave);
