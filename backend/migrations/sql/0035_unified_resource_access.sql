ALTER TABLE resources
    ADD COLUMN agent_ref uuid REFERENCES resources(id) ON DELETE RESTRICT;

ALTER TABLE resources
    DROP COLUMN IF EXISTS access_mode,
    DROP COLUMN IF EXISTS mcp_server_resource_id;

CREATE INDEX resources_agent_ref_idx
    ON resources(agent_ref) WHERE deleted_at IS NULL;
