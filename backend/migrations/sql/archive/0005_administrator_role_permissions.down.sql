DELETE FROM role_permissions
 WHERE permission IN ('diagnosis:start', 'diagnosis:read', 'inspection:execute')
   AND role_id IN (SELECT id FROM roles WHERE name IN ('TeamAdmin', 'ProjectAdmin'));
