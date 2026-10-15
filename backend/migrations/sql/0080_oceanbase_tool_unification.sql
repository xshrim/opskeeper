-- OceanBase uses a single read-only tool set for MySQL and Oracle compatibility modes.
UPDATE resource_schemas
SET schema='{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string","minLength":1},"port":{"title":"端口","type":"integer","minimum":1,"maximum":65535,"default":2881},"mode":{"title":"兼容模式","type":"string","enum":["mysql","oracle"],"default":"mysql"},"database":{"title":"数据库名","type":"string"},"service_name":{"title":"Service Name","type":"string"},"timeout_seconds":{"title":"超时时间（秒）","type":"integer","minimum":1,"maximum":300,"default":10}}}'::jsonb,
    display_name='OceanBase',
    description='OceanBase database resource with unified MySQL and Oracle mode access.',
    status='active'
WHERE kind='OceanBase';

INSERT INTO resources(scope_id,kind,schema_version,name,config,status)
SELECT p.scope_id,'Skill',2,'OceanBase 健康诊断',jsonb_build_object('summary','OceanBase MySQL/Oracle 模式的状态、性能、配置与表结构只读诊断。','owner','OpsKeeper builtin'),'active'
FROM platforms p
WHERE p.code='default'
ON CONFLICT (scope_id,kind,lower(name)) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO skill_versions(skill_resource_id,version,manifest,input_schema,output_schema,tools,risk_level,status,published_at)
SELECT r.id,
       COALESCE((SELECT max(v.version)+1 FROM skill_versions v WHERE v.skill_resource_id=r.id),1),
       jsonb_build_object('name',r.name,'description',r.config->>'summary','instruction','调用 OceanBase 固定只读工具；不要请求或建议写入 SQL。','target_kinds','["OceanBase"]'::jsonb),
       '{}'::jsonb,
       '{}'::jsonb,
       '[{"name":"oceanbase_health"},{"name":"oceanbase_status"},{"name":"oceanbase_performance"},{"name":"oceanbase_database_info"},{"name":"oceanbase_tables"},{"name":"oceanbase_table_columns"},{"name":"oceanbase_table_structure"}]'::jsonb,
       'read_only','published',now()
FROM resources r
WHERE r.kind='Skill' AND r.name='OceanBase 健康诊断' AND r.config->>'owner'='OpsKeeper builtin'
ON CONFLICT (skill_resource_id,version) DO NOTHING;
