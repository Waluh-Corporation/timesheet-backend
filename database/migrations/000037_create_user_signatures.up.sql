-- 000037_create_user_signatures.up.sql
-- Create 3NF normalized user_signatures table for DocuSeal specimen onboarding and document injection.

CREATE TABLE IF NOT EXISTS user_signatures (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    docuseal_submission_id BIGINT NOT NULL DEFAULT 0,
    docuseal_submitter_id BIGINT NOT NULL UNIQUE,
    docuseal_slug VARCHAR(128) NOT NULL,
    signature_type VARCHAR(16) NOT NULL DEFAULT 'drawn' CHECK (signature_type IN ('drawn', 'typed', 'uploaded')),
    signature_text VARCHAR(255),
    status VARCHAR(32) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'declined', 'revoked')),
    s3_signature_key VARCHAR(512),
    s3_audit_pdf_key VARCHAR(512),
    signer_ip VARCHAR(45),
    signer_user_agent TEXT,
    signed_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_user_signatures_user_id ON user_signatures(user_id);
CREATE INDEX IF NOT EXISTS idx_user_signatures_deleted_at ON user_signatures(deleted_at);
CREATE INDEX IF NOT EXISTS idx_user_signatures_slug ON user_signatures(docuseal_slug);
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_signatures_active_per_user
    ON user_signatures(user_id)
    WHERE is_active = true AND deleted_at IS NULL;
