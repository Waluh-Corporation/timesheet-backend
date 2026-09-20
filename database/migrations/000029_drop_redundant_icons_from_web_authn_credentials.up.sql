-- 000029_drop_redundant_icons_from_web_authn_credentials.up.sql
-- Remove redundant denormalized icon columns from web_authn_credentials.
-- Icons are normalized in authenticator_aaguids and resolved dynamically via the authenticator_aaguid foreign key.

ALTER TABLE web_authn_credentials DROP COLUMN IF EXISTS icon_light;
ALTER TABLE web_authn_credentials DROP COLUMN IF EXISTS icon_dark;
