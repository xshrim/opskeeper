UPDATE resource_schemas
   SET display_name = '模型服务商'
 WHERE kind = 'AIProvider';

UPDATE resource_schemas
   SET display_name = 'AI 接入',
       description = '绑定模型服务商和默认模型，供 AI 引擎统一调用。'
 WHERE kind = 'AIEndpoint';
