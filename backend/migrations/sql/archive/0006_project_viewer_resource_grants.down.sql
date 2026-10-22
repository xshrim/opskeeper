-- Rollback restores the retired role definition only. Existing grants remain
-- ProjectViewer grants so a rollback does not silently reduce access.
INSERT INTO roles (name, scope_type, builtin)
VALUES ('ProjectMember', 'project', true)
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_id, permission)
SELECT id, 'organization:read' FROM roles WHERE name = 'ProjectMember'
ON CONFLICT DO NOTHING;
