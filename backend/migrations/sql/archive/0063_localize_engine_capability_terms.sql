-- Localize the built-in Engine capability vocabulary without rewriting the
-- already-applied 0062 migration checksum.

CREATE TEMP TABLE engine_capability_term_localization (
    old_term text PRIMARY KEY,
    new_term text NOT NULL
) ON COMMIT DROP;

INSERT INTO engine_capability_term_localization (old_term, new_term)
VALUES
    ('Agent Loop', '智能体循环'),
    ('Context Orchestration', '上下文编排'),
    ('Tool Calling', '工具调用'),
    ('Tool Gateway', '工具网关'),
    ('Skill Orchestration', '技能编排'),
    ('Persona Routing', '专家路由'),
    ('Structured Output', '结构化输出'),
    ('RAG Retrieval', '检索增强'),
    ('Workflow Orchestration', '工作流编排'),
    ('Streaming Events', '流式事件');

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
    '["智能体循环", "上下文编排", "工具调用", "工具网关", "技能编排", "专家路由", "结构化输出", "检索增强", "工作流编排", "流式事件"]'::jsonb,
    true
)
WHERE config->'capabilities' = '["Agent Loop", "Context Orchestration", "Tool Calling", "Tool Gateway", "Skill Orchestration", "Persona Routing", "Structured Output", "RAG Retrieval", "Workflow Orchestration", "Streaming Events"]'::jsonb;
