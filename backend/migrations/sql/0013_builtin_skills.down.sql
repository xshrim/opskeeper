DELETE FROM resources
 WHERE kind = 'Skill'
   AND config->>'owner' = 'OpsKeeper builtin'
   AND name IN ('Kubernetes 工作负载诊断', 'PostgreSQL 健康诊断', 'Redis 健康诊断', 'Kafka 健康诊断');
