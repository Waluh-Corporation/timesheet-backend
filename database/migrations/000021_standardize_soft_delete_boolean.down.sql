-- 000021_standardize_soft_delete_boolean.down.sql
-- Rollback standarisasi soft delete: kembalikan kolom is_delete jika diperlukan

DROP INDEX IF EXISTS idx_projects_active_lookup;
DROP INDEX IF EXISTS idx_projects_code_active;
DROP INDEX IF EXISTS idx_approvers_active_lookup;

DO $$
DECLARE
    tbl text;
    tables text[] := ARRAY['departments', 'companies', 'projects', 'approvers', 'users', 'daily_activities', 'overtime_entries'];
BEGIN
    FOREACH tbl IN ARRAY tables
    LOOP
        -- Tambahkan kembali is_delete jika belum ada
        EXECUTE format('ALTER TABLE %I ADD COLUMN IF NOT EXISTS is_delete VARCHAR(1) NOT NULL DEFAULT ''N'';', tbl);

        -- Konversi is_active ke is_delete: false -> 'Y', true -> 'N'
        EXECUTE format('
            UPDATE %I 
            SET is_delete = CASE 
                WHEN is_active = false THEN ''Y'' 
                ELSE ''N'' 
            END;
        ', tbl);

        -- Buat index legacy
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I(is_delete);', 'idx_' || tbl || '_is_delete', tbl);
    END LOOP;
END $$;
