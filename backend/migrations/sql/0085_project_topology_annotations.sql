CREATE TABLE project_topology_annotations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT project_topology_annotations_name_check CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    CONSTRAINT project_topology_annotations_metadata_check CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX project_topology_annotations_project_idx
    ON project_topology_annotations(project_id, created_at, id);

CREATE TABLE project_topology_resources (
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, resource_id)
);

CREATE INDEX project_topology_resources_resource_idx
    ON project_topology_resources(resource_id);
