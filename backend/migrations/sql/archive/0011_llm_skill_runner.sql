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
