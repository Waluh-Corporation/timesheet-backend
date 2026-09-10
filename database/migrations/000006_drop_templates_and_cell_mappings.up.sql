-- 000006_drop_templates_and_cell_mappings.up.sql
-- Drop deprecated cell_mappings and templates tables in favor of programmatic company excel builders

DROP TABLE IF EXISTS cell_mappings CASCADE;
DROP TABLE IF EXISTS templates CASCADE;
