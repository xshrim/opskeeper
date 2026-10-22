CREATE TABLE agent_profile_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_profile_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    version integer NOT NULL CHECK (version > 0),
    config jsonb NOT NULL,
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'disabled')),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    UNIQUE (agent_profile_resource_id, version)
);

CREATE INDEX agent_profile_versions_profile_idx
    ON agent_profile_versions(agent_profile_resource_id, version DESC);

CREATE INDEX agent_profile_versions_published_idx
    ON agent_profile_versions(agent_profile_resource_id, version DESC)
    WHERE status = 'published';
