UPDATE resource_schemas
SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string","minLength":1},"port":{"title":"端口","type":"integer","minimum":1,"maximum":65535,"default":5432},"database":{"title":"数据库","type":"string","minLength":1},"timeout_seconds":{"title":"超时时间（秒）","type":"integer","minimum":1,"maximum":300,"default":10}}}'::jsonb,
    display_name = 'PostgreSQL', description = 'PostgreSQL database resource with Direct/Agent access.'
WHERE kind = 'PostgreSQL';

-- Skill versions are immutable. Publish a new version instead of changing the
-- version that was installed by the initial migration.
INSERT INTO skill_versions (skill_resource_id, version, manifest, input_schema, output_schema, tools, risk_level, status, published_at)
SELECT current.skill_resource_id,
       current.version + 1,
       current.manifest,
       current.input_schema,
       current.output_schema,
       '[{"name":"postgresql_health","description":"检查 PostgreSQL 健康"},{"name":"postgresql_sessions","description":"读取活跃会话"},{"name":"postgresql_long_running_queries","description":"读取长时间运行查询"},{"name":"postgresql_locks","description":"读取等待锁"},{"name":"postgresql_replication","description":"读取复制状态"},{"name":"postgresql_capacity","description":"读取数据库容量"},{"name":"postgresql_tables","description":"读取用户表统计"},{"name":"postgresql_performance","description":"读取性能与配置"},{"name":"postgresql_vacuum","description":"读取 VACUUM 配置"},{"name":"postgresql_extensions","description":"读取扩展信息"},{"name":"postgresql_database_info","description":"读取数据库概要"},{"name":"postgresql_table_columns","description":"读取指定用户表列信息"}]'::jsonb,
       current.risk_level,
       'published',
       now()
  FROM skill_versions current
  JOIN resources skill ON skill.id = current.skill_resource_id
 WHERE skill.name = 'PostgreSQL 健康诊断'
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
   AND skill.name = 'PostgreSQL 健康诊断'
   AND current.version = (
       SELECT max(previous.version)
         FROM skill_versions previous
        WHERE previous.skill_resource_id = current.skill_resource_id
   );

UPDATE skill_versions current
   SET status = 'disabled'
  FROM resources skill
 WHERE skill.id = current.skill_resource_id
   AND skill.name = 'PostgreSQL 健康诊断'
   AND current.status = 'published'
   AND current.version < (
       SELECT max(latest.version)
         FROM skill_versions latest
        WHERE latest.skill_resource_id = current.skill_resource_id
   );
