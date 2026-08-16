DROP TRIGGER IF EXISTS skill_scope_defaults_authorization_revision ON skill_scope_defaults;
DROP TRIGGER IF EXISTS skill_versions_authorization_revision ON skill_versions;
DROP TRIGGER IF EXISTS llm_scope_defaults_authorization_revision ON llm_scope_defaults;

DELETE FROM role_permissions
 WHERE permission IN ('skill:manage', 'skill:execute')
   AND role_id IN (SELECT id FROM roles WHERE name IN (
       'PlatformOperator', 'TeamAdmin', 'TeamOperator',
       'ProjectAdmin', 'ProjectOperator'
   ));

DELETE FROM resource_schemas WHERE kind IN ('LLMProvider', 'Skill', 'MCPServer') AND version = 2;

DROP TRIGGER IF EXISTS skill_scope_defaults_validate ON skill_scope_defaults;
DROP FUNCTION IF EXISTS validate_skill_scope_default();
DROP TRIGGER IF EXISTS skill_versions_validate ON skill_versions;
DROP FUNCTION IF EXISTS validate_skill_version();
DROP TRIGGER IF EXISTS llm_scope_defaults_validate ON llm_scope_defaults;
DROP FUNCTION IF EXISTS validate_llm_scope_default();

DROP TABLE IF EXISTS skill_tool_calls;
DROP TABLE IF EXISTS skill_executions;
DROP TABLE IF EXISTS skill_scope_defaults;
DROP TABLE IF EXISTS skill_versions;
DROP TABLE IF EXISTS llm_scope_defaults;
