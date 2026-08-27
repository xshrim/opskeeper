UPDATE resource_schemas
SET schema = jsonb_set(
      jsonb_set(
        schema,
        '{properties,icon,title}',
        '"AI 模型图标"'::jsonb,
        true
      ),
      '{properties,default,title}',
      '"默认模型"'::jsonb,
      true
    ),
    display_name = 'AI 模型',
    description = '聚合一个或多个大模型节点，并按统一能力和故障转移策略提供模型调用入口。'
WHERE kind = 'AIModel';
