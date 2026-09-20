-- Persist the Engine capability vocabulary used by the LLM workspace.
-- Terms are global catalog metadata; individual Engines still store the
-- selected terms in engines.config.capabilities.

CREATE TABLE engine_capability_terms (
    term text PRIMARY KEY CHECK (length(btrim(term)) BETWEEN 1 AND 80),
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX engine_capability_terms_lower_unique ON engine_capability_terms (lower(term));

INSERT INTO engine_capability_terms (term)
VALUES
    ('Agent Loop'),
    ('Context Orchestration'),
    ('Tool Calling'),
    ('Tool Gateway'),
    ('Skill Orchestration'),
    ('Persona Routing'),
    ('Structured Output'),
    ('RAG Retrieval'),
    ('Workflow Orchestration'),
    ('Streaming Events')
ON CONFLICT DO NOTHING;

-- Replace only the migration's original placeholder list. Custom capability
-- text entered after 0061 is left untouched.
UPDATE engines
SET config = jsonb_set(
    config,
    '{capabilities}',
    '["Agent Loop", "Context Orchestration", "Tool Calling", "Tool Gateway", "Skill Orchestration", "Persona Routing", "Structured Output", "RAG Retrieval", "Workflow Orchestration", "Streaming Events"]'::jsonb,
    true
)
WHERE config->'capabilities' = '["诊断编排", "工具调用", "结构化输出", "流式响应"]'::jsonb;
