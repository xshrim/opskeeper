-- ProjectMember was a navigation-only role. Preserve every existing grant by
-- moving it to ProjectViewer before removing the obsolete role.
INSERT INTO role_bindings (subject_type, subject_id, role_id, scope_id)
SELECT binding.subject_type, binding.subject_id, viewer.id, binding.scope_id
  FROM role_bindings binding
  JOIN roles member ON member.id = binding.role_id AND member.name = 'ProjectMember'
 CROSS JOIN roles viewer
 WHERE viewer.name = 'ProjectViewer'
ON CONFLICT (subject_type, subject_id, role_id, scope_id) DO NOTHING;

DELETE FROM role_bindings
 WHERE role_id = (SELECT id FROM roles WHERE name = 'ProjectMember');

DELETE FROM role_permissions
 WHERE role_id = (SELECT id FROM roles WHERE name = 'ProjectMember');

DELETE FROM roles WHERE name = 'ProjectMember';
