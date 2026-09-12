UPDATE resource_type_catalog
SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string","minLength":1},"port":{"title":"端口","type":"integer","minimum":1,"maximum":65535,"default":6379},"database":{"title":"数据库编号","type":"integer","minimum":0,"default":0},"timeout_seconds":{"title":"超时时间（秒）","type":"integer","minimum":1,"maximum":300,"default":10}}}'::jsonb,
    display_name='Redis', description='Redis resource with unified Direct/Agent read-only tools.'
WHERE kind='Redis';

UPDATE skill_versions version
SET tools='[{"name":"redis_health","description":"检查 Redis 健康"},{"name":"redis_memory","description":"读取内存统计"},{"name":"redis_clients","description":"读取客户端统计"},{"name":"redis_replication","description":"读取复制状态"},{"name":"redis_slowlog","description":"读取慢日志"},{"name":"redis_database_info","description":"读取数据库信息"}]'::jsonb
FROM resources skill
WHERE skill.id=version.skill_resource_id AND skill.name='Redis 健康诊断';
