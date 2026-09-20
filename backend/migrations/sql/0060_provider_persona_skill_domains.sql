-- Separate LLM providers, personas and skills from the generic resource catalog.
-- Existing UUIDs are deliberately retained so historical executions and links
-- remain addressable during the transition. Resource rows are kept for the
-- compatibility resource view until the dedicated pages are fully deployed.

CREATE TABLE providers (
    id uuid PRIMARY KEY,
    tenant_id text NOT NULL DEFAULT 'default',
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    config jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(config) = 'object'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'unknown')),
    credential_ciphertext bytea,
    credential_key_version text NOT NULL DEFAULT '',
    credential_purpose text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX providers_scope_idx ON providers(scope_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX providers_scope_name_unique ON providers(scope_id, lower(name)) WHERE deleted_at IS NULL;

INSERT INTO providers (id, tenant_id, scope_id, name, config, status, credential_ciphertext, credential_key_version, credential_purpose, created_at, updated_at, deleted_at)
SELECT id, tenant_id, scope_id, name, config, status, credential_ciphertext, credential_key_version, credential_purpose, created_at, updated_at, deleted_at
  FROM resources WHERE kind = 'AIProvider'
ON CONFLICT (id) DO NOTHING;

CREATE TABLE personas (
    id uuid PRIMARY KEY,
    tenant_id text NOT NULL DEFAULT 'default',
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    description text NOT NULL DEFAULT '',
    config jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(config) = 'object'),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'unknown')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX personas_scope_idx ON personas(scope_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX personas_scope_name_unique ON personas(scope_id, lower(name)) WHERE deleted_at IS NULL;

INSERT INTO personas (id, tenant_id, scope_id, name, description, config, status, created_at, updated_at, deleted_at)
SELECT id, tenant_id, scope_id, name, COALESCE(config->>'description', ''), config, status, created_at, updated_at, deleted_at
  FROM resources WHERE kind = 'AgentProfile'
ON CONFLICT (id) DO NOTHING;

CREATE TABLE skills (
    id uuid PRIMARY KEY,
    tenant_id text NOT NULL DEFAULT 'default',
    scope_id uuid NOT NULL REFERENCES scopes(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    identifier text NOT NULL CHECK (length(btrim(identifier)) BETWEEN 1 AND 200),
    category text NOT NULL DEFAULT 'diagnosis' CHECK (category IN ('diagnosis', 'monitoring', 'optimization', 'maintenance')),
    tags jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(tags) = 'array'),
    maintainer text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'unknown')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX skills_scope_idx ON skills(scope_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX skills_scope_identifier_unique ON skills(scope_id, lower(identifier)) WHERE deleted_at IS NULL;

INSERT INTO skills (id, tenant_id, scope_id, name, identifier, category, tags, maintainer, status, created_at, updated_at, deleted_at)
SELECT id, tenant_id, scope_id, name,
       COALESCE(
           NULLIF(btrim(config->>'identifier'), ''),
           NULLIF(regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g'), ''),
           id::text
       ),
       CASE config->>'category'
           WHEN 'diagnosis' THEN 'diagnosis'
           WHEN 'monitoring' THEN 'monitoring'
           WHEN 'optimization' THEN 'optimization'
           WHEN 'maintenance' THEN 'maintenance'
           ELSE 'diagnosis'
       END,
       CASE WHEN jsonb_typeof(config->'tags') = 'array' THEN config->'tags' ELSE '[]'::jsonb END,
       COALESCE(config->>'maintainer', config->>'owner', ''), status, created_at, updated_at, deleted_at
  FROM resources WHERE kind = 'Skill'
ON CONFLICT (id) DO NOTHING;

CREATE TABLE provider_scope_bindings (
    scope_id uuid NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    provider_id uuid NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    tag text NOT NULL CHECK (tag IN ('default', 'diagnosis', 'inspection', 'workflow')),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_id, tag)
);
INSERT INTO provider_scope_bindings (scope_id, provider_id, tag, created_by, created_at, updated_at)
SELECT scope_id, provider_resource_id, tag, created_by, created_at, updated_at
  FROM scope_ai_provider_bindings
ON CONFLICT (scope_id, tag) DO UPDATE SET provider_id = EXCLUDED.provider_id;
CREATE INDEX provider_scope_bindings_provider_idx ON provider_scope_bindings(provider_id);

ALTER TABLE ai_execution_events RENAME TO engine_execution_events;
ALTER INDEX ai_execution_events_order_idx RENAME TO engine_execution_events_order_idx;
ALTER TABLE ai_execution_tool_calls RENAME TO engine_execution_tool_calls;
ALTER INDEX ai_execution_tool_calls_resource_idx RENAME TO engine_execution_tool_calls_resource_idx;
ALTER INDEX ai_execution_tool_calls_provider_idx RENAME TO engine_execution_tool_calls_provider_idx;
ALTER TABLE engine_execution_tool_calls RENAME COLUMN provider_resource_id TO provider_id;

ALTER TABLE skill_versions RENAME COLUMN skill_resource_id TO skill_id;
ALTER TABLE skill_versions DROP CONSTRAINT IF EXISTS skill_versions_skill_resource_id_fkey;
ALTER TABLE skill_versions ADD CONSTRAINT skill_versions_skill_id_fkey FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE RESTRICT;
DROP INDEX IF EXISTS skill_versions_resource_idx;
CREATE INDEX skill_versions_skill_idx ON skill_versions(skill_id, version DESC);

ALTER TABLE skill_scope_defaults RENAME COLUMN skill_resource_id TO skill_id;
ALTER TABLE skill_scope_defaults DROP CONSTRAINT IF EXISTS skill_scope_defaults_skill_resource_id_fkey;
ALTER TABLE skill_scope_defaults ADD CONSTRAINT skill_scope_defaults_skill_id_fkey FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE RESTRICT;

ALTER TABLE agent_profile_versions RENAME TO persona_versions;
ALTER TABLE persona_versions RENAME COLUMN agent_profile_resource_id TO persona_id;
ALTER TABLE persona_versions DROP CONSTRAINT IF EXISTS agent_profile_versions_agent_profile_resource_id_fkey;
ALTER TABLE persona_versions ADD CONSTRAINT persona_versions_persona_id_fkey FOREIGN KEY (persona_id) REFERENCES personas(id) ON DELETE RESTRICT;
ALTER INDEX agent_profile_versions_profile_idx RENAME TO persona_versions_persona_idx;
ALTER INDEX agent_profile_versions_published_idx RENAME TO persona_versions_published_idx;

ALTER TABLE diagnosis_sessions RENAME COLUMN provider_resource_id TO provider_id;
ALTER TABLE diagnosis_sessions DROP CONSTRAINT IF EXISTS diagnosis_sessions_provider_resource_id_fkey;
ALTER TABLE diagnosis_sessions ADD CONSTRAINT diagnosis_sessions_provider_id_fkey FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE SET NULL;

ALTER TABLE inspection_policies RENAME COLUMN agent_profile_resource_id TO persona_id;
ALTER TABLE inspection_policies DROP CONSTRAINT IF EXISTS inspection_policies_agent_profile_resource_id_fkey;
ALTER TABLE inspection_policies ADD CONSTRAINT inspection_policies_persona_id_fkey FOREIGN KEY (persona_id) REFERENCES personas(id) ON DELETE SET NULL;

INSERT INTO role_permissions (role_id, permission)
SELECT roles.id, permissions.permission
  FROM roles
  JOIN (VALUES
    ('PlatformAdmin', 'provider:read'), ('PlatformAdmin', 'provider:manage'), ('PlatformAdmin', 'provider:use'),
    ('PlatformAdmin', 'skill:read'), ('PlatformAdmin', 'skill:manage'), ('PlatformAdmin', 'skill:execute'),
    ('PlatformAdmin', 'persona:read'), ('PlatformAdmin', 'persona:manage'), ('PlatformAdmin', 'persona:use'),
    ('PlatformOperator', 'provider:read'), ('PlatformOperator', 'provider:use'),
    ('PlatformOperator', 'skill:read'), ('PlatformOperator', 'skill:execute'),
    ('PlatformOperator', 'persona:read'), ('PlatformOperator', 'persona:use'),
    ('PlatformViewer', 'provider:read'), ('PlatformViewer', 'skill:read'), ('PlatformViewer', 'persona:read'),
    ('TeamAdmin', 'provider:read'), ('TeamAdmin', 'provider:manage'), ('TeamAdmin', 'provider:use'),
    ('TeamAdmin', 'skill:read'), ('TeamAdmin', 'skill:manage'), ('TeamAdmin', 'skill:execute'),
    ('TeamAdmin', 'persona:read'), ('TeamAdmin', 'persona:manage'), ('TeamAdmin', 'persona:use'),
    ('TeamOperator', 'provider:read'), ('TeamOperator', 'provider:use'),
    ('TeamOperator', 'skill:read'), ('TeamOperator', 'skill:execute'),
    ('TeamOperator', 'persona:read'), ('TeamOperator', 'persona:use'),
    ('TeamViewer', 'provider:read'), ('TeamViewer', 'skill:read'), ('TeamViewer', 'persona:read'),
    ('ProjectAdmin', 'provider:read'), ('ProjectAdmin', 'provider:manage'), ('ProjectAdmin', 'provider:use'),
    ('ProjectAdmin', 'skill:read'), ('ProjectAdmin', 'skill:manage'), ('ProjectAdmin', 'skill:execute'),
    ('ProjectAdmin', 'persona:read'), ('ProjectAdmin', 'persona:manage'), ('ProjectAdmin', 'persona:use'),
    ('ProjectOperator', 'provider:read'), ('ProjectOperator', 'provider:use'),
    ('ProjectOperator', 'skill:read'), ('ProjectOperator', 'skill:execute'),
    ('ProjectOperator', 'persona:read'), ('ProjectOperator', 'persona:use'),
    ('ProjectViewer', 'provider:read'), ('ProjectViewer', 'skill:read'), ('ProjectViewer', 'persona:read')
  ) AS permissions(role_name, permission) ON permissions.role_name = roles.name
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION validate_provider_scope_binding()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE provider_scope uuid; provider_status text;
BEGIN
    SELECT scope_id, status INTO provider_scope, provider_status
      FROM providers WHERE id = NEW.provider_id AND deleted_at IS NULL;
    IF provider_scope IS NULL OR provider_status <> 'active' OR NOT resource_scope_contains(provider_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'Provider is unavailable from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER provider_scope_bindings_validate
BEFORE INSERT OR UPDATE OF scope_id, provider_id ON provider_scope_bindings
FOR EACH ROW EXECUTE FUNCTION validate_provider_scope_binding();
CREATE TRIGGER provider_scope_bindings_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON provider_scope_bindings
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();
CREATE TRIGGER providers_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON providers
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();
CREATE TRIGGER personas_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON personas
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();
CREATE TRIGGER skills_authorization_revision
AFTER INSERT OR UPDATE OR DELETE ON skills
FOR EACH ROW EXECUTE FUNCTION bump_authorization_revision();
