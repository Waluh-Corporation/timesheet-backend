-- 000027_harden_user_registrations_relations_and_indexes.down.sql
-- Revert hardening on user_registrations.

DROP INDEX IF EXISTS idx_user_registrations_pending_user;
DROP INDEX IF EXISTS idx_user_registrations_status_created_at;
DROP INDEX IF EXISTS idx_user_registrations_division_id;
DROP INDEX IF EXISTS idx_user_registrations_department_id;
DROP INDEX IF EXISTS idx_user_registrations_site_id;
DROP INDEX IF EXISTS idx_user_registrations_company_id;
