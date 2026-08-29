-- T05: scope-owned knowledge bases and versioned workflow resources. The
-- resource catalog remains the authorization boundary; this migration only
-- adds schemas and resumable workflow run state.
INSERT INTO resource_schemas (kind, version, schema, status, display_name, description, icon)
VALUES
('KnowledgeBase', 1,
 '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["documents"],"properties":{"documents":{"type":"array","maxItems":10000,"items":{"type":"object","additionalProperties":false,"required":["id","content"],"properties":{"id":{"type":"string","minLength":1,"maxLength":200},"title":{"type":"string","maxLength":500},"content":{"type":"string","minLength":1,"maxLength":200000},"source_uri":{"type":"string","maxLength":2000},"metadata":{"type":"object"}}}}}}'::jsonb,
 'active', '知识库', '按 Scope 管理、可检索并提供引用的知识文档集合。', 'book'),
('Workflow', 1,
 '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["version","nodes","edges","enabled"],"properties":{"version":{"type":"integer","minimum":1},"description":{"type":"string","maxLength":2000},"enabled":{"type":"boolean"},"nodes":{"type":"array","minItems":1,"maxItems":200,"items":{"type":"object","additionalProperties":false,"required":["id","type","name"],"properties":{"id":{"type":"string","pattern":"^[A-Za-z][A-Za-z0-9_.-]{0,119}$"},"type":{"type":"string","enum":["agent","skill","tool","retrieval","condition","parallel","approval"]},"name":{"type":"string","minLength":1,"maxLength":200},"config":{"type":"object"},"retry":{"type":"object","additionalProperties":false,"properties":{"max_attempts":{"type":"integer","minimum":0,"maximum":10},"backoff_seconds":{"type":"integer","minimum":0,"maximum":3600}}},"timeout_seconds":{"type":"integer","minimum":0,"maximum":3600}}}},"edges":{"type":"array","maxItems":1000,"items":{"type":"object","additionalProperties":false,"required":["from","to"],"properties":{"from":{"type":"string"},"to":{"type":"string"},"condition":{"type":"string","maxLength":2000}}}}}}'::jsonb,
 'active', 'AI 工作流', '由 Agent、Skill、Tool、检索、条件、并行和审批节点组成的版本化 DAG。', 'workflow')
ON CONFLICT (kind, version) DO UPDATE SET
 schema = EXCLUDED.schema, status = EXCLUDED.status,
 display_name = EXCLUDED.display_name, description = EXCLUDED.description,
 icon = EXCLUDED.icon;

CREATE TABLE IF NOT EXISTS ai_workflow_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    workflow_version integer NOT NULL CHECK (workflow_version > 0),
    execution_id text NOT NULL UNIQUE,
    scope_id uuid NOT NULL REFERENCES scopes(id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','waiting_approval','succeeded','failed','cancelled')),
    current_node_id text NOT NULL DEFAULT '',
    attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    input jsonb NOT NULL DEFAULT '{}'::jsonb,
    state jsonb NOT NULL DEFAULT '{}'::jsonb,
    error_code text NOT NULL DEFAULT '',
    error_message text NOT NULL DEFAULT '',
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);
CREATE INDEX IF NOT EXISTS ai_workflow_runs_workflow_idx ON ai_workflow_runs(workflow_resource_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ai_workflow_runs_scope_idx ON ai_workflow_runs(scope_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ai_workflow_runs_status_idx ON ai_workflow_runs(status, updated_at);

CREATE OR REPLACE FUNCTION ai_workflow_runs_touch()
RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.updated_at = now(); RETURN NEW; END $$;
DROP TRIGGER IF EXISTS ai_workflow_runs_updated_at ON ai_workflow_runs;
CREATE TRIGGER ai_workflow_runs_updated_at BEFORE UPDATE ON ai_workflow_runs
FOR EACH ROW EXECUTE FUNCTION ai_workflow_runs_touch();
