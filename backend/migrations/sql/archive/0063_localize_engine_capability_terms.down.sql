CREATE TEMP TABLE engine_capability_term_localization (
    old_term text PRIMARY KEY,
    new_term text NOT NULL
) ON COMMIT DROP;

INSERT INTO engine_capability_term_localization (old_term, new_term)
VALUES
    ('智能体循环', 'Agent Loop'),
    ('上下文编排', 'Context Orchestration'),
    ('工具调用', 'Tool Calling'),
    ('工具网关', 'Tool Gateway'),
    ('技能编排', 'Skill Orchestration'),
    ('专家路由', 'Persona Routing'),
    ('结构化输出', 'Structured Output'),
    ('检索增强', 'RAG Retrieval'),
    ('工作流编排', 'Workflow Orchestration'),
    ('流式事件', 'Streaming Events');

DELETE FROM engine_capability_terms terms
USING engine_capability_term_localization mapping
WHERE lower(terms.term) = lower(mapping.old_term)
  AND EXISTS (
      SELECT 1
      FROM engine_capability_terms existing
      WHERE lower(existing.term) = lower(mapping.new_term)
  );

UPDATE engine_capability_terms terms
SET term = mapping.new_term
FROM engine_capability_term_localization mapping
WHERE lower(terms.term) = lower(mapping.old_term);

UPDATE engines
SET config = jsonb_set(
    config,
    '{capabilities}',
    '["Agent Loop", "Context Orchestration", "Tool Calling", "Tool Gateway", "Skill Orchestration", "Persona Routing", "Structured Output", "RAG Retrieval", "Workflow Orchestration", "Streaming Events"]'::jsonb,
    true
)
WHERE config->'capabilities' = '["智能体循环", "上下文编排", "工具调用", "工具网关", "技能编排", "专家路由", "结构化输出", "检索增强", "工作流编排", "流式事件"]'::jsonb;
