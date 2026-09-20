-- 000028_relate_authenticator_aaguids.down.sql
-- Revert foreign key relationship between web_authn_credentials and authenticator_aaguids

DROP INDEX IF EXISTS idx_webauthn_credentials_authenticator_aaguid;

ALTER TABLE web_authn_credentials 
DROP CONSTRAINT IF EXISTS fk_webauthn_credentials_authenticator_aaguid;

ALTER TABLE web_authn_credentials 
DROP COLUMN IF EXISTS authenticator_aaguid;
