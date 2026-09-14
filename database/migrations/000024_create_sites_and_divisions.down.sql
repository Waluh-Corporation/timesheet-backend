-- 000024_create_sites_and_divisions.down.sql

ALTER TABLE profile_change_requests DROP COLUMN IF EXISTS division_id;
ALTER TABLE profile_change_requests DROP COLUMN IF EXISTS site_id;

ALTER TABLE users DROP COLUMN IF EXISTS division_id;
ALTER TABLE users DROP COLUMN IF EXISTS site_id;

ALTER TABLE departments DROP COLUMN IF EXISTS division_id;

DROP TABLE IF EXISTS divisions CASCADE;
DROP TABLE IF EXISTS sites CASCADE;
