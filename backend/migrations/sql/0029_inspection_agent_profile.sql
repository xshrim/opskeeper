-- Inspection explanations are executed by AIEngine directly. A policy may
-- optionally select an AgentProfile, but it must not require a Skill.
ALTER TABLE inspection_policies
    ADD COLUMN IF NOT EXISTS agent_profile_resource_id uuid REFERENCES resources(id) ON DELETE SET NULL;

ALTER TABLE inspection_policies
    DROP COLUMN IF EXISTS skill_resource_ids;

CREATE INDEX IF NOT EXISTS inspection_policies_agent_profile_idx
    ON inspection_policies(agent_profile_resource_id)
    WHERE agent_profile_resource_id IS NOT NULL AND deleted_at IS NULL;
