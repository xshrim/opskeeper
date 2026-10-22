UPDATE resource_schemas
   SET schema = schema #- '{properties,icon}'
 WHERE kind = 'AIEngine'
   AND version = 1;

UPDATE resource_schemas
   SET schema = schema #- '{properties,default}'
 WHERE kind = 'AIEngine'
   AND version = 1;
