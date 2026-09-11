-- 000016_enforce_project_relational_integrity.up.sql
-- Pure Relational 3NF: Enforce project_ref_id integrity and synchronize project attributes

-- 1. Backfill daily_activities.project_ref_id from projects.code
UPDATE daily_activities 
SET project_ref_id = projects.id
FROM projects
WHERE daily_activities.project_ref_id IS NULL 
  AND daily_activities.project_id IS NOT NULL 
  AND daily_activities.project_id != ''
  AND projects.code = daily_activities.project_id;

-- 2. Backfill daily_activities.project_ref_id from projects.name if still NULL
UPDATE daily_activities 
SET project_ref_id = projects.id
FROM projects
WHERE daily_activities.project_ref_id IS NULL 
  AND daily_activities.project_name IS NOT NULL 
  AND daily_activities.project_name != ''
  AND LOWER(projects.name) = LOWER(daily_activities.project_name);

-- 3. Synchronize daily_activities project fields with master projects table for pure 3NF consistency
UPDATE daily_activities
SET project_id = projects.code,
    project_name = projects.name,
    app_impacted = projects.app_impacted
FROM projects
WHERE daily_activities.project_ref_id = projects.id;

-- 4. Ensure index on project_ref_id exists for high performance joins and preloads
CREATE INDEX IF NOT EXISTS idx_daily_activities_project_ref_id ON daily_activities(project_ref_id);

-- 5. Ensure canonical foreign key constraint exists
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'daily_activities_project_ref_id_fkey'
    ) THEN
        ALTER TABLE daily_activities 
        ADD CONSTRAINT daily_activities_project_ref_id_fkey 
        FOREIGN KEY (project_ref_id) REFERENCES projects(id) 
        ON UPDATE CASCADE ON DELETE SET NULL;
    END IF;
END $$;
