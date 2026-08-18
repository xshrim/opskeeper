ALTER TABLE projects
    ADD COLUMN source_resource_id uuid REFERENCES resources(id) ON DELETE SET NULL,
    ADD COLUMN external_uid text NOT NULL DEFAULT '' CHECK (length(external_uid) <= 512),
    ADD COLUMN source_config jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(source_config) = 'object'),
    ADD COLUMN last_synced_at timestamptz;

CREATE UNIQUE INDEX projects_external_source_unique
    ON projects(source_resource_id, external_uid)
    WHERE deleted_at IS NULL AND source_resource_id IS NOT NULL AND external_uid <> '';

UPDATE resource_schemas SET kind = 'Kubernetes' WHERE kind = 'KubernetesCluster';
UPDATE resources SET kind = 'Kubernetes' WHERE kind = 'KubernetesCluster';
UPDATE resource_schemas SET kind = 'Application' WHERE kind = 'BusinessApplication';
UPDATE resources SET kind = 'Application' WHERE kind = 'BusinessApplication';
UPDATE resource_schemas SET kind = 'Artifact' WHERE kind = 'ArtifactStore';
UPDATE resources SET kind = 'Artifact' WHERE kind = 'ArtifactStore';

UPDATE resource_schemas
   SET display_name = 'Kubernetes',
       description = 'Kubernetes 集群连接、发现和同步入口。',
       icon = 'kubernetes'
 WHERE kind = 'Kubernetes';

UPDATE resource_schemas
   SET display_name = '应用',
       description = '项目中的核心应用；可关联 Kubernetes 工作负载、Instance 和访问入口。',
       icon = 'application'
 WHERE kind = 'Application';

UPDATE resource_schemas
   SET status = 'disabled'
 WHERE kind IN ('Endpoint', 'CronApplication');

UPDATE resource_schemas
   SET display_name = '制品仓库',
       description = '容器镜像、软件包或其他构建制品的存储仓库。',
       icon = 'storage',
       schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"url":{"title":"仓库 URL","type":"string"},"provider":{"title":"仓库类型","type":"string","enum":["oci","harbor","docker","maven","npm","generic"]},"namespace":{"title":"命名空间","type":"string"},"username":{"title":"用户名","type":"string","sensitive":true},"password":{"title":"密码","type":"string","sensitive":true},"token":{"title":"访问 Token","type":"string","sensitive":true}}}'::jsonb
 WHERE kind = 'Artifact';

INSERT INTO resource_schemas (kind, version, schema, status, display_name, description, icon)
VALUES (
    'Repository', 1,
    '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"url":{"title":"仓库 URL","type":"string"},"provider":{"title":"代码托管平台","type":"string","enum":["git","github","gitlab","gitea","bitbucket"]},"default_branch":{"title":"默认分支","type":"string"},"username":{"title":"用户名","type":"string","sensitive":true},"token":{"title":"访问 Token","type":"string","sensitive":true},"ssh_private_key":{"title":"SSH 私钥","type":"string","sensitive":true}}}'::jsonb,
    'active', '代码仓库', '保存应用源代码及版本历史的 Git 仓库。', 'repository'
)
ON CONFLICT (kind, version) DO UPDATE
SET schema = EXCLUDED.schema, status = 'active', display_name = EXCLUDED.display_name,
    description = EXCLUDED.description, icon = EXCLUDED.icon;

CREATE TABLE resource_roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    builtin boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE resource_role_permissions (
    role_id uuid NOT NULL REFERENCES resource_roles(id) ON DELETE CASCADE,
    permission text NOT NULL,
    PRIMARY KEY (role_id, permission)
);

CREATE TABLE resource_role_bindings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_type text NOT NULL CHECK (subject_type IN ('user', 'group')),
    subject_id uuid NOT NULL,
    role_id uuid NOT NULL REFERENCES resource_roles(id) ON DELETE CASCADE,
    resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (subject_type, subject_id, role_id, resource_id)
);

CREATE INDEX resource_role_bindings_subject_idx
    ON resource_role_bindings(subject_type, subject_id);
CREATE INDEX resource_role_bindings_resource_idx
    ON resource_role_bindings(resource_id);

