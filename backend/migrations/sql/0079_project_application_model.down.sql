ALTER TABLE applications ADD COLUMN IF NOT EXISTS source text NOT NULL DEFAULT 'manual';
ALTER TABLE applications ALTER COLUMN status SET DEFAULT 'active';
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_runtime_kind_check;
ALTER TABLE applications DROP COLUMN IF EXISTS runtime_kind;
DROP TRIGGER IF EXISTS application_instances_validate_runtime ON application_instances;
DROP FUNCTION IF EXISTS validate_application_runtime();
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS source text NOT NULL DEFAULT 'manual',
    ADD COLUMN IF NOT EXISTS source_resource_id uuid REFERENCES resources(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS external_uid text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS last_synced_at timestamptz;
ALTER TABLE projects DROP COLUMN IF EXISTS description;
