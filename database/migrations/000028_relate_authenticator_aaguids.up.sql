-- 000028_relate_authenticator_aaguids.up.sql
-- Establish foreign key relationship between web_authn_credentials and authenticator_aaguids

ALTER TABLE web_authn_credentials 
ADD COLUMN IF NOT EXISTS authenticator_aaguid VARCHAR(36);

-- Backfill existing credentials if aaguid (bytea) is valid and matches an entry in authenticator_aaguids
UPDATE web_authn_credentials
SET authenticator_aaguid = LOWER(
    SUBSTRING(ENCODE(aaguid, 'hex') FROM 1 FOR 8) || '-' ||
    SUBSTRING(ENCODE(aaguid, 'hex') FROM 9 FOR 4) || '-' ||
    SUBSTRING(ENCODE(aaguid, 'hex') FROM 13 FOR 4) || '-' ||
    SUBSTRING(ENCODE(aaguid, 'hex') FROM 17 FOR 4) || '-' ||
    SUBSTRING(ENCODE(aaguid, 'hex') FROM 21 FOR 12)
)
WHERE aaguid IS NOT NULL 
  AND LENGTH(aaguid) = 16 
  AND aaguid <> '\x00000000000000000000000000000000'::bytea
  AND EXISTS (
      SELECT 1 FROM authenticator_aaguids aa 
      WHERE aa.aaguid = LOWER(
          SUBSTRING(ENCODE(web_authn_credentials.aaguid, 'hex') FROM 1 FOR 8) || '-' ||
          SUBSTRING(ENCODE(web_authn_credentials.aaguid, 'hex') FROM 9 FOR 4) || '-' ||
          SUBSTRING(ENCODE(web_authn_credentials.aaguid, 'hex') FROM 13 FOR 4) || '-' ||
          SUBSTRING(ENCODE(web_authn_credentials.aaguid, 'hex') FROM 17 FOR 4) || '-' ||
          SUBSTRING(ENCODE(web_authn_credentials.aaguid, 'hex') FROM 21 FOR 12)
      )
  );

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_webauthn_credentials_authenticator_aaguid'
    ) THEN
        ALTER TABLE web_authn_credentials
        ADD CONSTRAINT fk_webauthn_credentials_authenticator_aaguid
        FOREIGN KEY (authenticator_aaguid)
        REFERENCES authenticator_aaguids(aaguid)
        ON UPDATE CASCADE
        ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_webauthn_credentials_authenticator_aaguid 
ON web_authn_credentials(authenticator_aaguid);
