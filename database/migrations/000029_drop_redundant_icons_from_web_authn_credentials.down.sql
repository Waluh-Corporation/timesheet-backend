-- 000029_drop_redundant_icons_from_web_authn_credentials.down.sql
-- Re-add denormalized icon columns and backfill from authenticator_aaguids.

ALTER TABLE web_authn_credentials ADD COLUMN IF NOT EXISTS icon_light TEXT;
ALTER TABLE web_authn_credentials ADD COLUMN IF NOT EXISTS icon_dark TEXT;

UPDATE web_authn_credentials c
SET icon_light = a.icon_light,
    icon_dark = a.icon_dark
FROM authenticator_aaguids a
WHERE c.authenticator_aaguid = a.aaguid;
