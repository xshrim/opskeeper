DROP TRIGGER IF EXISTS ai_workflow_runs_updated_at ON ai_workflow_runs;
DROP FUNCTION IF EXISTS ai_workflow_runs_touch();
DROP TABLE IF EXISTS ai_workflow_runs;
DELETE FROM resource_schemas WHERE kind IN ('KnowledgeBase', 'Workflow') AND version = 1;
