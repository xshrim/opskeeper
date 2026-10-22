-- Remove the retired cluster-import and controlled-operation domains.
-- MCP snapshots remain because they are still part of resource context.
DROP TABLE IF EXISTS operation_executions;
DROP TABLE IF EXISTS operation_approvals;
DROP TABLE IF EXISTS operation_requests;
DROP TABLE IF EXISTS operation_policies;
DROP TABLE IF EXISTS discovery_items;
DROP TABLE IF EXISTS discovery_runs;

DELETE FROM role_permissions
 WHERE permission IN ('discovery:run', 'discovery:import', 'operation:approve');
