INSERT INTO role_permissions (role_id, permission)
SELECT role.id, 'ai_engine:default_manage'
  FROM roles role
 WHERE role.name IN ('PlatformAdmin', 'TeamAdmin', 'ProjectAdmin')
ON CONFLICT DO NOTHING;

UPDATE resource_schemas
   SET schema = jsonb_set(
     schema,
     '{properties,endpoints,items,properties,temperature}',
     '{"type":"number","minimum":0,"maximum":2}'::jsonb,
     true
   )
 WHERE kind = 'AIEngine' AND version = 1;
