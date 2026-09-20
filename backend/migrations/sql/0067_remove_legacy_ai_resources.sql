-- Hard cut to the independent Provider, Engine, Skill and Persona domains.
-- The generic resource catalog must not retain rows, schemas, permissions, or
-- relation types for those objects. Existing independent rows are preserved.

CREATE TEMP TABLE legacy_ai_resources ON COMMIT DROP AS
SELECT id
  FROM resources
 WHERE kind IN ('LLM', 'LLMProvider', 'AIProvider', 'Provider', 'Skill', 'AgentProfile', 'Persona', 'AIEngine');

DELETE FROM resource_relations
 WHERE relation_type IN ('uses_provider', 'uses_skill')
    OR source_resource_id IN (SELECT id FROM legacy_ai_resources)
    OR target_resource_id IN (SELECT id FROM legacy_ai_resources);

DELETE FROM resource_connection_checks WHERE resource_id IN (SELECT id FROM legacy_ai_resources);
DELETE FROM resource_sync_states WHERE resource_id IN (SELECT id FROM legacy_ai_resources);
DELETE FROM resource_role_bindings WHERE resource_id IN (SELECT id FROM legacy_ai_resources);
DELETE FROM scope_defaults WHERE resource_id IN (SELECT id FROM legacy_ai_resources);

DELETE FROM resource_schemas
 WHERE kind IN ('LLM', 'LLMProvider', 'AIProvider', 'Provider', 'Skill', 'AgentProfile', 'Persona', 'AIEngine');

DELETE FROM resources WHERE id IN (SELECT id FROM legacy_ai_resources);

ALTER TABLE resource_relations DROP CONSTRAINT IF EXISTS resource_relations_relation_type_check;
ALTER TABLE resource_relations
  ADD CONSTRAINT resource_relations_relation_type_check
  CHECK (relation_type IN ('contains', 'deployed_on', 'depends_on', 'observed_by', 'exposes', 'served_by_mcp'));

-- These tables were the old resource-backed execution/default stores. The
-- independent services use provider_scope_bindings, skill_versions,
-- skill_scope_defaults and the dedicated execution APIs instead.
DROP TABLE IF EXISTS skill_tool_calls;
DROP TABLE IF EXISTS skill_executions;
DROP TABLE IF EXISTS llm_scope_defaults;
