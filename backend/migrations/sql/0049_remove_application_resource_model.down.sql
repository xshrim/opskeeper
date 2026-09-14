SELECT 1;
ALTER TABLE discovery_items DROP CONSTRAINT IF EXISTS discovery_items_kind_check;
ALTER TABLE discovery_items
    ADD COLUMN IF NOT EXISTS imported_resource_id uuid REFERENCES resources(id) ON DELETE SET NULL;
UPDATE discovery_items SET kind = 'Application' WHERE kind = 'Workload';
ALTER TABLE discovery_items DROP COLUMN IF EXISTS imported_application_id;
ALTER TABLE discovery_items
    ADD CONSTRAINT discovery_items_kind_check CHECK (kind IN ('Project', 'Application'));
