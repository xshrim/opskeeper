-- Dedicated Engine catalog and scope-aware default tags for the LLM workspace.
-- Provider metadata remains in providers; provider_scope_bindings is the
-- effective per-scope default mapping and therefore supports inheritance.

ALTER TABLE provider_scope_bindings DROP CONSTRAINT IF EXISTS provider_scope_bindings_tag_check;
UPDATE provider_scope_bindings SET tag = 'general' WHERE tag = 'default';
ALTER TABLE provider_scope_bindings
  ADD CONSTRAINT provider_scope_bindings_tag_check CHECK (tag IN ('general', 'diagnosis', 'inspection', 'workflow'));

INSERT INTO role_permissions (role_id, permission)
SELECT roles.id, permissions.permission
  FROM roles
  JOIN (VALUES
    ('PlatformAdmin', 'engine:read'), ('PlatformAdmin', 'engine:manage'), ('PlatformAdmin', 'engine:use'),
    ('PlatformOperator', 'engine:read'), ('PlatformOperator', 'engine:use'),
    ('PlatformViewer', 'engine:read'),
    ('TeamAdmin', 'engine:read'), ('TeamAdmin', 'engine:manage'), ('TeamAdmin', 'engine:use'),
    ('TeamOperator', 'engine:read'), ('TeamOperator', 'engine:use'),
    ('TeamViewer', 'engine:read'),
    ('ProjectAdmin', 'engine:read'), ('ProjectAdmin', 'engine:manage'), ('ProjectAdmin', 'engine:use'),
    ('ProjectOperator', 'engine:read'), ('ProjectOperator', 'engine:use'),
    ('ProjectViewer', 'engine:read')
  ) AS permissions(role_name, permission) ON permissions.role_name = roles.name
ON CONFLICT DO NOTHING;

CREATE TABLE engines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id text NOT NULL DEFAULT 'default',
    scope_id uuid NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    description text NOT NULL DEFAULT '',
    icon text NOT NULL DEFAULT 'Cpu',
    config jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(config) = 'object'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'unknown')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX engines_scope_idx ON engines(scope_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX engines_scope_name_unique ON engines(scope_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE engine_scope_bindings (
    scope_id uuid NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    engine_id uuid NOT NULL REFERENCES engines(id) ON DELETE CASCADE,
    tag text NOT NULL CHECK (tag IN ('general', 'diagnosis', 'inspection', 'workflow')),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_id, tag)
);
CREATE INDEX engine_scope_bindings_engine_idx ON engine_scope_bindings(engine_id);

-- The built-in runtime is available at every platform scope. Future engines
-- can be added without changing the execution contract.
INSERT INTO engines (tenant_id, scope_id, name, description, icon, config)
SELECT s.tenant_id, s.id, 'OpsKeeper Engine',
       '统一承载诊断、巡检、技能和工作流的智能执行引擎。', 'Cpu',
       jsonb_build_object('capabilities', jsonb_build_array('诊断编排', '工具调用', '结构化输出', '流式响应'))
  FROM scopes s
 WHERE s.scope_type = 'platform' AND s.deleted_at IS NULL
   AND NOT EXISTS (SELECT 1 FROM engines e WHERE e.scope_id = s.id AND e.deleted_at IS NULL);
INSERT INTO engine_scope_bindings (scope_id, engine_id, tag)
SELECT e.scope_id, e.id, tags.tag
  FROM engines e
 CROSS JOIN unnest(ARRAY['general', 'diagnosis', 'inspection', 'workflow']) AS tags(tag)
 WHERE e.name = 'OpsKeeper Engine'
ON CONFLICT (scope_id, tag) DO NOTHING;

CREATE OR REPLACE FUNCTION validate_engine_scope_binding()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE engine_scope uuid; engine_status text;
BEGIN
    SELECT scope_id, status INTO engine_scope, engine_status
      FROM engines WHERE id = NEW.engine_id AND deleted_at IS NULL;
    IF engine_scope IS NULL OR engine_status <> 'active' OR NOT resource_scope_contains(engine_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'Engine is unavailable from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER engine_scope_bindings_validate
BEFORE INSERT OR UPDATE OF scope_id, engine_id ON engine_scope_bindings
FOR EACH ROW EXECUTE FUNCTION validate_engine_scope_binding();
CREATE TRIGGER engine_scope_bindings_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON engine_scope_bindings
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();
CREATE TRIGGER engines_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON engines
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();
