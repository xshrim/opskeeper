-- Resource catalog changes are intentionally forward-only. Re-enable the
-- previous catch-all schemas if this migration is rolled back.
UPDATE resource_schemas
   SET status = 'active'
 WHERE kind IN ('GenericMiddleware', 'GenericAPI');

DELETE FROM resource_schemas
 WHERE version = 1
   AND kind IN ('Host', 'Docker', 'TongRDS', 'RabbitMQ', 'OceanBase', 'Oracle', 'MySQL');

UPDATE resource_schemas SET status = 'active' WHERE kind = 'KubernetesCluster';
