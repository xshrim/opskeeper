DELETE FROM resources WHERE kind IN ('Application', 'BusinessApplication', 'CronApplication');
DELETE FROM resource_schemas WHERE kind IN ('Application', 'BusinessApplication', 'CronApplication');

ALTER TABLE discovery_items
    ADD COLUMN IF NOT EXISTS imported_application_id uuid REFERENCES applications(id) ON DELETE SET NULL;

ALTER TABLE discovery_items DROP CONSTRAINT IF EXISTS discovery_items_kind_check;
UPDATE discovery_items SET kind = 'Workload' WHERE kind = 'Application';
ALTER TABLE discovery_items
    ADD CONSTRAINT discovery_items_kind_check CHECK (kind IN ('Project', 'Workload'));

ALTER TABLE discovery_items DROP COLUMN IF EXISTS imported_resource_id;
