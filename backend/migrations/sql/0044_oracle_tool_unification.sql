-- Oracle tool unification uses the pure-Go go-ora driver; no Oracle Instant Client is required.
UPDATE resource_schemas
SET schema='{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string","minLength":1},"port":{"title":"端口","type":"integer","minimum":1,"maximum":65535,"default":1521},"service_name":{"title":"Service Name","type":"string"},"sid":{"title":"SID","type":"string"},"tls":{"title":"启用 TCPS","type":"boolean","default":false},"timeout_seconds":{"title":"超时时间（秒）","type":"integer","minimum":1,"maximum":300,"default":10}}}'::jsonb,display_name='Oracle',description='Oracle database resource with unified Direct/Agent access.',status='active'
WHERE kind='Oracle';

INSERT INTO resources(scope_id,kind,schema_version,name,config,status)
SELECT p.scope_id,'Skill',2,'Oracle 健康诊断',jsonb_build_object('summary','Oracle 数据库状态、性能、配置与表结构的只读诊断。','owner','OpsKeeper builtin'),'active' FROM platforms p WHERE p.code='default'
ON CONFLICT (scope_id,kind,lower(name)) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO skill_versions(skill_resource_id,version,manifest,input_schema,output_schema,tools,risk_level,status,published_at)
SELECT r.id,COALESCE((SELECT max(v.version)+1 FROM skill_versions v WHERE v.skill_resource_id=r.id),1),jsonb_build_object('name',r.name,'description',r.config->>'summary','instruction','调用 Oracle 固定只读工具；不要请求或建议写入 SQL。','target_kinds','["Oracle"]'::jsonb),'{}'::jsonb,'{}'::jsonb,'[{"name":"oracle_health"},{"name":"oracle_status"},{"name":"oracle_performance"},{"name":"oracle_database_info"},{"name":"oracle_tables"},{"name":"oracle_table_columns"},{"name":"oracle_table_structure"}]'::jsonb,'read_only','published',now() FROM resources r WHERE r.kind='Skill' AND r.name='Oracle 健康诊断' AND r.config->>'owner'='OpsKeeper builtin'
ON CONFLICT (skill_resource_id,version) DO NOTHING;
