-- 000027_create_authenticator_aaguids.down.sql
ALTER TABLE web_authn_credentials DROP COLUMN IF EXISTS icon;
DROP TABLE IF EXISTS authenticator_aaguids CASCADE;
