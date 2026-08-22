DELETE FROM role_permissions
 WHERE permission = 'ai_engine:default_manage';

UPDATE resource_schemas
   SET schema = schema #- '{properties,endpoints,items,properties,temperature}'
 WHERE kind = 'AIEngine' AND version = 1;
