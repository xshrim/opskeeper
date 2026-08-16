INSERT INTO skill_versions (skill_resource_id, version, manifest, input_schema, output_schema, tools, risk_level, status, published_at)
SELECT version.skill_resource_id, 2, version.manifest, version.input_schema,
       '{"type":"object","required":["facts","findings","evidence","hypotheses","confidence","recommendations"],"properties":{"facts":{"type":"object"},"findings":{"type":"array"},"evidence":{"type":"array"},"hypotheses":{"type":"array"},"confidence":{"type":"number","minimum":0,"maximum":1},"recommendations":{"type":"array"}},"additionalProperties":false}'::jsonb,
       version.tools, version.risk_level, 'published', now()
  FROM skill_versions AS version
  JOIN resources AS resource ON resource.id = version.skill_resource_id
 WHERE version.version = 1
   AND resource.kind = 'Skill'
   AND resource.config->>'owner' = 'OpsKeeper builtin'
   AND resource.name IN ('Kubernetes 工作负载诊断', 'PostgreSQL 健康诊断', 'Redis 健康诊断', 'Kafka 健康诊断')
ON CONFLICT (skill_resource_id, version) DO NOTHING;

UPDATE skill_versions AS version
   SET status = 'disabled'
  FROM resources AS resource
 WHERE version.skill_resource_id = resource.id
   AND version.version = 1
   AND resource.kind = 'Skill'
   AND resource.config->>'owner' = 'OpsKeeper builtin';
