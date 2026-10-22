DROP INDEX IF EXISTS resources_mcp_server_resource_idx;
DROP INDEX IF EXISTS resources_access_mode_idx;
ALTER TABLE resources
    DROP CONSTRAINT IF EXISTS resources_mcp_server_resource_fk,
    DROP CONSTRAINT IF EXISTS resources_agent_mcp_check,
    DROP CONSTRAINT IF EXISTS resources_direct_mcp_check,
    DROP CONSTRAINT IF EXISTS resources_access_mode_check,
    DROP COLUMN IF EXISTS mcp_server_resource_id,
    DROP COLUMN IF EXISTS access_mode;
