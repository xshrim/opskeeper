-- Skill is a plan source only. Execution state and tool audit are owned by
-- AIEngine, so the duplicate Skill-specific stores are removed.
DROP TABLE IF EXISTS skill_tool_calls;
DROP TABLE IF EXISTS skill_executions;