CREATE OR REPLACE FUNCTION validate_resource_role_binding()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.subject_type = 'user' AND NOT EXISTS (
        SELECT 1 FROM users WHERE id = NEW.subject_id AND deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'resource role binding subject user does not exist' USING ERRCODE = '23503';
    END IF;
    IF NEW.subject_type = 'group' AND NOT EXISTS (
        SELECT 1 FROM groups WHERE id = NEW.subject_id AND deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'resource role binding subject group does not exist' USING ERRCODE = '23503';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER resource_role_bindings_validate
BEFORE INSERT OR UPDATE OF subject_type, subject_id ON resource_role_bindings
FOR EACH ROW EXECUTE FUNCTION validate_resource_role_binding();

CREATE TRIGGER resource_roles_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON resource_roles
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER resource_role_permissions_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON resource_role_permissions
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER resource_role_bindings_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON resource_role_bindings
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

INSERT INTO roles (name, scope_type, builtin)
VALUES ('ProjectMember', 'project', true)
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT id, 'organization:read' FROM roles WHERE name = 'ProjectMember'
ON CONFLICT DO NOTHING;

WITH seeded_roles(name) AS (
    VALUES ('ResourceAdmin'), ('ResourceOperator'), ('ResourceViewer')
)
INSERT INTO resource_roles (name)
SELECT name FROM seeded_roles
ON CONFLICT (name) DO NOTHING;

WITH permission_seed(role_name, permission) AS (
    VALUES
        ('ResourceAdmin', 'resource:read'), ('ResourceAdmin', 'resource:update'),
        ('ResourceAdmin', 'resource:delete'), ('ResourceAdmin', 'resource:use'),
        ('ResourceAdmin', 'relation:manage'), ('ResourceAdmin', 'discovery:run'),
        ('ResourceAdmin', 'diagnosis:start'), ('ResourceAdmin', 'diagnosis:read'),
        ('ResourceAdmin', 'inspection:manage'), ('ResourceAdmin', 'inspection:execute'),
        ('ResourceOperator', 'resource:read'), ('ResourceOperator', 'resource:use'),
        ('ResourceOperator', 'discovery:run'), ('ResourceOperator', 'diagnosis:start'),
        ('ResourceOperator', 'diagnosis:read'), ('ResourceOperator', 'inspection:execute'),
        ('ResourceViewer', 'resource:read'), ('ResourceViewer', 'diagnosis:read')
)
INSERT INTO resource_role_permissions (role_id, permission)
SELECT role.id, permission_seed.permission
  FROM permission_seed JOIN resource_roles role ON role.name = permission_seed.role_name
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT role.id, permission.name
  FROM roles role
 CROSS JOIN (VALUES ('discovery:run'), ('discovery:import')) AS permission(name)
 WHERE role.name = 'ProjectAdmin'
ON CONFLICT DO NOTHING;

CREATE TABLE discovery_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    error_message text NOT NULL DEFAULT '' CHECK (length(error_message) <= 2000),
    started_at timestamptz,
    completed_at timestamptz,
    item_count integer NOT NULL DEFAULT 0 CHECK (item_count >= 0),
    imported_count integer NOT NULL DEFAULT 0 CHECK (imported_count >= 0),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX discovery_runs_cluster_idx ON discovery_runs(cluster_resource_id, created_at DESC);
CREATE UNIQUE INDEX discovery_runs_one_active_per_cluster
    ON discovery_runs(cluster_resource_id)
    WHERE status IN ('queued', 'running');

CREATE TABLE discovery_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL REFERENCES discovery_runs(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('Project', 'Application')),
    namespace text NOT NULL DEFAULT '' CHECK (length(namespace) <= 253),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 253),
    external_uid text NOT NULL CHECK (length(btrim(external_uid)) BETWEEN 1 AND 512),
    resource_version text NOT NULL DEFAULT '' CHECK (length(resource_version) <= 512),
    labels jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(labels) = 'object'),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(payload) = 'object'),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'imported', 'ignored', 'missing')),
    imported_project_id uuid REFERENCES projects(id) ON DELETE SET NULL,
    imported_resource_id uuid REFERENCES resources(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (run_id, kind, external_uid)
);

CREATE INDEX discovery_items_run_idx ON discovery_items(run_id, status, kind);
CREATE INDEX discovery_items_identity_idx ON discovery_items(kind, external_uid);
