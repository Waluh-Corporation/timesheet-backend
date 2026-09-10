-- 000003_normalize_departments_statuses_holidays.down.sql
ALTER TABLE templates DROP CONSTRAINT IF EXISTS fk_templates_creator;
ALTER TABLE profile_change_requests DROP CONSTRAINT IF EXISTS fk_profile_change_requests_reviewer;
ALTER TABLE profile_change_requests DROP COLUMN IF EXISTS department_id;
ALTER TABLE profile_change_requests DROP COLUMN IF EXISTS company_id;
ALTER TABLE users DROP COLUMN IF EXISTS department_id;
DROP TABLE IF EXISTS holidays;
DROP TABLE IF EXISTS activity_statuses;
DROP TABLE IF EXISTS departments;
