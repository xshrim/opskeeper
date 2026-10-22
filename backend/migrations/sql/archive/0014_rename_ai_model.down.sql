UPDATE role_permissions
   SET permission = 'engine:manage'
 WHERE permission = 'model:manage';

UPDATE resource_schemas
   SET kind = 'AIEngine',
       display_name = 'AI 引擎',
       description = '聚合一个或多个模型连接，并按统一能力和故障转移策略提供模型调用入口。'
 WHERE kind = 'AIModel';

UPDATE resources
   SET kind = 'AIEngine', updated_at = now()
 WHERE kind = 'AIModel';
