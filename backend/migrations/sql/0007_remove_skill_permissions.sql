-- Skill is a resource kind. Its read, use and management authorization is
-- expressed through the ordinary resource permission model.
DELETE FROM role_permissions
 WHERE permission IN ('skill:execute', 'skill:manage');
