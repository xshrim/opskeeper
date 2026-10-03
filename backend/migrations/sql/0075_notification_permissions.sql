INSERT INTO role_permissions(role_id,permission)
SELECT role.id, permission.value
  FROM roles role
 CROSS JOIN LATERAL unnest(CASE role.name
       WHEN 'PlatformAdmin' THEN ARRAY['notification:read','notification:manage','notification:test','notification:retry']::text[]
       WHEN 'TeamAdmin' THEN ARRAY['notification:read','notification:manage','notification:test','notification:retry']::text[]
       WHEN 'ProjectAdmin' THEN ARRAY['notification:read','notification:manage','notification:test','notification:retry']::text[]
       WHEN 'PlatformOperator' THEN ARRAY['notification:read','notification:test']::text[]
       WHEN 'TeamOperator' THEN ARRAY['notification:read','notification:test']::text[]
       WHEN 'ProjectOperator' THEN ARRAY['notification:read','notification:test']::text[]
       WHEN 'PlatformViewer' THEN ARRAY['notification:read']::text[]
       WHEN 'TeamViewer' THEN ARRAY['notification:read']::text[]
       WHEN 'ProjectViewer' THEN ARRAY['notification:read']::text[]
       ELSE ARRAY[]::text[] END) AS permission(value)
ON CONFLICT(role_id,permission) DO NOTHING;

INSERT INTO resource_role_permissions(role_id,permission)
SELECT role.id, permission.value
  FROM resource_roles role
 CROSS JOIN LATERAL unnest(CASE role.name
       WHEN 'ResourceAdmin' THEN ARRAY['notification:read','notification:manage','notification:test','notification:retry']::text[]
       WHEN 'ResourceViewer' THEN ARRAY['notification:read']::text[]
       ELSE ARRAY[]::text[] END) AS permission(value)
ON CONFLICT(role_id,permission) DO NOTHING;
