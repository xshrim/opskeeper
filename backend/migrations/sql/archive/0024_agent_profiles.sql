-- AgentProfile is a scope-scoped expert prompt and tool contract. It is a
-- Resource so resource:read/resource:use permissions apply uniformly.
INSERT INTO resource_schemas (kind, version, schema, status, display_name, description, icon)
VALUES (
    'AgentProfile', 1,
    '{
      "$schema":"https://json-schema.org/draft/2020-12/schema",
      "type":"object","additionalProperties":false,
      "required":["instruction","version","capabilities","allowed_tools","target_kinds","enabled"],
      "properties":{
        "description":{"type":"string","maxLength":1000},
        "version":{"type":"integer","minimum":1},
        "instruction":{"type":"string","minLength":1,"maxLength":20000},
        "capabilities":{"type":"array","maxItems":30,"items":{"type":"string","maxLength":80}},
        "allowed_tools":{"type":"array","maxItems":50,"items":{"type":"string","pattern":"^[A-Za-z0-9][A-Za-z0-9:_-]{0,119}$"}},
        "target_kinds":{"type":"array","maxItems":50,"items":{"type":"string","maxLength":120}},
        "input_schema":{"type":"object"},
        "output_schema":{"type":"object"},
        "enabled":{"type":"boolean"}
      }
    }'::jsonb,
    'active', 'Agent 专家配置', '可复用的专家提示词、工具白名单和模型能力契约。', 'agent'
)
ON CONFLICT (kind, version) DO UPDATE SET
    schema = EXCLUDED.schema,
    status = EXCLUDED.status,
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    icon = EXCLUDED.icon;
