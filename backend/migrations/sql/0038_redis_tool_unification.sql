UPDATE resource_schemas
SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string","minLength":1},"port":{"title":"端口","type":"integer","minimum":1,"maximum":65535,"default":6379},"database":{"title":"数据库编号","type":"integer","minimum":0,"default":0},"timeout_seconds":{"title":"超时时间（秒）","type":"integer","minimum":1,"maximum":300,"default":10}}}'::jsonb,
    display_name='Redis', description='Redis resource with unified Direct/Agent read-only tools.'
WHERE kind='Redis';

-- Skill versions are immutable. Publish a new version instead of changing the
-- version that was installed by the initial migration.
INSERT INTO skill_versions (skill_resource_id, version, manifest, input_schema, output_schema, tools, risk_level, status, published_at)
SELECT current.skill_resource_id,
       current.version + 1,
       current.manifest,
       current.input_schema,
       current.output_schema,
       '[{"name":"redis_health","description":"检查 Redis 健康"},{"name":"redis_memory","description":"读取内存统计"},{"name":"redis_clients","description":"读取客户端统计"},{"name":"redis_replication","description":"读取复制状态"},{"name":"redis_slowlog","description":"读取慢日志"},{"name":"redis_database_info","description":"读取数据库信息"}]'::jsonb,
       current.risk_level,
       'published',
       now()
  FROM skill_versions current
  JOIN resources skill ON skill.id = current.skill_resource_id
 WHERE skill.name = 'Redis 健康诊断'
   AND current.version = (
       SELECT max(previous.version)
         FROM skill_versions previous
        WHERE previous.skill_resource_id = current.skill_resource_id
   )
ON CONFLICT (skill_resource_id, version) DO NOTHING;

UPDATE skill_scope_defaults defaults
   SET skill_version_id = current.id,
       updated_at = now()
  FROM skill_versions current
  JOIN resources skill ON skill.id = current.skill_resource_id
 WHERE defaults.skill_resource_id = current.skill_resource_id
   AND skill.name = 'Redis 健康诊断'
   AND current.version = (
       SELECT max(previous.version)
         FROM skill_versions previous
        WHERE previous.skill_resource_id = current.skill_resource_id
   );

UPDATE skill_versions current
   SET status = 'disabled'
  FROM resources skill
 WHERE skill.id = current.skill_resource_id
   AND skill.name = 'Redis 健康诊断'
   AND current.status = 'published'
   AND current.version < (
       SELECT max(latest.version)
         FROM skill_versions latest
        WHERE latest.skill_resource_id = current.skill_resource_id
   );
