INSERT INTO resources (scope_id, kind, schema_version, name, config, status)
SELECT platform.scope_id, 'Skill', 2, item.name, jsonb_build_object('summary', item.summary, 'owner', 'OpsKeeper builtin'), 'active'
  FROM platforms platform
 CROSS JOIN (VALUES
    ('Kubernetes 工作负载诊断', 'Kubernetes 工作负载、Pod、事件、探针和发布状态的只读诊断。'),
    ('PostgreSQL 健康诊断', 'PostgreSQL 连接、会话、慢查询、锁、复制和容量的只读诊断。'),
    ('Redis 健康诊断', 'Redis 内存、热 Key 能力、慢命令、客户端和复制的只读诊断。'),
    ('Kafka 健康诊断', 'Kafka Broker、Topic、分区、ISR、消费组和积压的只读诊断。')
 ) AS item(name, summary)
 WHERE platform.code = 'default'
ON CONFLICT (scope_id, kind, lower(name)) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO skill_versions (skill_resource_id, version, manifest, input_schema, output_schema, tools, risk_level, status, published_at)
SELECT resource.id, 1,
       jsonb_build_object('name', resource.name, 'description', resource.config->>'summary', 'instruction', item.instruction, 'target_kinds', item.target_kinds::jsonb),
	       '{"type":"object","required":["facts","findings","evidence","hypotheses","confidence","recommendations"],"properties":{"facts":{"type":"object"},"findings":{"type":"array"},"evidence":{"type":"array"},"hypotheses":{"type":"array"},"confidence":{"type":"number","minimum":0,"maximum":1},"recommendations":{"type":"array"}},"additionalProperties":false}'::jsonb,
       '{"type":"object","additionalProperties":true}'::jsonb,
       item.tools::jsonb, 'read_only', 'published', now()
  FROM resources resource
  JOIN (VALUES
	    ('Kubernetes 工作负载诊断', '仅调用声明的 Kubernetes 只读工具。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations；将事实、异常和待验证假设分开。', '["Application","Kubernetes"]', '[{"name":"connector_kubernetes_read","description":"受限读取 Kubernetes 对象。","input_schema":{"type":"object","required":["target_resource_id","resource"],"properties":{"target_resource_id":{"type":"string"},"resource":{"type":"string"},"namespace":{"type":"string"},"name":{"type":"string"},"label_selector":{"type":"string"},"limit":{"type":"integer"}},"additionalProperties":false}}]'),
	    ('PostgreSQL 健康诊断', '只调用 PostgreSQL 固定诊断快照；不得请求写 SQL。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。', '["PostgreSQL"]', '[{"name":"connector_postgresql_inspect","description":"读取固定 PostgreSQL 健康快照。","input_schema":{"type":"object","required":["target_resource_id"],"properties":{"target_resource_id":{"type":"string"}},"additionalProperties":false}}]'),
	    ('Redis 健康诊断', '只调用 Redis 固定诊断快照；不得请求写命令。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。', '["Redis"]', '[{"name":"connector_redis_inspect","description":"读取固定 Redis 健康快照。","input_schema":{"type":"object","required":["target_resource_id"],"properties":{"target_resource_id":{"type":"string"}},"additionalProperties":false}}]'),
	    ('Kafka 健康诊断', '只调用 Kafka 固定诊断快照；不得请求管理操作。输出 JSON，且必须包含 facts、findings、evidence、hypotheses、confidence 和 recommendations。', '["Kafka"]', '[{"name":"connector_kafka_inspect","description":"读取固定 Kafka 健康快照。","input_schema":{"type":"object","required":["target_resource_id"],"properties":{"target_resource_id":{"type":"string"}},"additionalProperties":false}}]')
  ) AS item(name, instruction, target_kinds, tools) ON item.name = resource.name
 WHERE resource.kind = 'Skill' AND resource.config->>'owner' = 'OpsKeeper builtin'
ON CONFLICT (skill_resource_id, version) DO NOTHING;
