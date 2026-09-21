-- 000031_add_notes_and_email_to_profile_change_requests.down.sql
ALTER TABLE profile_change_requests DROP COLUMN IF EXISTS email;
ALTER TABLE profile_change_requests DROP COLUMN IF EXISTS notes;
