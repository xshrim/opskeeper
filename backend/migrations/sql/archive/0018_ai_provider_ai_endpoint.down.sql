UPDATE role_permissions SET permission = 'model:manage' WHERE permission = 'endpoint:manage';
ALTER TABLE ai_endpoint_scope_defaults RENAME COLUMN ai_endpoint_resource_id TO provider_resource_id;
ALTER TABLE ai_endpoint_scope_defaults RENAME TO llm_scope_defaults;
UPDATE resource_schemas SET kind = 'AIModel', display_name = 'AI 模型' WHERE kind = 'AIEndpoint';
UPDATE resource_schemas SET kind = 'LLMProvider', display_name = '大模型服务' WHERE kind = 'AIProvider';
UPDATE resources SET kind = 'AIModel' WHERE kind = 'AIEndpoint';
UPDATE resources SET kind = 'LLMProvider' WHERE kind = 'AIProvider';
