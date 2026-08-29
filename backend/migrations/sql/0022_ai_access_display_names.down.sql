UPDATE resource_schemas
   SET display_name = 'AI 提供方'
 WHERE kind = 'AIProvider';

UPDATE resource_schemas
   SET display_name = 'AI 端点',
       description = '面向业务的统一模型策略入口，引用一个 AIProvider 并约束模型选择。'
 WHERE kind = 'AIEndpoint';
