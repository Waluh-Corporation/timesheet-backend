-- 000002_normalize_projects_and_companies.down.sql
ALTER TABLE daily_activities DROP COLUMN IF EXISTS project_ref_id;
ALTER TABLE templates DROP COLUMN IF EXISTS company_id;
ALTER TABLE users DROP COLUMN IF EXISTS company_id;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS companies;
