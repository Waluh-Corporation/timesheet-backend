-- 000037_create_signature_and_document_submissions.up.sql
-- Add signature file key to users and create tables for timesheet and berita acara signing submissions

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS signature_file_key VARCHAR(255) NULL;

-- 1. Timesheet Submissions Table
CREATE TABLE IF NOT EXISTS timesheet_submissions (
    id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    month INT NOT NULL,
    year INT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    signed_at TIMESTAMPTZ NULL,
    verification_token VARCHAR(64) NOT NULL UNIQUE,
    document_hash VARCHAR(64) NOT NULL,
    audit_metadata JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_timesheet_submissions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT uq_timesheet_submissions_user_period UNIQUE (user_id, month, year),
    CONSTRAINT chk_timesheet_submissions_status CHECK (status IN ('draft', 'signed', 'submitted', 'revoked'))
);

CREATE INDEX IF NOT EXISTS idx_timesheet_submissions_user_status ON timesheet_submissions(user_id, status);
CREATE INDEX IF NOT EXISTS idx_timesheet_submissions_token ON timesheet_submissions(verification_token);
CREATE INDEX IF NOT EXISTS idx_timesheet_submissions_period ON timesheet_submissions(month, year);

-- 2. Berita Acara Submissions Table
CREATE TABLE IF NOT EXISTS berita_acara_submissions (
    id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    witness_id BIGINT NOT NULL,
    month INT NOT NULL,
    year INT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    employee_signed_at TIMESTAMPTZ NULL,
    witness_signed_at TIMESTAMPTZ NULL,
    witness_rejection_notes TEXT NULL,
    verification_token VARCHAR(64) NOT NULL UNIQUE,
    document_hash VARCHAR(64) NULL,
    audit_metadata JSONB NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_berita_acara_submissions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_berita_acara_submissions_witness FOREIGN KEY (witness_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT uq_berita_acara_submissions_user_period UNIQUE (user_id, month, year),
    CONSTRAINT chk_berita_acara_submissions_status CHECK (status IN ('draft', 'pending_witness', 'signed', 'rejected', 'revoked'))
);

CREATE INDEX IF NOT EXISTS idx_berita_acara_submissions_user_status ON berita_acara_submissions(user_id, status);
CREATE INDEX IF NOT EXISTS idx_berita_acara_submissions_witness_status ON berita_acara_submissions(witness_id, status);
CREATE INDEX IF NOT EXISTS idx_berita_acara_submissions_token ON berita_acara_submissions(verification_token);
CREATE INDEX IF NOT EXISTS idx_berita_acara_submissions_period ON berita_acara_submissions(month, year);
