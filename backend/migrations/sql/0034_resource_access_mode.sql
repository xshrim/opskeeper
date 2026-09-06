-- Make resource connection mode explicit while retaining subtype as the
-- compatibility mirror used by older clients.
ALTER TABLE resources
    ADD COLUMN access_mode text,
    ADD COLUMN mcp_server_resource_id uuid;

ALTER TABLE resources
    ADD CONSTRAINT resources_access_mode_check
    CHECK (access_mode IS NULL OR access_mode IN ('direct', 'agent')),
    ADD CONSTRAINT resources_direct_mcp_check
    CHECK (access_mode IS DISTINCT FROM 'direct' OR mcp_server_resource_id IS NULL),
    ADD CONSTRAINT resources_agent_mcp_check
    CHECK (access_mode IS DISTINCT FROM 'agent' OR mcp_server_resource_id IS NOT NULL) NOT VALID,
    ADD CONSTRAINT resources_mcp_server_resource_fk
    FOREIGN KEY (mcp_server_resource_id) REFERENCES resources(id) ON DELETE RESTRICT;

-- The catalog migration normalized these resource kinds to Direct/Agent
-- subtype values. Convert those values to the explicit field. Non-connection
-- resources intentionally remain NULL.
UPDATE resources
   SET access_mode = CASE lower(btrim(subtype))
       WHEN 'agent' THEN 'agent'
       ELSE 'direct'
   END,
       updated_at = now()
 WHERE kind IN ('Host', 'Docker', 'Kubernetes', 'Redis', 'TongRDS', 'Kafka',
                'RabbitMQ', 'Elasticsearch', 'OceanBase', 'Oracle', 'MySQL',
                'PostgreSQL', 'Prometheus', 'Loki')
   AND access_mode IS NULL;

-- Recover an Agent transport target when discovery already recorded the
-- explicit served_by_mcp relation. Ambiguous or missing relations are left
-- unlinked so the resource is visible and can be repaired through the API.
UPDATE resources logical_resource
   SET mcp_server_resource_id = linked.server_id,
       updated_at = now()
  FROM (
      SELECT relation.source_resource_id AS logical_id,
             min(relation.target_resource_id::text)::uuid AS server_id
        FROM resource_relations relation
        JOIN resources server ON server.id = relation.target_resource_id
       WHERE relation.relation_type = 'served_by_mcp'
         AND server.kind = 'MCPServer'
       GROUP BY relation.source_resource_id
       HAVING count(DISTINCT relation.target_resource_id) = 1
  ) linked
 WHERE logical_resource.id = linked.logical_id
   AND logical_resource.access_mode = 'agent'
   AND logical_resource.mcp_server_resource_id IS NULL;

CREATE INDEX resources_access_mode_idx
    ON resources(access_mode) WHERE deleted_at IS NULL;
CREATE INDEX resources_mcp_server_resource_idx
    ON resources(mcp_server_resource_id) WHERE deleted_at IS NULL;
