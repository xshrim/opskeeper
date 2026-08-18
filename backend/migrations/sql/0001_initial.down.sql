-- OpsKeeper rollback for the fresh database baseline.
-- Generated from backend/migrations/sql/archive/*.down.sql in reverse order.

-- <<< 0019_usernames.down.sql
DROP INDEX IF EXISTS users_username_unique;
DROP INDEX IF EXISTS users_phone_unique;
UPDATE users SET email = username || '@invalid.local' WHERE email IS NULL;
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_username_format,
    DROP CONSTRAINT IF EXISTS users_email_nonempty,
    DROP CONSTRAINT IF EXISTS users_phone_format,
    DROP COLUMN IF EXISTS username,
    DROP COLUMN IF EXISTS phone;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

-- <<< 0018_audit_retention.down.sql
DROP FUNCTION IF EXISTS prune_audit_events(timestamptz, text, text);
DROP TRIGGER IF EXISTS audit_retention_runs_append_only ON audit_retention_runs;
DROP TRIGGER IF EXISTS audit_events_append_only ON audit_events;
DROP FUNCTION IF EXISTS protect_audit_rows();
DROP TABLE IF EXISTS audit_retention_runs;

-- <<< 0017_mcp_server_schema.down.sql
DELETE FROM resource_schemas WHERE kind = 'MCPServer' AND version = 3;

-- <<< 0016_mcp_operations.down.sql
DELETE FROM role_permissions WHERE permission = 'operation:approve';
DROP TABLE IF EXISTS operation_executions;
DROP TABLE IF EXISTS operation_approvals;
DROP TABLE IF EXISTS operation_requests;
DROP TABLE IF EXISTS operation_policies;
DROP TABLE IF EXISTS mcp_server_snapshots;

-- <<< 0015_inspection_notification.down.sql
DROP TABLE IF EXISTS notification_deliveries;
DROP TABLE IF EXISTS notification_channels;
DROP TABLE IF EXISTS inspection_health_snapshots;
DROP TABLE IF EXISTS inspection_findings;
DROP TABLE IF EXISTS inspection_jobs;
DROP TABLE IF EXISTS inspection_run_steps;
DROP TABLE IF EXISTS inspection_runs;
DROP TABLE IF EXISTS inspection_policy_targets;
DROP TABLE IF EXISTS inspection_policies;

-- <<< 0014_builtin_skill_output_contract.down.sql
DELETE FROM skill_versions AS version
 USING resources AS resource
 WHERE version.skill_resource_id = resource.id
   AND version.version = 2
   AND resource.kind = 'Skill'
   AND resource.config->>'owner' = 'OpsKeeper builtin';

UPDATE skill_versions AS version
   SET status = 'published', published_at = COALESCE(version.published_at, now())
  FROM resources AS resource
 WHERE version.skill_resource_id = resource.id
   AND version.version = 1
   AND resource.kind = 'Skill'
   AND resource.config->>'owner' = 'OpsKeeper builtin';

-- <<< 0013_builtin_skills.down.sql
DELETE FROM skill_versions AS version
 USING resources AS resource
 WHERE version.skill_resource_id = resource.id
   AND resource.kind = 'Skill'
   AND resource.config->>'owner' = 'OpsKeeper builtin';

DELETE FROM resources
 WHERE kind = 'Skill'
   AND config->>'owner' = 'OpsKeeper builtin'
   AND name IN ('Kubernetes 工作负载诊断', 'PostgreSQL 健康诊断', 'Redis 健康诊断', 'Kafka 健康诊断');

-- <<< 0012_diagnosis_workbench.down.sql
DROP TABLE IF EXISTS diagnosis_reports;
DROP TABLE IF EXISTS diagnosis_hypotheses;
DROP TABLE IF EXISTS diagnosis_evidence;
DROP TABLE IF EXISTS diagnosis_events;
DROP TABLE IF EXISTS diagnosis_plan_steps;
DROP TABLE IF EXISTS diagnosis_plans;
DROP TABLE IF EXISTS diagnosis_messages;
DROP TABLE IF EXISTS diagnosis_targets;
DROP TABLE IF EXISTS diagnosis_sessions;

-- <<< 0011_llm_skill_runner.down.sql
DROP TRIGGER IF EXISTS skill_scope_defaults_authorization_revision ON skill_scope_defaults;
DROP TRIGGER IF EXISTS skill_versions_authorization_revision ON skill_versions;
DROP TRIGGER IF EXISTS llm_scope_defaults_authorization_revision ON llm_scope_defaults;

DELETE FROM role_permissions
 WHERE permission IN ('skill:manage', 'skill:execute')
   AND role_id IN (SELECT id FROM roles WHERE name IN (
       'PlatformOperator', 'TeamAdmin', 'TeamOperator',
       'ProjectAdmin', 'ProjectOperator'
   ));

DELETE FROM resource_schemas WHERE kind IN ('LLMProvider', 'Skill', 'MCPServer') AND version = 2;

DROP TRIGGER IF EXISTS skill_scope_defaults_validate ON skill_scope_defaults;
DROP FUNCTION IF EXISTS validate_skill_scope_default();
DROP TRIGGER IF EXISTS skill_versions_validate ON skill_versions;
DROP FUNCTION IF EXISTS validate_skill_version();
DROP TRIGGER IF EXISTS llm_scope_defaults_validate ON llm_scope_defaults;
DROP FUNCTION IF EXISTS validate_llm_scope_default();

DROP TABLE IF EXISTS skill_tool_calls;
DROP TABLE IF EXISTS skill_executions;
DROP TABLE IF EXISTS skill_scope_defaults;
DROP TABLE IF EXISTS skill_versions;
DROP TABLE IF EXISTS llm_scope_defaults;

-- <<< 0010_connector_runtime.down.sql
DROP TABLE IF EXISTS resource_connection_checks;

UPDATE resource_schemas
   SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb,
       display_name = 'Prometheus',
       description = 'Prometheus 指标服务连接配置。',
       icon = 'metrics'
 WHERE kind = 'Prometheus';

UPDATE resource_schemas
   SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb,
       display_name = 'Loki',
       description = 'Loki 日志服务连接配置。',
       icon = 'logs'
 WHERE kind = 'Loki';

-- <<< 0009_kubernetes_discovery.down.sql
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

-- <<< 0008_organization_icons.down.sql
ALTER TABLE teams DROP CONSTRAINT IF EXISTS teams_icon_length;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS projects_icon_length;
ALTER TABLE platforms DROP CONSTRAINT IF EXISTS platforms_icon_length;

ALTER TABLE teams DROP COLUMN IF EXISTS icon;
ALTER TABLE projects DROP COLUMN IF EXISTS icon;
ALTER TABLE platforms DROP COLUMN IF EXISTS icon;

-- <<< 0007_resource_catalog_metadata.down.sql
UPDATE resource_schemas
   SET status = 'active'
 WHERE kind IN ('Namespace', 'Node', 'Workload', 'Pod', 'Service', 'Ingress', 'Model', 'Credential');

ALTER TABLE resource_schemas
    DROP COLUMN IF EXISTS display_name,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS icon;

-- <<< 0006_resource_catalog.down.sql
DROP TRIGGER IF EXISTS scope_defaults_authorization_revision ON scope_defaults;
DROP TRIGGER IF EXISTS resource_relations_authorization_revision ON resource_relations;
DROP TRIGGER IF EXISTS resource_credentials_authorization_revision ON resource_credentials;
DROP TRIGGER IF EXISTS resources_authorization_revision ON resources;
DROP TRIGGER IF EXISTS scope_defaults_validate ON scope_defaults;
DROP TRIGGER IF EXISTS resources_validate_scope_move ON resources;
DROP TRIGGER IF EXISTS resource_relations_validate ON resource_relations;
DROP TRIGGER IF EXISTS resources_validate_credential_scope ON resources;
DROP TRIGGER IF EXISTS resource_credentials_validate_scope ON resource_credentials;
DROP FUNCTION IF EXISTS validate_scope_default();
DROP FUNCTION IF EXISTS validate_resource_scope_move();
DROP FUNCTION IF EXISTS validate_resource_relation();
DROP FUNCTION IF EXISTS validate_resource_credential_scope();
DROP FUNCTION IF EXISTS validate_resource_credential_record_scope();
DROP FUNCTION IF EXISTS resource_scope_contains(uuid, uuid);
DROP TABLE IF EXISTS scope_defaults;
DROP TABLE IF EXISTS resource_relations;
DROP TABLE IF EXISTS resource_sync_states;
DROP TABLE IF EXISTS resources;
DROP TABLE IF EXISTS resource_credentials;
DROP TABLE IF EXISTS resource_schemas;

-- <<< 0005_access_audit.down.sql
DROP TRIGGER IF EXISTS role_bindings_authorization_revision ON role_bindings;
DROP TRIGGER IF EXISTS role_permissions_authorization_revision ON role_permissions;
DROP TRIGGER IF EXISTS roles_authorization_revision ON roles;
DROP TRIGGER IF EXISTS group_members_authorization_revision ON group_members;
DROP TRIGGER IF EXISTS groups_authorization_revision ON groups;
DROP TRIGGER IF EXISTS scopes_authorization_revision ON scopes;
DROP TRIGGER IF EXISTS users_authorization_revision ON users;
DROP FUNCTION IF EXISTS bump_authorization_revision();
DROP TABLE authorization_revision;

DROP TABLE audit_events;
DROP TABLE group_members;
DROP TABLE groups;

DELETE FROM role_bindings WHERE subject_type = 'group';
ALTER TABLE role_bindings DROP CONSTRAINT role_bindings_subject_type_check;
ALTER TABLE role_bindings ADD CONSTRAINT role_bindings_subject_type_check CHECK (subject_type = 'user');
ALTER TABLE role_bindings ADD CONSTRAINT role_bindings_subject_id_fkey FOREIGN KEY (subject_id) REFERENCES users(id);

CREATE OR REPLACE FUNCTION validate_role_binding()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    role_scope_type text;
    binding_scope_type text;
    user_exists boolean;
BEGIN
    SELECT scope_type INTO role_scope_type FROM roles WHERE id = NEW.role_id;
    SELECT scope_type INTO binding_scope_type
      FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    SELECT EXISTS (SELECT 1 FROM users WHERE id = NEW.subject_id AND deleted_at IS NULL)
      INTO user_exists;
    IF role_scope_type IS NULL OR binding_scope_type IS NULL OR role_scope_type <> binding_scope_type THEN
        RAISE EXCEPTION 'role binding scope type does not match role' USING ERRCODE = '23514';
    END IF;
    IF NOT user_exists THEN
        RAISE EXCEPTION 'role binding subject user does not exist' USING ERRCODE = '23503';
    END IF;
    RETURN NEW;
END;
$$;

-- <<< 0004_rbac.down.sql
DROP TABLE role_bindings;
DROP TABLE role_permissions;
DROP TABLE roles;

-- <<< 0003_identity_session.down.sql
DROP TABLE sessions;
DROP TABLE credentials;
DROP TABLE users;

-- <<< 0002_scope_status.down.sql
ALTER TABLE platforms ADD COLUMN status text;
UPDATE platforms AS organization
   SET status = scope.status
  FROM scopes AS scope
 WHERE scope.id = organization.scope_id;
ALTER TABLE platforms ALTER COLUMN status SET DEFAULT 'active';
ALTER TABLE platforms ALTER COLUMN status SET NOT NULL;
ALTER TABLE platforms ADD CONSTRAINT platforms_status_check CHECK (status IN ('active', 'disabled'));

ALTER TABLE teams ADD COLUMN status text;
UPDATE teams AS organization
   SET status = scope.status
  FROM scopes AS scope
 WHERE scope.id = organization.scope_id;
ALTER TABLE teams ALTER COLUMN status SET DEFAULT 'active';
ALTER TABLE teams ALTER COLUMN status SET NOT NULL;
ALTER TABLE teams ADD CONSTRAINT teams_status_check CHECK (status IN ('active', 'disabled'));

ALTER TABLE projects ADD COLUMN status text;
UPDATE projects AS organization
   SET status = scope.status
  FROM scopes AS scope
 WHERE scope.id = organization.scope_id;
ALTER TABLE projects ALTER COLUMN status SET DEFAULT 'active';
ALTER TABLE projects ALTER COLUMN status SET NOT NULL;
ALTER TABLE projects ADD CONSTRAINT projects_status_check CHECK (status IN ('active', 'disabled'));

-- <<< 0001_scope_organization.down.sql
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS platforms;

DROP FUNCTION IF EXISTS validate_organization_scope();

DROP TRIGGER IF EXISTS scopes_validate_parent ON scopes;
DROP FUNCTION IF EXISTS validate_scope_parent();
DROP TABLE IF EXISTS scopes;
