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
