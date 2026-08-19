-- Keep the role catalog explicit for all supported authorization levels.
WITH seeded_roles(name, scope_type) AS (
    VALUES
        ('PlatformAdmin', 'platform'), ('PlatformOperator', 'platform'), ('PlatformViewer', 'platform'),
        ('TeamAdmin', 'team'), ('TeamOperator', 'team'), ('TeamViewer', 'team'),
        ('ProjectAdmin', 'project'), ('ProjectOperator', 'project'), ('ProjectViewer', 'project'),
        ('ProjectMember', 'project')
)
INSERT INTO roles (name, scope_type, builtin)
SELECT name, scope_type, true FROM seeded_roles
ON CONFLICT (name) DO UPDATE SET scope_type = EXCLUDED.scope_type, builtin = true;

-- Administrators may grant roles within their scope and descendants. Scope inheritance
-- restricts platform administrators to all levels, team administrators to team/project,
-- and project administrators to project only.
INSERT INTO role_permissions (role_id, permission)
SELECT id, 'member:grant' FROM roles
 WHERE name IN ('PlatformAdmin', 'TeamAdmin', 'ProjectAdmin')
ON CONFLICT DO NOTHING;

-- Project participants may enter their project but have no project-wide resource access.
INSERT INTO role_permissions (role_id, permission)
SELECT id, 'organization:read' FROM roles WHERE name = 'ProjectMember'
ON CONFLICT DO NOTHING;

DELETE FROM role_permissions
 WHERE role_id = (SELECT id FROM roles WHERE name = 'ProjectMember')
   AND permission LIKE 'resource:%';
