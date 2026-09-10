CREATE TABLE IF NOT EXISTS system_settings (
    key VARCHAR(64) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Default is_new flag to 'Y' on fresh systems, or 'N' if administrators already exist
INSERT INTO system_settings (key, value, updated_at)
VALUES (
    'is_new',
    CASE 
        WHEN EXISTS (SELECT 1 FROM users WHERE role = 'admin' AND deleted_at IS NULL) THEN 'N'
        ELSE 'Y'
    END,
    NOW()
)
ON CONFLICT (key) DO NOTHING;
