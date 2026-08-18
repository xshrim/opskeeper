-- OpsKeeper fresh database baseline.
-- Generated from backend/migrations/sql/archive/*.sql in version order.

-- >>> 0001_scope_organization.sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE scopes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL DEFAULT 'default',
    scope_type text NOT NULL CHECK (scope_type IN ('platform', 'team', 'project')),
    parent_scope_id uuid REFERENCES scopes(id),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CHECK (
        (scope_type = 'platform' AND parent_scope_id IS NULL)
        OR (scope_type IN ('team', 'project') AND parent_scope_id IS NOT NULL)
    )
);

CREATE INDEX scopes_parent_scope_id_idx ON scopes(parent_scope_id) WHERE deleted_at IS NULL;
CREATE INDEX scopes_tenant_type_idx ON scopes(tenant_id, scope_type) WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION validate_scope_parent()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    parent_type text;
    parent_tenant text;
BEGIN
    IF NEW.scope_type = 'platform' THEN
        IF NEW.parent_scope_id IS NOT NULL THEN
            RAISE EXCEPTION 'platform scope cannot have a parent' USING ERRCODE = '23514';
        END IF;
        RETURN NEW;
    END IF;

    SELECT scope_type, tenant_id
      INTO parent_type, parent_tenant
      FROM scopes
     WHERE id = NEW.parent_scope_id
       AND deleted_at IS NULL;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'scope parent does not exist' USING ERRCODE = '23503';
    END IF;
    IF parent_tenant <> NEW.tenant_id THEN
        RAISE EXCEPTION 'scope parent belongs to a different tenant' USING ERRCODE = '23514';
    END IF;
    IF NEW.scope_type = 'team' AND parent_type <> 'platform' THEN
        RAISE EXCEPTION 'team scope parent must be platform' USING ERRCODE = '23514';
    END IF;
    IF NEW.scope_type = 'project' AND parent_type <> 'team' THEN
        RAISE EXCEPTION 'project scope parent must be team' USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER scopes_validate_parent
BEFORE INSERT OR UPDATE OF tenant_id, scope_type, parent_scope_id ON scopes
FOR EACH ROW EXECUTE FUNCTION validate_scope_parent();

CREATE TABLE platforms (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL UNIQUE REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    code text NOT NULL UNIQUE CHECK (code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE TABLE teams (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL UNIQUE REFERENCES scopes(id),
    platform_id uuid NOT NULL REFERENCES platforms(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    code text NOT NULL CHECK (code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    labels jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(labels) = 'object'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (platform_id, code)
);

CREATE INDEX teams_platform_id_idx ON teams(platform_id) WHERE deleted_at IS NULL;

CREATE TABLE projects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL UNIQUE REFERENCES scopes(id),
    platform_id uuid NOT NULL REFERENCES platforms(id),
    team_id uuid NOT NULL REFERENCES teams(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    code text NOT NULL CHECK (code ~ '^[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?$'),
    labels jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(labels) = 'object'),
    source text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'kubernetes')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (team_id, code)
);

CREATE INDEX projects_platform_id_idx ON projects(platform_id) WHERE deleted_at IS NULL;
CREATE INDEX projects_team_id_idx ON projects(team_id) WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION validate_organization_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    actual_scope_type text;
    actual_parent uuid;
    expected_parent uuid;
BEGIN
    SELECT scope_type, parent_scope_id
      INTO actual_scope_type, actual_parent
      FROM scopes
     WHERE id = NEW.scope_id
       AND deleted_at IS NULL;

    IF TG_TABLE_NAME = 'platforms' THEN
        IF actual_scope_type <> 'platform' OR actual_parent IS NOT NULL THEN
            RAISE EXCEPTION 'platform must reference a platform root scope' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'teams' THEN
        SELECT scope_id INTO expected_parent FROM platforms WHERE id = NEW.platform_id AND deleted_at IS NULL;
        IF actual_scope_type <> 'team' OR actual_parent IS DISTINCT FROM expected_parent THEN
            RAISE EXCEPTION 'team scope does not match platform' USING ERRCODE = '23514';
        END IF;
    ELSIF TG_TABLE_NAME = 'projects' THEN
        SELECT scope_id INTO expected_parent FROM teams WHERE id = NEW.team_id AND platform_id = NEW.platform_id AND deleted_at IS NULL;
        IF actual_scope_type <> 'project' OR actual_parent IS DISTINCT FROM expected_parent THEN
            RAISE EXCEPTION 'project scope does not match team and platform' USING ERRCODE = '23514';
        END IF;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER platforms_validate_scope
BEFORE INSERT OR UPDATE OF scope_id ON platforms
FOR EACH ROW EXECUTE FUNCTION validate_organization_scope();

CREATE TRIGGER teams_validate_scope
BEFORE INSERT OR UPDATE OF scope_id, platform_id ON teams
FOR EACH ROW EXECUTE FUNCTION validate_organization_scope();

CREATE TRIGGER projects_validate_scope
BEFORE INSERT OR UPDATE OF scope_id, platform_id, team_id ON projects
FOR EACH ROW EXECUTE FUNCTION validate_organization_scope();

DO $$
DECLARE
    root_scope_id uuid;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM platforms WHERE code = 'default' AND deleted_at IS NULL) THEN
        INSERT INTO scopes (scope_type, status)
        VALUES ('platform', 'active')
        RETURNING id INTO root_scope_id;

        INSERT INTO platforms (scope_id, name, code, status)
        VALUES (root_scope_id, 'OpsKeeper', 'default', 'active');
    END IF;
END;
$$;

-- >>> 0002_scope_status.sql
ALTER TABLE platforms DROP COLUMN status;
ALTER TABLE teams DROP COLUMN status;
ALTER TABLE projects DROP COLUMN status;

-- >>> 0003_identity_session.sql
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    display_name text NOT NULL CHECK (length(btrim(display_name)) BETWEEN 1 AND 120),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'locked')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX users_email_unique ON users (lower(email)) WHERE deleted_at IS NULL;

CREATE TABLE credentials (
    user_id uuid PRIMARY KEY REFERENCES users(id),
    password_hash text NOT NULL,
    password_changed_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    access_token_hash bytea NOT NULL UNIQUE,
    refresh_token_hash bytea NOT NULL UNIQUE,
    access_expires_at timestamptz NOT NULL,
    refresh_expires_at timestamptz NOT NULL,
    user_agent text NOT NULL DEFAULT '' CHECK (length(user_agent) <= 512),
    client_ip text NOT NULL DEFAULT '' CHECK (length(client_ip) <= 255),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id, created_at DESC);
CREATE INDEX sessions_refresh_lookup_idx ON sessions(refresh_token_hash) WHERE revoked_at IS NULL;
CREATE INDEX sessions_access_lookup_idx ON sessions(access_token_hash) WHERE revoked_at IS NULL;

-- >>> 0004_rbac.sql
CREATE TABLE roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    scope_type text NOT NULL CHECK (scope_type IN ('platform', 'team', 'project')),
    builtin boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
    role_id uuid NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission text NOT NULL,
    PRIMARY KEY (role_id, permission)
);

CREATE TABLE role_bindings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_type text NOT NULL CHECK (subject_type = 'user'),
    subject_id uuid NOT NULL REFERENCES users(id),
    role_id uuid NOT NULL REFERENCES roles(id),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (subject_type, subject_id, role_id, scope_id)
);

CREATE INDEX role_bindings_subject_idx ON role_bindings(subject_type, subject_id);
CREATE INDEX role_bindings_scope_idx ON role_bindings(scope_id);

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

CREATE TRIGGER role_bindings_validate
BEFORE INSERT OR UPDATE OF subject_id, role_id, scope_id ON role_bindings
FOR EACH ROW EXECUTE FUNCTION validate_role_binding();

WITH seeded_roles(name, scope_type) AS (
    VALUES
        ('PlatformAdmin', 'platform'),
        ('PlatformOperator', 'platform'),
        ('PlatformViewer', 'platform'),
        ('TeamAdmin', 'team'),
        ('TeamOperator', 'team'),
        ('TeamViewer', 'team'),
        ('ProjectAdmin', 'project'),
        ('ProjectOperator', 'project'),
        ('ProjectViewer', 'project')
)
INSERT INTO roles (name, scope_type)
SELECT name, scope_type FROM seeded_roles
ON CONFLICT (name) DO NOTHING;

WITH role_permissions_seed(role_name, permission) AS (
    VALUES
        ('PlatformAdmin', 'organization:read'),
        ('PlatformAdmin', 'team:manage'),
        ('PlatformAdmin', 'project:manage'),
        ('PlatformAdmin', 'resource:read'),
        ('PlatformAdmin', 'resource:create'),
        ('PlatformAdmin', 'resource:update'),
        ('PlatformAdmin', 'resource:delete'),
        ('PlatformAdmin', 'resource:use'),
        ('PlatformAdmin', 'credential:manage'),
        ('PlatformAdmin', 'credential:test'),
        ('PlatformAdmin', 'relation:manage'),
        ('PlatformAdmin', 'discovery:run'),
        ('PlatformAdmin', 'discovery:import'),
        ('PlatformAdmin', 'diagnosis:start'),
        ('PlatformAdmin', 'diagnosis:read'),
        ('PlatformAdmin', 'skill:execute'),
        ('PlatformAdmin', 'skill:manage'),
        ('PlatformAdmin', 'inspection:manage'),
        ('PlatformAdmin', 'inspection:execute'),
        ('PlatformAdmin', 'operation:approve'),
        ('PlatformAdmin', 'audit:read'),
        ('PlatformOperator', 'organization:read'),
        ('PlatformOperator', 'resource:read'),
        ('PlatformOperator', 'resource:use'),
        ('PlatformOperator', 'diagnosis:start'),
        ('PlatformOperator', 'diagnosis:read'),
        ('PlatformOperator', 'discovery:run'),
        ('PlatformOperator', 'inspection:execute'),
        ('PlatformViewer', 'organization:read'),
        ('PlatformViewer', 'resource:read'),
        ('PlatformViewer', 'diagnosis:read'),
        ('TeamAdmin', 'organization:read'),
        ('TeamAdmin', 'team:manage'),
        ('TeamAdmin', 'project:manage'),
        ('TeamAdmin', 'resource:read'),
        ('TeamAdmin', 'resource:create'),
        ('TeamAdmin', 'resource:update'),
        ('TeamAdmin', 'resource:delete'),
        ('TeamAdmin', 'resource:use'),
        ('TeamAdmin', 'credential:manage'),
        ('TeamAdmin', 'credential:test'),
        ('TeamAdmin', 'relation:manage'),
        ('TeamAdmin', 'discovery:run'),
        ('TeamAdmin', 'discovery:import'),
        ('TeamOperator', 'organization:read'),
        ('TeamOperator', 'resource:read'),
        ('TeamOperator', 'resource:use'),
        ('TeamOperator', 'diagnosis:start'),
        ('TeamOperator', 'diagnosis:read'),
        ('TeamOperator', 'discovery:run'),
        ('TeamOperator', 'inspection:execute'),
        ('TeamViewer', 'organization:read'),
        ('TeamViewer', 'resource:read'),
        ('TeamViewer', 'diagnosis:read'),
        ('ProjectAdmin', 'organization:read'),
        ('ProjectAdmin', 'project:manage'),
        ('ProjectAdmin', 'resource:read'),
        ('ProjectAdmin', 'resource:create'),
        ('ProjectAdmin', 'resource:update'),
        ('ProjectAdmin', 'resource:delete'),
        ('ProjectAdmin', 'resource:use'),
        ('ProjectAdmin', 'credential:manage'),
        ('ProjectAdmin', 'credential:test'),
        ('ProjectAdmin', 'relation:manage'),
        ('ProjectOperator', 'organization:read'),
        ('ProjectOperator', 'resource:read'),
        ('ProjectOperator', 'resource:use'),
        ('ProjectOperator', 'diagnosis:start'),
        ('ProjectOperator', 'diagnosis:read'),
        ('ProjectOperator', 'inspection:execute'),
        ('ProjectViewer', 'organization:read'),
        ('ProjectViewer', 'resource:read'),
        ('ProjectViewer', 'diagnosis:read')
)
INSERT INTO role_permissions (role_id, permission)
SELECT roles.id, seed.permission
  FROM role_permissions_seed seed
  JOIN roles ON roles.name = seed.role_name
ON CONFLICT DO NOTHING;

-- >>> 0005_access_audit.sql
CREATE TABLE groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX groups_scope_name_unique ON groups(scope_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX groups_scope_idx ON groups(scope_id) WHERE deleted_at IS NULL;

CREATE TABLE group_members (
    group_id uuid NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX group_members_user_idx ON group_members(user_id, group_id);

ALTER TABLE role_bindings DROP CONSTRAINT role_bindings_subject_id_fkey;
ALTER TABLE role_bindings DROP CONSTRAINT role_bindings_subject_type_check;
ALTER TABLE role_bindings ADD CONSTRAINT role_bindings_subject_type_check CHECK (subject_type IN ('user', 'group'));

CREATE OR REPLACE FUNCTION validate_role_binding()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    role_scope_type text;
    binding_scope_type text;
    subject_exists boolean;
BEGIN
    SELECT scope_type INTO role_scope_type FROM roles WHERE id = NEW.role_id;
    SELECT scope_type INTO binding_scope_type
      FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    IF NEW.subject_type = 'user' THEN
        SELECT EXISTS (SELECT 1 FROM users WHERE id = NEW.subject_id AND deleted_at IS NULL)
          INTO subject_exists;
    ELSE
        SELECT EXISTS (SELECT 1 FROM groups WHERE id = NEW.subject_id AND deleted_at IS NULL AND status = 'active')
          INTO subject_exists;
    END IF;

    IF role_scope_type IS NULL OR binding_scope_type IS NULL OR role_scope_type <> binding_scope_type THEN
        RAISE EXCEPTION 'role binding scope type does not match role' USING ERRCODE = '23514';
    END IF;
    IF NOT subject_exists THEN
        RAISE EXCEPTION 'role binding subject does not exist or is inactive' USING ERRCODE = '23503';
    END IF;
    RETURN NEW;
END;
$$;

INSERT INTO role_permissions (role_id, permission)
SELECT roles.id, 'member:grant'
  FROM roles
 WHERE roles.name IN ('PlatformAdmin', 'TeamAdmin', 'ProjectAdmin')
ON CONFLICT DO NOTHING;

CREATE TABLE audit_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL CHECK (length(btrim(action)) BETWEEN 1 AND 120),
    target_type text NOT NULL DEFAULT '',
    target_id text NOT NULL DEFAULT '',
    scope_id uuid REFERENCES scopes(id) ON DELETE SET NULL,
    result text NOT NULL CHECK (result IN ('success', 'failure')),
    request_id text NOT NULL DEFAULT '' CHECK (length(request_id) <= 128),
    client_ip text NOT NULL DEFAULT '' CHECK (length(client_ip) <= 255),
    details jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(details) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_events_created_at_idx ON audit_events(created_at DESC);
CREATE INDEX audit_events_actor_idx ON audit_events(actor_user_id, created_at DESC);
CREATE INDEX audit_events_scope_idx ON audit_events(scope_id, created_at DESC);

CREATE TABLE authorization_revision (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    revision bigint NOT NULL DEFAULT 1
);

INSERT INTO authorization_revision (id, revision) VALUES (true, 1);

CREATE OR REPLACE FUNCTION bump_authorization_revision()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    UPDATE authorization_revision SET revision = revision + 1 WHERE id = true;
    RETURN COALESCE(NEW, OLD);
END;
$$;

CREATE TRIGGER users_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER scopes_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON scopes
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER groups_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON groups
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER group_members_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON group_members
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER roles_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON roles
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER role_permissions_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON role_permissions
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER role_bindings_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON role_bindings
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

-- >>> 0006_resource_catalog.sql
CREATE TABLE resource_schemas (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL CHECK (length(btrim(kind)) BETWEEN 1 AND 120),
    version integer NOT NULL CHECK (version > 0),
    schema jsonb NOT NULL CHECK (jsonb_typeof(schema) = 'object'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kind, version)
);

CREATE INDEX resource_schemas_kind_idx ON resource_schemas(kind, version DESC) WHERE status = 'active';

CREATE TABLE resource_credentials (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    purpose text NOT NULL DEFAULT '' CHECK (length(purpose) <= 500),
    ciphertext bytea NOT NULL CHECK (octet_length(ciphertext) > 0),
    encryption_algorithm text NOT NULL DEFAULT 'AES-256-GCM',
    key_version text NOT NULL DEFAULT 'local-v1',
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX resource_credentials_scope_name_unique
    ON resource_credentials(scope_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX resource_credentials_scope_idx
    ON resource_credentials(scope_id) WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION validate_resource_credential_record_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    scope_status text;
BEGIN
    SELECT status INTO scope_status FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    IF scope_status IS DISTINCT FROM 'active' THEN
        RAISE EXCEPTION 'credential scope is inactive or missing' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER resource_credentials_validate_scope
BEFORE INSERT OR UPDATE OF scope_id ON resource_credentials
FOR EACH ROW EXECUTE FUNCTION validate_resource_credential_record_scope();

CREATE TABLE resources (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL DEFAULT 'default',
    scope_id uuid NOT NULL REFERENCES scopes(id),
    kind text NOT NULL CHECK (length(btrim(kind)) BETWEEN 1 AND 120),
    schema_version integer NOT NULL DEFAULT 1 CHECK (schema_version > 0),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    external_uid text NOT NULL DEFAULT '' CHECK (length(external_uid) <= 512),
    source_resource_id text NOT NULL DEFAULT '' CHECK (length(source_resource_id) <= 512),
    labels jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(labels) = 'object'),
    config jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(config) = 'object'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'unknown')),
    credential_id uuid REFERENCES resource_credentials(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX resources_scope_kind_name_unique
    ON resources(scope_id, kind, lower(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX resources_external_identity_unique
    ON resources(scope_id, kind, external_uid, source_resource_id)
    WHERE deleted_at IS NULL AND external_uid <> '' AND source_resource_id <> '';
CREATE INDEX resources_scope_idx ON resources(scope_id) WHERE deleted_at IS NULL;
CREATE INDEX resources_kind_idx ON resources(kind) WHERE deleted_at IS NULL;
CREATE INDEX resources_labels_idx ON resources USING gin(labels) WHERE deleted_at IS NULL;

CREATE TABLE resource_sync_states (
    resource_id uuid PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
    state text NOT NULL DEFAULT 'never' CHECK (state IN ('never', 'running', 'succeeded', 'failed')),
    last_started_at timestamptz,
    last_completed_at timestamptz,
    last_error text NOT NULL DEFAULT '' CHECK (length(last_error) <= 2000),
    external_version text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE resource_relations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    target_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    relation_type text NOT NULL CHECK (relation_type IN ('contains', 'deployed_on', 'depends_on', 'observed_by', 'exposes', 'uses_provider', 'uses_skill', 'served_by_mcp')),
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(attributes) = 'object'),
    discovery_source text NOT NULL DEFAULT 'manual' CHECK (discovery_source IN ('manual', 'discovery', 'import')),
    confidence numeric(5,4) NOT NULL DEFAULT 1 CHECK (confidence >= 0 AND confidence <= 1),
    confirmed boolean NOT NULL DEFAULT true,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_resource_id, target_resource_id, relation_type)
);

CREATE INDEX resource_relations_source_idx ON resource_relations(source_resource_id);
CREATE INDEX resource_relations_target_idx ON resource_relations(target_resource_id);

CREATE TABLE scope_defaults (
    scope_id uuid NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    default_key text NOT NULL CHECK (length(btrim(default_key)) BETWEEN 1 AND 120),
    resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_id, default_key)
);

CREATE OR REPLACE FUNCTION resource_scope_contains(ancestor_id uuid, descendant_id uuid)
RETURNS boolean
LANGUAGE sql
STABLE
AS $$
    WITH RECURSIVE chain(id) AS (
        SELECT descendant_id
        UNION
        SELECT scopes.parent_scope_id
          FROM scopes
          JOIN chain ON chain.id = scopes.id
         WHERE scopes.parent_scope_id IS NOT NULL
    )
    SELECT EXISTS (SELECT 1 FROM chain WHERE id = ancestor_id);
$$;

CREATE OR REPLACE FUNCTION validate_resource_credential_scope()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    credential_scope uuid;
    scope_status text;
BEGIN
    SELECT status INTO scope_status FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    IF scope_status IS DISTINCT FROM 'active' THEN
        RAISE EXCEPTION 'resource scope is inactive or missing' USING ERRCODE = '23514';
    END IF;
    IF NEW.credential_id IS NULL THEN
        RETURN NEW;
    END IF;
    SELECT scope_id INTO credential_scope
      FROM resource_credentials
     WHERE id = NEW.credential_id AND deleted_at IS NULL;
    IF credential_scope IS NULL OR NOT resource_scope_contains(credential_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'resource credential is outside resource scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER resources_validate_credential_scope
BEFORE INSERT OR UPDATE OF scope_id, credential_id ON resources
FOR EACH ROW EXECUTE FUNCTION validate_resource_credential_scope();

CREATE OR REPLACE FUNCTION validate_resource_relation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    source_scope uuid;
    target_scope uuid;
    cycle_found boolean;
BEGIN
    IF NEW.source_resource_id = NEW.target_resource_id THEN
        RAISE EXCEPTION 'resource relation cannot point to itself' USING ERRCODE = '23514';
    END IF;
    SELECT scope_id INTO source_scope FROM resources WHERE id = NEW.source_resource_id AND deleted_at IS NULL;
    SELECT scope_id INTO target_scope FROM resources WHERE id = NEW.target_resource_id AND deleted_at IS NULL;
    IF source_scope IS NULL OR target_scope IS NULL OR NOT resource_scope_contains(target_scope, source_scope) THEN
        RAISE EXCEPTION 'resource relation target is outside the source scope chain' USING ERRCODE = '23514';
    END IF;

    WITH RECURSIVE reachable(id) AS (
        SELECT NEW.target_resource_id
        UNION
        SELECT relation.target_resource_id
          FROM resource_relations relation
          JOIN reachable ON reachable.id = relation.source_resource_id
         WHERE relation.id <> COALESCE(NEW.id, '00000000-0000-0000-0000-000000000000')::uuid
    )
    SELECT EXISTS (SELECT 1 FROM reachable WHERE id = NEW.source_resource_id) INTO cycle_found;
    IF cycle_found THEN
        RAISE EXCEPTION 'resource relation would create a cycle' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER resource_relations_validate
BEFORE INSERT OR UPDATE OF source_resource_id, target_resource_id ON resource_relations
FOR EACH ROW EXECUTE FUNCTION validate_resource_relation();

CREATE OR REPLACE FUNCTION validate_resource_scope_move()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    relation record;
    other_scope uuid;
BEGIN
    IF NEW.scope_id = OLD.scope_id THEN
        RETURN NEW;
    END IF;
    FOR relation IN
        SELECT source_resource_id, target_resource_id
          FROM resource_relations
         WHERE source_resource_id = NEW.id OR target_resource_id = NEW.id
    LOOP
        IF relation.source_resource_id = NEW.id THEN
            SELECT scope_id INTO other_scope FROM resources WHERE id = relation.target_resource_id;
            IF NOT resource_scope_contains(other_scope, NEW.scope_id) THEN
                RAISE EXCEPTION 'resource scope move would invalidate relation' USING ERRCODE = '23514';
            END IF;
        ELSE
            SELECT scope_id INTO other_scope FROM resources WHERE id = relation.source_resource_id;
            IF NOT resource_scope_contains(NEW.scope_id, other_scope) THEN
                RAISE EXCEPTION 'resource scope move would invalidate relation' USING ERRCODE = '23514';
            END IF;
        END IF;
    END LOOP;
    RETURN NEW;
END;
$$;

CREATE TRIGGER resources_validate_scope_move
BEFORE UPDATE OF scope_id ON resources
FOR EACH ROW EXECUTE FUNCTION validate_resource_scope_move();

CREATE OR REPLACE FUNCTION validate_scope_default()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    default_scope uuid;
    resource_scope uuid;
    scope_status text;
BEGIN
    SELECT scope_id INTO resource_scope FROM resources WHERE id = NEW.resource_id AND deleted_at IS NULL;
    SELECT id, status INTO default_scope, scope_status FROM scopes WHERE id = NEW.scope_id AND deleted_at IS NULL;
    IF default_scope IS NULL OR scope_status <> 'active' OR resource_scope IS NULL OR NOT resource_scope_contains(resource_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'scope default resource is not visible from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER scope_defaults_validate
BEFORE INSERT OR UPDATE OF scope_id, resource_id ON scope_defaults
FOR EACH ROW EXECUTE FUNCTION validate_scope_default();

INSERT INTO resource_schemas (kind, version, schema)
SELECT kind, 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb
  FROM unnest(ARRAY[
      'KubernetesCluster', 'Namespace', 'Node', 'Workload', 'Pod', 'Service', 'Ingress',
      'BusinessApplication', 'Endpoint', 'CronApplication', 'PostgreSQL', 'Redis', 'Kafka',
      'Elasticsearch', 'GenericMiddleware', 'LLMProvider', 'Model', 'MCPServer', 'Skill',
      'Prometheus', 'Loki', 'Tempo', 'Jaeger', 'Elastic', 'Datadog', 'GenericAPI',
      'Credential', 'NotificationChannel', 'Runbook', 'ArtifactStore'
  ]) AS kinds(kind)
ON CONFLICT (kind, version) DO NOTHING;

CREATE TRIGGER resources_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON resources
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER resource_credentials_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON resource_credentials
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER resource_relations_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON resource_relations
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER scope_defaults_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON scope_defaults
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

-- >>> 0007_resource_catalog_metadata.sql
ALTER TABLE resource_schemas
    ADD COLUMN display_name text NOT NULL DEFAULT '',
    ADD COLUMN description text NOT NULL DEFAULT '',
    ADD COLUMN icon text NOT NULL DEFAULT 'resource';

UPDATE resource_schemas
   SET display_name = metadata.display_name,
       description = metadata.description,
       icon = metadata.icon
  FROM (VALUES
    ('KubernetesCluster', 'Kubernetes 集群', '连接并管理 Kubernetes 集群的入口资源。', 'kubernetes'),
    ('BusinessApplication', '业务应用', '面向业务的应用或服务集合。', 'application'),
    ('Endpoint', '服务端点', '业务应用对外提供的访问端点。', 'endpoint'),
    ('CronApplication', '定时应用', '按计划运行的批处理或定时任务应用。', 'schedule'),
    ('PostgreSQL', 'PostgreSQL 数据库', 'PostgreSQL 数据库连接配置。', 'postgresql'),
    ('Redis', 'Redis', 'Redis 缓存或数据服务连接配置。', 'redis'),
    ('Kafka', 'Kafka 集群', 'Kafka 消息集群连接配置。', 'kafka'),
    ('Elasticsearch', 'Elasticsearch', 'Elasticsearch 搜索服务连接配置。', 'search'),
    ('GenericMiddleware', '通用中间件', '无法归入具体类型的中间件或平台服务。', 'middleware'),
    ('LLMProvider', '大模型服务', 'OpenAI 兼容或其他大模型服务提供方。', 'llm'),
    ('MCPServer', 'MCP 服务', '提供受控工具能力的 MCP 服务。', 'mcp'),
    ('Skill', '诊断技能', '可被受控 Runner 调用的诊断技能。', 'skill'),
    ('Prometheus', 'Prometheus', 'Prometheus 指标服务连接配置。', 'metrics'),
    ('Loki', 'Loki', 'Loki 日志服务连接配置。', 'logs'),
    ('Tempo', 'Tempo', 'Tempo 链路追踪服务连接配置。', 'traces'),
    ('Jaeger', 'Jaeger', 'Jaeger 链路追踪服务连接配置。', 'traces'),
    ('Elastic', 'Elastic Observability', 'Elastic 可观测性服务连接配置。', 'search'),
    ('Datadog', 'Datadog', 'Datadog 可观测性服务连接配置。', 'observability'),
    ('GenericAPI', '通用 API', '可通过 HTTP 访问的外部 API。', 'api'),
    ('NotificationChannel', '通知渠道', 'Webhook、邮件或其他通知目标。', 'notification'),
    ('Runbook', '运行手册', '可供运维流程引用的运行手册。', 'runbook'),
    ('ArtifactStore', '制品存储', '保存诊断报告和其他制品的存储服务。', 'storage'),
    ('Credential', '连接凭据', '已由独立凭据模型管理，不应作为资源登记。', 'credential')
  ) AS metadata(kind, display_name, description, icon)
 WHERE resource_schemas.kind = metadata.kind;

UPDATE resource_schemas
   SET schema = CASE kind
       WHEN 'KubernetesCluster' THEN '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"kubeconfig":{"title":"kubeconfig","type":"string","sensitive":true},"context":{"title":"Context","type":"string"},"api_server":{"title":"API Server","type":"string"}}}'::jsonb
       WHEN 'PostgreSQL' THEN '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string"},"port":{"title":"端口","type":"integer"},"database":{"title":"数据库","type":"string"},"username":{"title":"用户名","type":"string"},"password":{"title":"密码","type":"string","sensitive":true}}}'::jsonb
       WHEN 'Redis' THEN '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string"},"port":{"title":"端口","type":"integer"},"database":{"title":"数据库编号","type":"integer"},"username":{"title":"用户名","type":"string"},"password":{"title":"密码","type":"string","sensitive":true}}}'::jsonb
       WHEN 'Kafka' THEN '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"brokers":{"title":"Broker 地址","type":"array"},"username":{"title":"用户名","type":"string"},"password":{"title":"密码","type":"string","sensitive":true},"tls":{"title":"启用 TLS","type":"boolean"}}}'::jsonb
       WHEN 'LLMProvider' THEN '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"url":{"title":"服务 URL","type":"string"},"model":{"title":"默认模型","type":"string"},"token":{"title":"访问 Token","type":"string","sensitive":true}}}'::jsonb
       ELSE schema
   END
 WHERE kind IN ('KubernetesCluster', 'PostgreSQL', 'Redis', 'Kafka', 'LLMProvider');

UPDATE resource_schemas
   SET status = 'disabled'
 WHERE kind IN ('Namespace', 'Node', 'Workload', 'Pod', 'Service', 'Ingress', 'Model', 'Credential');


-- >>> 0008_organization_icons.sql
ALTER TABLE teams
    ADD COLUMN icon text NOT NULL DEFAULT 'team';

ALTER TABLE projects
    ADD COLUMN icon text NOT NULL DEFAULT 'project';

ALTER TABLE platforms
    ADD COLUMN icon text NOT NULL DEFAULT 'platform';

ALTER TABLE teams
    ADD CONSTRAINT teams_icon_length CHECK (length(icon) BETWEEN 1 AND 64);

ALTER TABLE projects
    ADD CONSTRAINT projects_icon_length CHECK (length(icon) BETWEEN 1 AND 64);

ALTER TABLE platforms
    ADD CONSTRAINT platforms_icon_length CHECK (length(icon) BETWEEN 1 AND 64);

-- >>> 0009_kubernetes_discovery.sql
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

-- >>> 0010_connector_runtime.sql
UPDATE resource_schemas
   SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["url"],"properties":{"url":{"title":"服务 URL","type":"string","format":"uri"},"username":{"title":"用户名","type":"string","sensitive":true},"password":{"title":"密码","type":"string","sensitive":true},"token":{"title":"访问 Token","type":"string","sensitive":true}}}'::jsonb,
       display_name = 'Prometheus',
       description = 'Prometheus 指标查询和告警来源。',
       icon = 'metrics'
 WHERE kind = 'Prometheus';

UPDATE resource_schemas
   SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["url"],"properties":{"url":{"title":"服务 URL","type":"string","format":"uri"},"tenant_id":{"title":"租户 ID","type":"string"},"username":{"title":"用户名","type":"string","sensitive":true},"password":{"title":"密码","type":"string","sensitive":true},"token":{"title":"访问 Token","type":"string","sensitive":true}}}'::jsonb,
       display_name = 'Loki',
       description = 'Loki 日志查询来源。',
       icon = 'logs'
 WHERE kind = 'Loki';

CREATE TABLE resource_connection_checks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('succeeded', 'failed')),
    error_category text NOT NULL DEFAULT '' CHECK (
        error_category IN ('', 'configuration', 'authentication', 'timeout', 'rate_limited', 'response_too_large', 'upstream', 'unsupported', 'internal')
    ),
    message text NOT NULL DEFAULT '' CHECK (length(message) <= 500),
    latency_ms bigint NOT NULL DEFAULT 0 CHECK (latency_ms >= 0),
    capabilities text[] NOT NULL DEFAULT '{}',
    checked_by uuid REFERENCES users(id) ON DELETE SET NULL,
    checked_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX resource_connection_checks_latest_idx
    ON resource_connection_checks(resource_id, checked_at DESC, id DESC);

-- >>> 0011_llm_skill_runner.sql
CREATE TABLE llm_scope_defaults (
    scope_id uuid PRIMARY KEY REFERENCES scopes(id) ON DELETE CASCADE,
    provider_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    model_name text NOT NULL CHECK (length(btrim(model_name)) BETWEEN 1 AND 200),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE skill_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    skill_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    version integer NOT NULL CHECK (version > 0),
    manifest jsonb NOT NULL CHECK (jsonb_typeof(manifest) = 'object'),
    input_schema jsonb NOT NULL CHECK (jsonb_typeof(input_schema) = 'object'),
    output_schema jsonb NOT NULL CHECK (jsonb_typeof(output_schema) = 'object'),
    tools jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(tools) = 'array'),
    risk_level text NOT NULL CHECK (risk_level IN ('read_only', 'controlled', 'high')),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'disabled')),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    UNIQUE (skill_resource_id, version),
    CHECK ((status = 'published' AND published_at IS NOT NULL) OR status <> 'published')
);

CREATE INDEX skill_versions_resource_idx
    ON skill_versions(skill_resource_id, version DESC);

CREATE TABLE skill_scope_defaults (
    scope_id uuid PRIMARY KEY REFERENCES scopes(id) ON DELETE CASCADE,
    skill_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    skill_version_id uuid NOT NULL REFERENCES skill_versions(id) ON DELETE RESTRICT,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE skill_executions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id) ON DELETE RESTRICT,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    target_resource_id uuid REFERENCES resources(id) ON DELETE RESTRICT,
    skill_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    skill_version_id uuid NOT NULL REFERENCES skill_versions(id) ON DELETE RESTRICT,
    provider_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    model_name text NOT NULL CHECK (length(btrim(model_name)) BETWEEN 1 AND 200),
    status text NOT NULL CHECK (status IN ('running', 'succeeded', 'failed', 'cancelled')),
    input_digest text NOT NULL DEFAULT '' CHECK (length(input_digest) <= 128),
    output_preview text NOT NULL DEFAULT '' CHECK (length(output_preview) <= 4000),
    prompt_tokens bigint NOT NULL DEFAULT 0 CHECK (prompt_tokens >= 0),
    completion_tokens bigint NOT NULL DEFAULT 0 CHECK (completion_tokens >= 0),
    total_tokens bigint NOT NULL DEFAULT 0 CHECK (total_tokens >= 0),
    tool_call_count integer NOT NULL DEFAULT 0 CHECK (tool_call_count >= 0),
    error_code text NOT NULL DEFAULT '' CHECK (length(error_code) <= 120),
    error_message text NOT NULL DEFAULT '' CHECK (length(error_message) <= 1000),
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX skill_executions_scope_idx ON skill_executions(scope_id, created_at DESC);
CREATE INDEX skill_executions_actor_idx ON skill_executions(actor_user_id, created_at DESC);
CREATE INDEX skill_executions_target_idx ON skill_executions(target_resource_id, created_at DESC);

CREATE TABLE skill_tool_calls (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id uuid NOT NULL REFERENCES skill_executions(id) ON DELETE CASCADE,
    sequence integer NOT NULL CHECK (sequence > 0),
    tool_name text NOT NULL CHECK (length(btrim(tool_name)) BETWEEN 1 AND 120),
    target_resource_id uuid REFERENCES resources(id) ON DELETE RESTRICT,
    status text NOT NULL CHECK (status IN ('running', 'succeeded', 'failed', 'rejected')),
    input_digest text NOT NULL DEFAULT '' CHECK (length(input_digest) <= 128),
    output_preview text NOT NULL DEFAULT '' CHECK (length(output_preview) <= 2000),
    error_code text NOT NULL DEFAULT '' CHECK (length(error_code) <= 120),
    error_message text NOT NULL DEFAULT '' CHECK (length(error_message) <= 1000),
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    UNIQUE (execution_id, sequence)
);

CREATE OR REPLACE FUNCTION validate_llm_scope_default()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    provider_scope uuid;
    provider_kind text;
    provider_status text;
BEGIN
    SELECT scope_id, kind, status INTO provider_scope, provider_kind, provider_status
      FROM resources
     WHERE id = NEW.provider_resource_id AND deleted_at IS NULL;
    IF provider_kind IS DISTINCT FROM 'LLMProvider' OR provider_status IS DISTINCT FROM 'active'
       OR NOT resource_scope_contains(provider_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'default LLM provider is unavailable from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER llm_scope_defaults_validate
BEFORE INSERT OR UPDATE OF scope_id, provider_resource_id ON llm_scope_defaults
FOR EACH ROW EXECUTE FUNCTION validate_llm_scope_default();

CREATE OR REPLACE FUNCTION validate_skill_version()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    resource_kind text;
BEGIN
    SELECT kind INTO resource_kind FROM resources
     WHERE id = NEW.skill_resource_id AND deleted_at IS NULL;
    IF resource_kind IS DISTINCT FROM 'Skill' THEN
        RAISE EXCEPTION 'skill version must belong to a Skill resource' USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'UPDATE' AND (
        NEW.skill_resource_id <> OLD.skill_resource_id OR NEW.version <> OLD.version
        OR NEW.manifest <> OLD.manifest OR NEW.input_schema <> OLD.input_schema
        OR NEW.output_schema <> OLD.output_schema OR NEW.tools <> OLD.tools
        OR NEW.risk_level <> OLD.risk_level
    ) THEN
        RAISE EXCEPTION 'skill version content is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER skill_versions_validate
BEFORE INSERT OR UPDATE ON skill_versions
FOR EACH ROW EXECUTE FUNCTION validate_skill_version();

CREATE OR REPLACE FUNCTION validate_skill_scope_default()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    skill_scope uuid;
    skill_kind text;
    skill_status text;
    version_resource uuid;
    version_status text;
BEGIN
    SELECT scope_id, kind, status INTO skill_scope, skill_kind, skill_status
      FROM resources WHERE id = NEW.skill_resource_id AND deleted_at IS NULL;
    SELECT skill_resource_id, status INTO version_resource, version_status
      FROM skill_versions WHERE id = NEW.skill_version_id;
    IF skill_kind IS DISTINCT FROM 'Skill' OR skill_status IS DISTINCT FROM 'active'
       OR version_resource IS DISTINCT FROM NEW.skill_resource_id
       OR version_status IS DISTINCT FROM 'published'
       OR NOT resource_scope_contains(skill_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'default Skill version is unavailable from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER skill_scope_defaults_validate
BEFORE INSERT OR UPDATE ON skill_scope_defaults
FOR EACH ROW EXECUTE FUNCTION validate_skill_scope_default();

INSERT INTO resource_schemas (kind, version, schema, display_name, description, icon)
VALUES
('LLMProvider', 2,
 '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["provider_type","base_url","models"],"properties":{"provider_type":{"title":"提供方类型","type":"string","enum":["openai_compatible","openai"]},"base_url":{"title":"服务 URL","type":"string","format":"uri"},"models":{"title":"模型列表","type":"array","items":{"type":"object","required":["name","context_window"],"properties":{"name":{"type":"string"},"context_window":{"type":"integer"},"input_price_per_million":{"type":"number"},"output_price_per_million":{"type":"number"},"capabilities":{"type":"array","items":{"type":"string"}}}}},"token":{"title":"API Token","type":"string","sensitive":true},"timeout_seconds":{"title":"超时秒数","type":"integer"}}}'::jsonb,
 '大模型服务', 'OpenAI-compatible 或 OpenAI Responses API 模型提供方；访问 Token 使用独立加密凭据。', 'llm'),
('Skill', 2,
 '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"summary":{"title":"用途说明","type":"string"},"owner":{"title":"维护者","type":"string"}}}'::jsonb,
 '诊断技能', '声明式、版本化并由受控 ADK Runner 执行的技能。', 'skill'),
('MCPServer', 2,
 '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["transport"],"properties":{"transport":{"title":"传输方式","type":"string","enum":["stdio","streamable_http"]},"url":{"title":"服务 URL","type":"string"},"command":{"title":"启动命令","type":"string"}}}'::jsonb,
 'MCP 服务', '提供受控工具能力的 MCP 服务；运行时接入在 T14 实施。', 'mcp')
ON CONFLICT (kind, version) DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT roles.id, permissions.permission
  FROM roles
  CROSS JOIN (VALUES ('skill:manage'), ('skill:execute')) AS permissions(permission)
 WHERE roles.name IN ('PlatformAdmin', 'TeamAdmin', 'ProjectAdmin')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT roles.id, 'skill:execute'
  FROM roles
 WHERE roles.name IN ('PlatformOperator', 'TeamOperator', 'ProjectOperator')
ON CONFLICT DO NOTHING;

CREATE TRIGGER llm_scope_defaults_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON llm_scope_defaults
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER skill_versions_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON skill_versions
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

CREATE TRIGGER skill_scope_defaults_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON skill_scope_defaults
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();

-- >>> 0012_diagnosis_workbench.sql
CREATE TABLE diagnosis_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id) ON DELETE RESTRICT,
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    status text NOT NULL CHECK (status IN ('queued', 'planning', 'collecting', 'analyzing', 'succeeded', 'failed', 'cancelled')),
    title text NOT NULL DEFAULT '' CHECK (length(title) <= 200),
    error_code text NOT NULL DEFAULT '' CHECK (length(error_code) <= 120),
    error_message text NOT NULL DEFAULT '' CHECK (length(error_message) <= 1000),
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX diagnosis_sessions_scope_idx ON diagnosis_sessions(scope_id, created_at DESC);
CREATE INDEX diagnosis_sessions_actor_idx ON diagnosis_sessions(actor_user_id, created_at DESC);

CREATE TABLE diagnosis_targets (
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, resource_id)
);

CREATE TABLE diagnosis_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content text NOT NULL CHECK (length(content) <= 16000),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX diagnosis_messages_session_idx ON diagnosis_messages(session_id, created_at, id);

CREATE TABLE diagnosis_plans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL UNIQUE REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    summary text NOT NULL DEFAULT '' CHECK (length(summary) <= 2000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE diagnosis_plan_steps (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id uuid NOT NULL REFERENCES diagnosis_plans(id) ON DELETE CASCADE,
    sequence integer NOT NULL CHECK (sequence > 0),
    phase text NOT NULL CHECK (phase IN ('plan', 'collect', 'verify', 'summarize')),
    status text NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'skipped')),
    title text NOT NULL CHECK (length(title) BETWEEN 1 AND 300),
    detail text NOT NULL DEFAULT '' CHECK (length(detail) <= 2000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (plan_id, sequence)
);

CREATE TABLE diagnosis_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    event_type text NOT NULL CHECK (length(event_type) BETWEEN 1 AND 120),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(payload) = 'object'),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX diagnosis_events_session_idx ON diagnosis_events(session_id, id);

CREATE TABLE diagnosis_evidence (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    target_resource_id uuid REFERENCES resources(id) ON DELETE RESTRICT,
    source_resource_id uuid REFERENCES resources(id) ON DELETE RESTRICT,
    capability text NOT NULL DEFAULT '' CHECK (length(capability) <= 120),
    collected_at timestamptz NOT NULL,
    window_start timestamptz,
    window_end timestamptz,
    content_hash text NOT NULL CHECK (length(content_hash) = 64),
    summary jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(summary) = 'object'),
    content jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(content) IN ('object', 'array')),
    partial boolean NOT NULL DEFAULT false,
    untrusted boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((window_start IS NULL AND window_end IS NULL) OR (window_start IS NOT NULL AND window_end IS NOT NULL AND window_start <= window_end))
);

CREATE INDEX diagnosis_evidence_session_idx ON diagnosis_evidence(session_id, created_at, id);

CREATE TABLE diagnosis_hypotheses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    statement text NOT NULL CHECK (length(statement) BETWEEN 1 AND 4000),
    status text NOT NULL CHECK (status IN ('pending', 'supported', 'rejected', 'needs_verification')),
    confidence numeric(4,3) NOT NULL DEFAULT 0 CHECK (confidence >= 0 AND confidence <= 1),
    evidence_ids uuid[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX diagnosis_hypotheses_session_idx ON diagnosis_hypotheses(session_id, created_at, id);

CREATE TABLE diagnosis_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL UNIQUE REFERENCES diagnosis_sessions(id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('succeeded', 'warning', 'failed')),
    conclusion text NOT NULL DEFAULT '' CHECK (length(conclusion) <= 8000),
    recommendations jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(recommendations) = 'array'),
    evidence_ids uuid[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- >>> 0013_builtin_skills.sql
INSERT INTO resources (scope_id, kind, schema_version, name, config, status)
SELECT platform.scope_id, 'Skill', 2, item.name, jsonb_build_object('summary', item.summary, 'owner', 'OpsKeeper builtin'), 'active'
  FROM platforms platform
 CROSS JOIN (VALUES
    ('Kubernetes 工作负载诊断', 'Kubernetes 工作负载、Pod、事件、探针和发布状态的只读诊断。'),
    ('PostgreSQL 健康诊断', 'PostgreSQL 连接、会话、慢查询、锁、复制和容量的只读诊断。'),
    ('Redis 健康诊断', 'Redis 内存、热 Key 能力、慢命令、客户端和复制的只读诊断。'),
    ('Kafka 健康诊断', 'Kafka Broker、Topic、分区、ISR、消费组和积压的只读诊断。')
 ) AS item(name, summary)
 WHERE platform.code = 'default'
ON CONFLICT (scope_id, kind, lower(name)) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO skill_versions (skill_resource_id, version, manifest, input_schema, output_schema, tools, risk_level, status, published_at)
SELECT resource.id, 1,
       jsonb_build_object('name', resource.name, 'description', resource.config->>'summary', 'instruction', item.instruction, 'target_kinds', item.target_kinds::jsonb),
	       '{"type":"object","required":["facts","findings","evidence","hypotheses","confidence","recommendations"],"properties":{"facts":{"type":"object"},"findings":{"type":"array"},"evidence":{"type":"array"},"hypotheses":{"type":"array"},"confidence":{"type":"number","minimum":0,"maximum":1},"recommendations":{"type":"array"}},"additionalProperties":false}'::jsonb,
       '{"type":"object","additionalProperties":true}'::jsonb,
       item.tools::jsonb, 'read_only', 'published', now()
  FROM resources resource
  JOIN (VALUES
	    ('Kubernetes 工作负载诊断', '仅调用声明的 Kubernetes 只读工具。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations；将事实、异常和待验证假设分开。', '["Application","Kubernetes"]', '[{"name":"connector_kubernetes_read","description":"受限读取 Kubernetes 对象。","input_schema":{"type":"object","required":["target_resource_id","resource"],"properties":{"target_resource_id":{"type":"string"},"resource":{"type":"string"},"namespace":{"type":"string"},"name":{"type":"string"},"label_selector":{"type":"string"},"limit":{"type":"integer"}},"additionalProperties":false}}]'),
	    ('PostgreSQL 健康诊断', '只调用 PostgreSQL 固定诊断快照；不得请求写 SQL。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。', '["PostgreSQL"]', '[{"name":"connector_postgresql_inspect","description":"读取固定 PostgreSQL 健康快照。","input_schema":{"type":"object","required":["target_resource_id"],"properties":{"target_resource_id":{"type":"string"}},"additionalProperties":false}}]'),
	    ('Redis 健康诊断', '只调用 Redis 固定诊断快照；不得请求写命令。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。', '["Redis"]', '[{"name":"connector_redis_inspect","description":"读取固定 Redis 健康快照。","input_schema":{"type":"object","required":["target_resource_id"],"properties":{"target_resource_id":{"type":"string"}},"additionalProperties":false}}]'),
	    ('Kafka 健康诊断', '只调用 Kafka 固定诊断快照；不得请求管理操作。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。', '["Kafka"]', '[{"name":"connector_kafka_inspect","description":"读取固定 Kafka 健康快照。","input_schema":{"type":"object","required":["target_resource_id"],"properties":{"target_resource_id":{"type":"string"}},"additionalProperties":false}}]')
  ) AS item(name, instruction, target_kinds, tools) ON item.name = resource.name
 WHERE resource.kind = 'Skill' AND resource.config->>'owner' = 'OpsKeeper builtin'
ON CONFLICT (skill_resource_id, version) DO NOTHING;

-- >>> 0014_builtin_skill_output_contract.sql
INSERT INTO skill_versions (skill_resource_id, version, manifest, input_schema, output_schema, tools, risk_level, status, published_at)
SELECT version.skill_resource_id, 2, version.manifest, version.input_schema,
       '{"type":"object","required":["facts","findings","evidence","hypotheses","confidence","recommendations"],"properties":{"facts":{"type":"object"},"findings":{"type":"array"},"evidence":{"type":"array"},"hypotheses":{"type":"array"},"confidence":{"type":"number","minimum":0,"maximum":1},"recommendations":{"type":"array"}},"additionalProperties":false}'::jsonb,
       version.tools, version.risk_level, 'published', now()
  FROM skill_versions AS version
  JOIN resources AS resource ON resource.id = version.skill_resource_id
 WHERE version.version = 1
   AND resource.kind = 'Skill'
   AND resource.config->>'owner' = 'OpsKeeper builtin'
   AND resource.name IN ('Kubernetes 工作负载诊断', 'PostgreSQL 健康诊断', 'Redis 健康诊断', 'Kafka 健康诊断')
ON CONFLICT (skill_resource_id, version) DO NOTHING;

UPDATE skill_versions AS version
   SET status = 'disabled'
  FROM resources AS resource
 WHERE version.skill_resource_id = resource.id
   AND version.version = 1
   AND resource.kind = 'Skill'
   AND resource.config->>'owner' = 'OpsKeeper builtin';

-- >>> 0015_inspection_notification.sql
CREATE TABLE inspection_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    cron text NOT NULL CHECK (length(btrim(cron)) BETWEEN 1 AND 200),
    timezone text NOT NULL DEFAULT 'UTC' CHECK (length(btrim(timezone)) BETWEEN 1 AND 100),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    target_labels jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(target_labels) = 'object'),
    skill_resource_ids uuid[] NOT NULL DEFAULT '{}',
    timeout_seconds integer NOT NULL DEFAULT 120 CHECK (timeout_seconds BETWEEN 1 AND 3600),
    retries integer NOT NULL DEFAULT 1 CHECK (retries BETWEEN 0 AND 10),
    max_concurrent integer NOT NULL DEFAULT 1 CHECK (max_concurrent BETWEEN 1 AND 64),
    max_tool_calls integer NOT NULL DEFAULT 12 CHECK (max_tool_calls BETWEEN 1 AND 100),
    max_tokens bigint NOT NULL DEFAULT 20000 CHECK (max_tokens BETWEEN 1 AND 200000),
    maintenance_windows jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(maintenance_windows) = 'array'),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX inspection_policies_scope_idx ON inspection_policies(scope_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX inspection_policies_scope_name_unique ON inspection_policies(scope_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE inspection_policy_targets (
    policy_id uuid NOT NULL REFERENCES inspection_policies(id) ON DELETE CASCADE,
    resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    PRIMARY KEY (policy_id, resource_id)
);

CREATE TABLE inspection_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id uuid NOT NULL REFERENCES inspection_policies(id) ON DELETE RESTRICT,
    scope_id uuid NOT NULL REFERENCES scopes(id),
    window_start timestamptz NOT NULL,
    window_end timestamptz NOT NULL,
    trigger text NOT NULL CHECK (trigger IN ('schedule', 'manual', 'retry')),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'skipped')),
    policy_snapshot jsonb NOT NULL CHECK (jsonb_typeof(policy_snapshot) = 'object'),
    target_snapshot jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(target_snapshot) = 'array'),
    score integer CHECK (score BETWEEN 0 AND 100),
    deterministic_completed boolean NOT NULL DEFAULT false,
    llm_status text NOT NULL DEFAULT 'not_requested' CHECK (llm_status IN ('not_requested', 'succeeded', 'degraded', 'failed')),
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (policy_id, window_start)
);
CREATE INDEX inspection_runs_scope_created_idx ON inspection_runs(scope_id, created_at DESC);
CREATE INDEX inspection_runs_policy_window_idx ON inspection_runs(policy_id, window_start DESC);

CREATE TABLE inspection_run_steps (
    id bigserial PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES inspection_runs(id) ON DELETE CASCADE,
    sequence integer NOT NULL CHECK (sequence > 0),
    kind text NOT NULL CHECK (length(btrim(kind)) BETWEEN 1 AND 120),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'skipped')),
    detail text NOT NULL DEFAULT '',
    started_at timestamptz,
    completed_at timestamptz,
    UNIQUE (run_id, sequence)
);

CREATE TABLE inspection_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id uuid NOT NULL UNIQUE REFERENCES inspection_runs(id) ON DELETE CASCADE,
    idempotency_key text NOT NULL UNIQUE CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 200),
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'leased', 'succeeded', 'failed')),
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    max_attempts integer NOT NULL DEFAULT 2 CHECK (max_attempts BETWEEN 1 AND 11),
    lease_owner text NOT NULL DEFAULT '',
    lease_expires_at timestamptz,
    heartbeat_at timestamptz,
    available_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inspection_jobs_claim_idx ON inspection_jobs(status, available_at) WHERE status IN ('queued', 'leased');

CREATE TABLE inspection_findings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id uuid NOT NULL REFERENCES inspection_policies(id) ON DELETE RESTRICT,
    target_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    rule text NOT NULL CHECK (length(btrim(rule)) BETWEEN 1 AND 200),
    -- identity_key represents the enduring condition. fingerprint represents
    -- the observation in one scheduling window and is used for event dedupe.
    identity_key text NOT NULL CHECK (length(identity_key) = 64),
    fingerprint text NOT NULL CHECK (length(fingerprint) = 64),
    severity text NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    message text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved')),
    first_observed_at timestamptz NOT NULL,
    last_observed_at timestamptz NOT NULL,
    resolved_at timestamptz,
    last_run_id uuid REFERENCES inspection_runs(id) ON DELETE SET NULL,
    UNIQUE (policy_id, identity_key)
);
CREATE INDEX inspection_findings_scope_idx ON inspection_findings(policy_id, status, last_observed_at DESC);

CREATE TABLE inspection_health_snapshots (
    id bigserial PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES inspection_runs(id) ON DELETE CASCADE,
    policy_id uuid NOT NULL REFERENCES inspection_policies(id) ON DELETE RESTRICT,
    target_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    score integer NOT NULL CHECK (score BETWEEN 0 AND 100),
    reasons jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(reasons) = 'array'),
    collected_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inspection_health_snapshots_target_idx ON inspection_health_snapshots(target_resource_id, collected_at DESC);

CREATE TABLE notification_channels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
    kind text NOT NULL CHECK (kind = 'webhook'),
    webhook_url text NOT NULL CHECK (length(btrim(webhook_url)) BETWEEN 1 AND 2000),
    credential_id uuid REFERENCES resource_credentials(id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    rate_limit_per_minute integer NOT NULL DEFAULT 30 CHECK (rate_limit_per_minute BETWEEN 1 AND 600),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE UNIQUE INDEX notification_channels_scope_name_unique ON notification_channels(scope_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE notification_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id uuid NOT NULL REFERENCES notification_channels(id) ON DELETE RESTRICT,
    finding_id uuid REFERENCES inspection_findings(id) ON DELETE SET NULL,
    run_id uuid REFERENCES inspection_runs(id) ON DELETE SET NULL,
    idempotency_key text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'delivering', 'succeeded', 'failed')),
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    available_at timestamptz NOT NULL DEFAULT now(),
    response_status integer,
    response_body text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX notification_deliveries_claim_idx ON notification_deliveries(status, available_at) WHERE status IN ('queued', 'delivering');

-- >>> 0016_mcp_operations.sql
CREATE TABLE mcp_server_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    scope_id uuid NOT NULL REFERENCES scopes(id),
    protocol_version text NOT NULL DEFAULT '',
    server_name text NOT NULL DEFAULT '',
    server_version text NOT NULL DEFAULT '',
    tools jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(tools) = 'array'),
    content_hash text NOT NULL CHECK (length(content_hash) = 64),
    status text NOT NULL CHECK (status IN ('succeeded', 'failed')),
    error_message text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX mcp_server_snapshots_server_created_idx ON mcp_server_snapshots(server_resource_id, created_at DESC);

CREATE TABLE operation_policies (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 160),
    target_kinds text[] NOT NULL DEFAULT '{}',
    operation_names text[] NOT NULL DEFAULT '{}',
    minimum_risk text NOT NULL CHECK (minimum_risk IN ('read_only', 'low', 'medium', 'high')),
    approval_required boolean NOT NULL DEFAULT true,
    approver_permission text NOT NULL DEFAULT 'operation:approve',
    expires_after_seconds integer NOT NULL DEFAULT 1800 CHECK (expires_after_seconds BETWEEN 60 AND 86400),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (scope_id, name)
);

CREATE TABLE operation_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_id uuid NOT NULL REFERENCES scopes(id),
    target_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
    requested_by uuid NOT NULL REFERENCES users(id),
    source text NOT NULL CHECK (source IN ('user', 'skill', 'mcp')),
    operation_name text NOT NULL CHECK (length(btrim(operation_name)) BETWEEN 1 AND 200),
    risk_level text NOT NULL CHECK (risk_level IN ('read_only', 'low', 'medium', 'high')),
    parameters jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(parameters) = 'object'),
    parameters_hash text NOT NULL CHECK (length(parameters_hash) = 64),
    impact_summary text NOT NULL DEFAULT '',
    rollback_summary text NOT NULL DEFAULT '',
    dry_run jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(dry_run) = 'object'),
    idempotency_key text NOT NULL CHECK (length(btrim(idempotency_key)) BETWEEN 1 AND 250),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'expired', 'executing', 'succeeded', 'failed', 'cancelled')),
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (scope_id, idempotency_key)
);
CREATE INDEX operation_requests_scope_status_idx ON operation_requests(scope_id, status, created_at DESC);

CREATE TABLE operation_approvals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_request_id uuid NOT NULL REFERENCES operation_requests(id) ON DELETE CASCADE,
    approver_user_id uuid NOT NULL REFERENCES users(id),
    decision text NOT NULL CHECK (decision IN ('approved', 'rejected')),
    parameters_hash text NOT NULL CHECK (length(parameters_hash) = 64),
    comment text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (operation_request_id, approver_user_id)
);

CREATE TABLE operation_executions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_request_id uuid NOT NULL UNIQUE REFERENCES operation_requests(id) ON DELETE RESTRICT,
    executor text NOT NULL CHECK (executor IN ('kubernetes_job', 'mcp')),
    idempotency_key text NOT NULL UNIQUE,
    status text NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    result jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(result) = 'object'),
    error_message text NOT NULL DEFAULT '',
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO role_permissions (role_id, permission)
SELECT id, 'operation:approve' FROM roles
 WHERE name IN ('PlatformAdmin', 'TeamAdmin', 'ProjectAdmin')
ON CONFLICT DO NOTHING;

-- >>> 0017_mcp_server_schema.sql
INSERT INTO resource_schemas (kind, version, schema, display_name, description, icon)
VALUES
('MCPServer', 3,
 '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["transport","url","tool_allowlist"],"properties":{"transport":{"title":"传输方式","type":"string","enum":["streamable_http"]},"url":{"title":"服务 URL","type":"string","format":"uri","description":"仅允许 HTTPS 公网地址。"},"tool_allowlist":{"title":"允许的工具","type":"array","minItems":1,"items":{"type":"string"}},"timeout_seconds":{"title":"连接和调用超时（秒）","type":"integer","minimum":1,"maximum":60},"max_response_bytes":{"title":"最大响应字节数","type":"integer","minimum":1,"maximum":1048576}}}'::jsonb,
 'MCP 服务', '仅通过 HTTPS Streamable HTTP 接入；所有工具、响应和描述均为不可信外部内容。', 'mcp')
ON CONFLICT (kind, version) DO NOTHING;

-- >>> 0018_audit_retention.sql
CREATE TABLE audit_retention_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cutoff timestamptz NOT NULL,
    export_reference text NOT NULL CHECK (length(btrim(export_reference)) BETWEEN 1 AND 512),
    requested_by text NOT NULL CHECK (length(btrim(requested_by)) BETWEEN 1 AND 255),
    deleted_count bigint NOT NULL CHECK (deleted_count >= 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION protect_audit_rows()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_setting('opsk.audit_retention', true) = 'enabled' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION '% is not allowed on append-only table %', TG_OP, TG_TABLE_NAME;
END;
$$;

CREATE TRIGGER audit_events_append_only
BEFORE UPDATE OR DELETE ON audit_events
FOR EACH ROW EXECUTE FUNCTION protect_audit_rows();

CREATE TRIGGER audit_retention_runs_append_only
BEFORE UPDATE OR DELETE ON audit_retention_runs
FOR EACH ROW EXECUTE FUNCTION protect_audit_rows();

CREATE OR REPLACE FUNCTION prune_audit_events(
    p_cutoff timestamptz,
    p_export_reference text,
    p_requested_by text
)
RETURNS bigint
LANGUAGE plpgsql
AS $$
DECLARE
    removed bigint;
BEGIN
    IF p_cutoff IS NULL OR p_cutoff >= now() - interval '30 days' THEN
        RAISE EXCEPTION 'audit cutoff must be at least 30 days old';
    END IF;
    IF length(btrim(COALESCE(p_export_reference, ''))) NOT BETWEEN 1 AND 512 THEN
        RAISE EXCEPTION 'export reference is required';
    END IF;
    IF length(btrim(COALESCE(p_requested_by, ''))) NOT BETWEEN 1 AND 255 THEN
        RAISE EXCEPTION 'requested by is required';
    END IF;

    PERFORM pg_advisory_xact_lock(5715441255318348116);
    PERFORM set_config('opsk.audit_retention', 'enabled', true);
    DELETE FROM audit_events WHERE created_at < p_cutoff;
    GET DIAGNOSTICS removed = ROW_COUNT;
    INSERT INTO audit_retention_runs (cutoff, export_reference, requested_by, deleted_count)
    VALUES (p_cutoff, btrim(p_export_reference), btrim(p_requested_by), removed);
    RETURN removed;
END;
$$;

COMMENT ON FUNCTION prune_audit_events(timestamptz, text, text) IS
'Delete exported audit events older than the retention cutoff and append an immutable retention record.';

REVOKE ALL ON FUNCTION prune_audit_events(timestamptz, text, text) FROM PUBLIC;

-- >>> 0019_usernames.sql
ALTER TABLE users
    ADD COLUMN username text,
    ADD COLUMN phone text;

UPDATE users
   SET username = 'user-' || left(replace(id::text, '-', ''), 16)
 WHERE username IS NULL;

ALTER TABLE users
    ALTER COLUMN email DROP NOT NULL,
    ALTER COLUMN username SET DEFAULT ('user-' || left(replace(gen_random_uuid()::text, '-', ''), 16)),
    ALTER COLUMN username SET NOT NULL,
    ADD CONSTRAINT users_username_format CHECK (username ~ '^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$');

UPDATE users SET email = NULL WHERE btrim(email) = '';

ALTER TABLE users
    ADD CONSTRAINT users_email_nonempty CHECK (email IS NULL OR length(btrim(email)) > 0),
    ADD CONSTRAINT users_phone_format CHECK (phone IS NULL OR phone ~ '^\+?[0-9]{3,32}$');

CREATE UNIQUE INDEX users_username_unique ON users (lower(username)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_phone_unique ON users (phone) WHERE deleted_at IS NULL AND phone IS NOT NULL AND phone <> '';
