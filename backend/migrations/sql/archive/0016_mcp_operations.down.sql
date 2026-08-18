DELETE FROM role_permissions WHERE permission = 'operation:approve';
DROP TABLE IF EXISTS operation_executions;
DROP TABLE IF EXISTS operation_approvals;
DROP TABLE IF EXISTS operation_requests;
DROP TABLE IF EXISTS operation_policies;
DROP TABLE IF EXISTS mcp_server_snapshots;
