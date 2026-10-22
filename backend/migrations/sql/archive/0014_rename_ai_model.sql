UPDATE resources
   SET kind = 'AIModel', updated_at = now()
 WHERE kind = 'AIEngine';

UPDATE resource_schemas
   SET kind = 'AIModel',
       display_name = 'AI 模型',
       description = '聚合一个或多个大模型节点，并按统一能力和故障转移策略提供模型调用入口。'
 WHERE kind = 'AIEngine';

UPDATE role_permissions
   SET permission = 'model:manage'
 WHERE permission = 'engine:manage';
