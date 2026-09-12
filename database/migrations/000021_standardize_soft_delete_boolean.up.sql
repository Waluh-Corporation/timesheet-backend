-- 000021_standardize_soft_delete_boolean.up.sql
-- Standarisasi soft delete dengan kolom is_active BOOLEAN NOT NULL DEFAULT true
-- Melakukan migrasi data dari legacy is_delete (jika ada), menghapus kolom is_delete, dan membuat indeks teroptimasi.

-- 1. Helper function / block untuk migrasi is_delete ke is_active pada tabel target
DO $$
DECLARE
    tbl text;
    tables text[] := ARRAY['departments', 'companies', 'projects', 'approvers', 'users', 'daily_activities', 'overtime_entries'];
BEGIN
    FOREACH tbl IN ARRAY tables
    LOOP
        -- Pastikan kolom is_active sudah ada
        EXECUTE format('ALTER TABLE %I ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;', tbl);

        -- Jika kolom legacy is_delete masih ada pada tabel
        IF EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_schema = 'public' 
              AND table_name = tbl 
              AND column_name = 'is_delete'
        ) THEN
            -- Backfill: 'Y', '1', 'TRUE' -> false; 'N', '0', 'FALSE', NULL -> true
            EXECUTE format('
                UPDATE %I 
                SET is_active = CASE 
                    WHEN UPPER(TRIM(is_delete::text)) IN (''Y'', ''YES'', ''1'', ''TRUE'') THEN false 
                    ELSE true 
                END;
            ', tbl);

            -- Hapus index lama yang terkait dengan is_delete jika ada
            EXECUTE format('DROP INDEX IF EXISTS %I;', 'idx_' || tbl || '_is_delete');
            EXECUTE format('DROP INDEX IF EXISTS %I;', 'uq_' || tbl || '_code_is_delete');

            -- Hapus kolom legacy is_delete
            EXECUTE format('ALTER TABLE %I DROP COLUMN IF EXISTS is_delete;', tbl);
        END IF;
    END LOOP;
END $$;

-- 2. Buat atau perbarui indeks performa untuk filter is_active
CREATE INDEX IF NOT EXISTS idx_projects_is_active ON projects(is_active);
CREATE INDEX IF NOT EXISTS idx_projects_active_lookup ON projects(id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_projects_code_active ON projects(code, is_active);

CREATE INDEX IF NOT EXISTS idx_approvers_is_active ON approvers(is_active);
CREATE INDEX IF NOT EXISTS idx_approvers_active_lookup ON approvers(id) WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_departments_is_active ON departments(is_active);
CREATE INDEX IF NOT EXISTS idx_companies_is_active ON companies(is_active);
CREATE INDEX IF NOT EXISTS idx_overtime_entries_active ON overtime_entries(is_active);
CREATE INDEX IF NOT EXISTS idx_daily_activities_is_active ON daily_activities(is_active);

-- Update statistik PostgreSQL query planner
ANALYZE projects;
ANALYZE approvers;
ANALYZE departments;
ANALYZE companies;
ANALYZE overtime_entries;
ANALYZE daily_activities;
ANALYZE users;
