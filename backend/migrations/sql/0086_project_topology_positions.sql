CREATE TABLE project_topology_positions (
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    node_id uuid NOT NULL,
    position_x integer NOT NULL CHECK (position_x BETWEEN 0 AND 10000),
    position_y integer NOT NULL CHECK (position_y BETWEEN 0 AND 10000),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, node_id)
);
