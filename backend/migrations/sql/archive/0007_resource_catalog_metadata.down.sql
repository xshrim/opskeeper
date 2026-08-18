UPDATE resource_schemas
   SET status = 'active'
 WHERE kind IN ('Namespace', 'Node', 'Workload', 'Pod', 'Service', 'Ingress', 'Model', 'Credential');

ALTER TABLE resource_schemas
    DROP COLUMN IF EXISTS display_name,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS icon;
