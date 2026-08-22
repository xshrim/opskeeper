UPDATE resource_schemas
   SET schema = jsonb_set(
     jsonb_set(
       schema,
       '{properties,icon}',
       '{"title":"AI 引擎图标","type":"string","maxLength":80}'::jsonb,
       true
     ),
     '{properties,default}',
     '{"title":"默认引擎","type":"boolean"}'::jsonb,
     true
   )
 WHERE kind = 'AIEngine'
   AND version = 1;
