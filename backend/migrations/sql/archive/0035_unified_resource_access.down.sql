DROP INDEX IF EXISTS resources_agent_ref_idx;
ALTER TABLE resources
    DROP COLUMN IF EXISTS agent_ref,
    ADD COLUMN IF NOT EXISTS access_mode text,
    ADD COLUMN IF NOT EXISTS mcp_server_resource_id uuid;
