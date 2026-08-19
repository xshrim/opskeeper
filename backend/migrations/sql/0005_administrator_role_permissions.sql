-- Administrators must include every permission held by the built-in roles
-- they are allowed to grant at their own authorization level.
WITH permission_seed(role_name, permission) AS (
    VALUES
        ('TeamAdmin', 'diagnosis:start'),
        ('TeamAdmin', 'diagnosis:read'),
        ('TeamAdmin', 'inspection:execute'),
        ('ProjectAdmin', 'diagnosis:start'),
        ('ProjectAdmin', 'diagnosis:read'),
        ('ProjectAdmin', 'inspection:execute')
)
INSERT INTO role_permissions (role_id, permission)
SELECT role.id, permission_seed.permission
  FROM permission_seed
  JOIN roles role ON role.name = permission_seed.role_name
ON CONFLICT DO NOTHING;
