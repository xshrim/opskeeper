-- Built-in roles and their long-lived bindings are retained on rollback.
DELETE FROM role_permissions
 WHERE role_id = (SELECT id FROM roles WHERE name = 'ProjectMember')
   AND permission = 'organization:read';
