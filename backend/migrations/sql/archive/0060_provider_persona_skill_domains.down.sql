ALTER TABLE engine_execution_tool_calls RENAME COLUMN provider_id TO provider_resource_id;
ALTER INDEX engine_execution_tool_calls_provider_idx RENAME TO ai_execution_tool_calls_provider_idx;
ALTER INDEX engine_execution_tool_calls_resource_idx RENAME TO ai_execution_tool_calls_resource_idx;
ALTER TABLE engine_execution_tool_calls RENAME TO ai_execution_tool_calls;
ALTER INDEX engine_execution_events_order_idx RENAME TO ai_execution_events_order_idx;
ALTER TABLE engine_execution_events RENAME TO ai_execution_events;

ALTER TABLE inspection_policies RENAME COLUMN persona_id TO agent_profile_resource_id;
ALTER TABLE inspection_policies DROP CONSTRAINT IF EXISTS inspection_policies_persona_id_fkey;
ALTER TABLE inspection_policies ADD CONSTRAINT inspection_policies_agent_profile_resource_id_fkey FOREIGN KEY (agent_profile_resource_id) REFERENCES resources(id) ON DELETE SET NULL;

ALTER TABLE diagnosis_sessions RENAME COLUMN provider_id TO provider_resource_id;
ALTER TABLE diagnosis_sessions DROP CONSTRAINT IF EXISTS diagnosis_sessions_provider_id_fkey;
ALTER TABLE diagnosis_sessions ADD CONSTRAINT diagnosis_sessions_provider_resource_id_fkey FOREIGN KEY (provider_resource_id) REFERENCES resources(id) ON DELETE SET NULL;

ALTER TABLE persona_versions RENAME COLUMN persona_id TO agent_profile_resource_id;
ALTER TABLE persona_versions DROP CONSTRAINT IF EXISTS persona_versions_persona_id_fkey;
ALTER TABLE persona_versions ADD CONSTRAINT agent_profile_versions_agent_profile_resource_id_fkey FOREIGN KEY (agent_profile_resource_id) REFERENCES resources(id) ON DELETE CASCADE;
ALTER INDEX persona_versions_persona_idx RENAME TO agent_profile_versions_profile_idx;
ALTER INDEX persona_versions_published_idx RENAME TO agent_profile_versions_published_idx;
ALTER TABLE persona_versions RENAME TO agent_profile_versions;

ALTER TABLE skill_scope_defaults RENAME COLUMN skill_id TO skill_resource_id;
ALTER TABLE skill_scope_defaults DROP CONSTRAINT IF EXISTS skill_scope_defaults_skill_id_fkey;
ALTER TABLE skill_scope_defaults ADD CONSTRAINT skill_scope_defaults_skill_resource_id_fkey FOREIGN KEY (skill_resource_id) REFERENCES resources(id) ON DELETE RESTRICT;
DROP INDEX IF EXISTS skill_versions_skill_idx;
ALTER TABLE skill_versions RENAME COLUMN skill_id TO skill_resource_id;
ALTER TABLE skill_versions DROP CONSTRAINT IF EXISTS skill_versions_skill_id_fkey;
ALTER TABLE skill_versions ADD CONSTRAINT skill_versions_skill_resource_id_fkey FOREIGN KEY (skill_resource_id) REFERENCES resources(id) ON DELETE RESTRICT;

INSERT INTO scope_ai_provider_bindings (scope_id, provider_resource_id, tag, created_by, created_at, updated_at)
SELECT scope_id, provider_id, tag, created_by, created_at, updated_at
  FROM provider_scope_bindings
ON CONFLICT (scope_id, tag) DO UPDATE SET
    provider_resource_id = EXCLUDED.provider_resource_id,
    created_by = EXCLUDED.created_by,
    updated_at = EXCLUDED.updated_at;

DROP TRIGGER IF EXISTS providers_authorization_revision ON providers;
DROP TRIGGER IF EXISTS personas_authorization_revision ON personas;
DROP TRIGGER IF EXISTS skills_authorization_revision ON skills;
DROP TRIGGER IF EXISTS provider_scope_bindings_authorization_revision ON provider_scope_bindings;
DROP TRIGGER IF EXISTS provider_scope_bindings_validate ON provider_scope_bindings;
DROP FUNCTION IF EXISTS validate_provider_scope_binding();
DROP TABLE IF EXISTS provider_scope_bindings;
DROP TABLE IF EXISTS skills;
DROP TABLE IF EXISTS personas;
DROP TABLE IF EXISTS providers;
