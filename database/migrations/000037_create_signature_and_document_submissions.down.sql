-- 000037_create_signature_and_document_submissions.down.sql
-- Revert signature file key and submission tables

DROP TABLE IF EXISTS berita_acara_submissions CASCADE;
DROP TABLE IF EXISTS timesheet_submissions CASCADE;

ALTER TABLE users
    DROP COLUMN IF EXISTS signature_file_key;
