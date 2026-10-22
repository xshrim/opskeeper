-- Replace the legacy model/endpoint resource contract with the three-layer
-- AIProvider -> AIEndpoint -> AIEngine contract. Existing AI resources are
-- intentionally reset; deployments using this migration start with an empty
-- AI catalog and can recreate resources through the new API.
UPDATE resources
   SET kind = 'AIProvider', updated_at = now()
 WHERE kind = 'LLMProvider';
UPDATE resources
   SET kind = 'AIEndpoint', updated_at = now(), config = '{}'::jsonb
 WHERE kind = 'AIModel';

UPDATE resource_schemas
   SET kind = 'AIProvider', display_name = 'AI 提供方',
       description = '保存模型服务地址、凭证和可用模型目录。', icon = 'llm'
 WHERE kind = 'LLMProvider';
UPDATE resource_schemas
   SET kind = 'AIEndpoint', display_name = 'AI 端点',
       description = '面向业务的统一模型策略入口，引用一个 AIProvider 并约束模型选择。', icon = 'llm'
 WHERE kind = 'AIModel';

UPDATE resource_schemas
   SET schema = '{
     "$schema":"https://json-schema.org/draft/2020-12/schema",
     "type":"object","additionalProperties":false,
     "required":["provider_type","base_url","models"],
     "properties":{
       "provider_type":{"title":"提供方类型","type":"string"},
       "base_url":{"title":"服务地址","type":"string","format":"uri"},
       "models":{"title":"模型目录","type":"array","minItems":1},
       "default_model":{"title":"默认模型","type":"string"},
       "timeout_seconds":{"title":"超时秒数","type":"integer"},
       "icon":{"title":"图标","type":"string"}
     }}'::jsonb
 WHERE kind = 'AIProvider';

UPDATE resource_schemas
   SET schema = '{
     "$schema":"https://json-schema.org/draft/2020-12/schema",
     "type":"object","additionalProperties":false,
     "required":["provider_id"],
     "properties":{
       "provider_id":{"title":"AI 提供方","type":"string"},
       "default_model":{"title":"默认模型","type":"string"},
       "allowed_models":{"title":"允许的模型","type":"array"},
       "requirements":{"title":"能力要求","type":"array"},
       "allow_model_override":{"title":"允许模型覆盖","type":"boolean"},
       "allow_fallback":{"title":"允许故障切换","type":"boolean"},
       "icon":{"title":"图标","type":"string"},
       "default":{"title":"默认端点","type":"boolean"}
     }}'::jsonb
 WHERE kind = 'AIEndpoint';

ALTER TABLE llm_scope_defaults RENAME TO ai_endpoint_scope_defaults;
ALTER TABLE ai_endpoint_scope_defaults RENAME COLUMN provider_resource_id TO ai_endpoint_resource_id;

CREATE OR REPLACE FUNCTION validate_llm_scope_default()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE endpoint_scope uuid; endpoint_kind text; endpoint_status text;
BEGIN
  SELECT scope_id, kind, status INTO endpoint_scope, endpoint_kind, endpoint_status
    FROM resources WHERE id = NEW.ai_endpoint_resource_id AND deleted_at IS NULL;
  IF endpoint_kind IS DISTINCT FROM 'AIEndpoint' OR endpoint_status IS DISTINCT FROM 'active'
     OR NOT resource_scope_contains(endpoint_scope, NEW.scope_id) THEN
    RAISE EXCEPTION 'default AIEndpoint is unavailable from scope' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END; $$;

UPDATE role_permissions SET permission = 'endpoint:manage' WHERE permission = 'model:manage';
