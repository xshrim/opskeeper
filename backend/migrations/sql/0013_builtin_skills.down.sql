DELETE FROM skill_versions AS version
 USING resources AS resource
 WHERE version.skill_resource_id = resource.id
   AND resource.kind = 'Skill'
   AND resource.config->>'owner' = 'OpsKeeper builtin';

DELETE FROM resources
 WHERE kind = 'Skill'
   AND config->>'owner' = 'OpsKeeper builtin'
   AND name IN ('Kubernetes 工作负载诊断', 'PostgreSQL 健康诊断', 'Redis 健康诊断', 'Kafka 健康诊断');
