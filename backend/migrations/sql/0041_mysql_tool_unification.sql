UPDATE resource_schemas
SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string","minLength":1},"port":{"title":"端口","type":"integer","minimum":1,"maximum":65535,"default":3306},"database":{"title":"数据库","type":"string","minLength":1},"timeout_seconds":{"title":"超时时间（秒）","type":"integer","minimum":1,"maximum":300,"default":10}}}'::jsonb,
    display_name = 'MySQL', description = 'MySQL database resource with Direct/Agent access.', status = 'active'
WHERE kind = 'MySQL';

INSERT INTO resources (scope_id, kind, schema_version, name, config, status)
SELECT platform.scope_id, 'Skill', 2, 'MySQL 健康诊断', jsonb_build_object('summary', 'MySQL 数据库状态、性能、配置和结构的只读诊断。', 'owner', 'OpsKeeper builtin'), 'active'
  FROM platforms platform
 WHERE platform.code = 'default'
ON CONFLICT (scope_id, kind, lower(name)) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO skill_versions (skill_resource_id, version, manifest, input_schema, output_schema, tools, risk_level, status, published_at)
SELECT resource.id, 1,
       jsonb_build_object('name', resource.name, 'description', resource.config->>'summary', 'instruction', '调用 MySQL 固定只读工具；不要请求或建议写 SQL。', 'target_kinds', '["MySQL"]'::jsonb),
       '{"type":"object","required":["facts","findings","evidence","hypotheses","confidence","recommendations"],"properties":{"facts":{"type":"object"},"findings":{"type":"array"},"evidence":{"type":"array"},"hypotheses":{"type":"array"},"confidence":{"type":"number","minimum":0,"maximum":1},"recommendations":{"type":"array"}},"additionalProperties":false}'::jsonb,
       '{"type":"object","additionalProperties":true}'::jsonb,
       '[{"name":"mysql_health","description":"检查 MySQL 健康"},{"name":"mysql_status","description":"读取 MySQL 状态"},{"name":"mysql_performance","description":"读取性能与配置"},{"name":"mysql_tables","description":"读取用户表信息"},{"name":"mysql_table_columns","description":"读取指定表列信息"},{"name":"mysql_table_structure","description":"读取指定表结构和索引"},{"name":"mysql_database_info","description":"读取数据库概要"}]'::jsonb,
       'read_only', 'published', now()
  FROM resources resource
 WHERE resource.kind = 'Skill' AND resource.name = 'MySQL 健康诊断' AND resource.config->>'owner' = 'OpsKeeper builtin'
ON CONFLICT (skill_resource_id, version) DO NOTHING;
