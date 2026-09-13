-- 000022_harden_password_hash_column_and_argon2id.up.sql
-- Standarisasi kolom password_hash untuk format Argon2id PHC string (~97 karakter).
-- Memastikan tipe data VARCHAR(255) memadai dan menambahkan dokumentasi metadata kolom.

DO $$
BEGIN
    -- Pastikan kolom password_hash bertipe VARCHAR(255)
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_schema = 'public' 
          AND table_name = 'users' 
          AND column_name = 'password_hash'
    ) THEN
        ALTER TABLE users ALTER COLUMN password_hash TYPE VARCHAR(255);
    ELSE
        ALTER TABLE users ADD COLUMN password_hash VARCHAR(255);
    END IF;
END $$;

-- Dokumentasikan spesifikasi hashing Argon2id pada kolom
COMMENT ON COLUMN users.password_hash IS 'Argon2id PHC-encoded password hash conforming to OWASP guidelines: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>';

-- Pastikan indeks pendukung users updated_at jika dibutuhkan query audit
CREATE INDEX IF NOT EXISTS idx_users_updated_at ON users(updated_at);

-- Pastikan constraint UNIQUE dan unique index untuk username dan email tersedia
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'uq_users_username'
    ) AND NOT EXISTS (
        SELECT 1 FROM pg_indexes WHERE tablename = 'users' AND indexname = 'idx_users_username'
    ) THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'uq_users_email'
    ) AND NOT EXISTS (
        SELECT 1 FROM pg_indexes WHERE tablename = 'users' AND indexname = 'idx_users_email'
    ) THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
    END IF;
END $$;

ANALYZE users;
