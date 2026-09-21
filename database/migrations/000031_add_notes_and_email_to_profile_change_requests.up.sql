-- 000031_add_notes_and_email_to_profile_change_requests.up.sql
ALTER TABLE profile_change_requests ADD COLUMN IF NOT EXISTS email VARCHAR(255);
ALTER TABLE profile_change_requests ADD COLUMN IF NOT EXISTS notes TEXT;
