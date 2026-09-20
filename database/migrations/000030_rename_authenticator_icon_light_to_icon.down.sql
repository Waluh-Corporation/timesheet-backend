-- 000030_rename_authenticator_icon_light_to_icon.down.sql
-- Revert single 'icon' column back to 'icon_light' and restore 'icon_dark'

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'authenticator_aaguids' AND column_name = 'icon'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'authenticator_aaguids' AND column_name = 'icon_light'
    ) THEN
        ALTER TABLE authenticator_aaguids RENAME COLUMN icon TO icon_light;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'authenticator_aaguids' AND column_name = 'icon_dark'
    ) THEN
        ALTER TABLE authenticator_aaguids ADD COLUMN icon_dark TEXT;
        UPDATE authenticator_aaguids SET icon_dark = icon_light;
    END IF;
END $$;
