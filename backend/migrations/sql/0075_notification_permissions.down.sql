DELETE FROM role_permissions WHERE permission IN ('notification:read','notification:manage','notification:test','notification:retry');
DELETE FROM resource_role_permissions WHERE permission IN ('notification:read','notification:manage','notification:test','notification:retry');
