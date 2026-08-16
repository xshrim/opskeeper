DROP TABLE IF EXISTS discovery_items;
DROP TABLE IF EXISTS discovery_runs;

DELETE FROM role_permissions
 WHERE role_id = (SELECT id FROM roles WHERE name = 'ProjectAdmin')
   AND permission IN ('discovery:run', 'discovery:import');

DROP TABLE IF EXISTS resource_role_bindings;
DROP TABLE IF EXISTS resource_role_permissions;
DROP TABLE IF EXISTS resource_roles;
DROP FUNCTION IF EXISTS validate_resource_role_binding();

DELETE FROM role_bindings
 WHERE role_id = (SELECT id FROM roles WHERE name = 'ProjectMember');
DELETE FROM roles WHERE name = 'ProjectMember';

UPDATE resources SET kind = 'KubernetesCluster' WHERE kind = 'Kubernetes';
UPDATE resource_schemas SET kind = 'KubernetesCluster' WHERE kind = 'Kubernetes';
UPDATE resources SET kind = 'BusinessApplication' WHERE kind = 'Application';
UPDATE resource_schemas SET kind = 'BusinessApplication' WHERE kind = 'Application';
UPDATE resources SET kind = 'ArtifactStore' WHERE kind = 'Artifact';
UPDATE resource_schemas SET kind = 'ArtifactStore' WHERE kind = 'Artifact';

UPDATE resource_schemas
   SET display_name = 'Kubernetes 集群',
       description = '连接并管理 Kubernetes 集群的入口资源。',
       icon = 'kubernetes',
       schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"kubeconfig":{"title":"kubeconfig","type":"string","sensitive":true},"context":{"title":"Context","type":"string"},"api_server":{"title":"API Server","type":"string"}}}'::jsonb
 WHERE kind = 'KubernetesCluster';

UPDATE resource_schemas
   SET display_name = '业务应用',
       description = '面向业务的应用或服务集合。',
       icon = 'application',
       schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb
 WHERE kind = 'BusinessApplication';

UPDATE resource_schemas
   SET display_name = '制品存储',
       description = '保存诊断报告和其他制品的存储服务。',
       icon = 'storage',
       schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb
 WHERE kind = 'ArtifactStore';

UPDATE resource_schemas
   SET status = 'active'
 WHERE kind IN ('Endpoint', 'CronApplication');

DELETE FROM resource_schemas schema
 WHERE schema.kind = 'Repository'
   AND NOT EXISTS (SELECT 1 FROM resources resource WHERE resource.kind = 'Repository');
UPDATE resource_schemas SET status = 'disabled' WHERE kind = 'Repository';

DROP INDEX IF EXISTS projects_external_source_unique;
ALTER TABLE projects
    DROP COLUMN IF EXISTS last_synced_at,
    DROP COLUMN IF EXISTS source_config,
    DROP COLUMN IF EXISTS external_uid,
    DROP COLUMN IF EXISTS source_resource_id;
