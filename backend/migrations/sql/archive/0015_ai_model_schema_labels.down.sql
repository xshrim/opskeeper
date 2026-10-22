UPDATE resource_schemas
SET schema = jsonb_set(
      jsonb_set(
        schema,
        '{properties,icon,title}',
        '"AI 引擎图标"'::jsonb,
        true
      ),
      '{properties,default,title}',
      '"默认引擎"'::jsonb,
      true
    ),
    display_name = 'AI 引擎',
    description = '聚合一个或多个模型连接，并按统一能力和故障转移策略提供模型调用入口。'
WHERE kind = 'AIModel';
