CREATE TABLE project_topology_edges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_node_id uuid NOT NULL,
    target_node_id uuid NOT NULL,
    relation text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT project_topology_edges_distinct_nodes CHECK (source_node_id <> target_node_id),
    CONSTRAINT project_topology_edges_relation_check CHECK (length(btrim(relation)) BETWEEN 1 AND 80),
    CONSTRAINT project_topology_edges_metadata_check CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX project_topology_edges_project_idx
    ON project_topology_edges(project_id, created_at, id);
