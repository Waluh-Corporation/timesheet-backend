-- 000013_add_holiday_categories.down.sql
DROP INDEX IF EXISTS idx_holidays_joint_leave;
ALTER TABLE holidays DROP COLUMN IF EXISTS is_religious;
ALTER TABLE holidays DROP COLUMN IF EXISTS is_civic;
