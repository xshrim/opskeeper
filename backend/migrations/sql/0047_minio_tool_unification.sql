INSERT INTO resources(scope_id,kind,schema_version,name,config,status)
SELECT p.scope_id,'Skill',2,'MinIO 健康诊断',jsonb_build_object('summary','MinIO 连接、存储桶、对象和桶配置的只读诊断。','owner','OpsKeeper builtin'),'active'
FROM platforms p WHERE p.code='default'
ON CONFLICT (scope_id,kind,lower(name)) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO skill_versions(skill_resource_id,version,manifest,input_schema,output_schema,tools,risk_level,status,published_at)
SELECT r.id,COALESCE((SELECT max(v.version)+1 FROM skill_versions v WHERE v.skill_resource_id=r.id),1),
jsonb_build_object('name',r.name,'description',r.config->>'summary','instruction','调用 MinIO 固定只读工具；不要执行写入、删除或管理操作。','target_kinds','["MinIO"]'::jsonb),
'{}'::jsonb,'{}'::jsonb,
'[{"name":"minio_health"},{"name":"minio_buckets"},{"name":"minio_bucket_objects"},{"name":"minio_object_stat"},{"name":"minio_bucket_versioning"},{"name":"minio_bucket_lifecycle"}]'::jsonb,
'read_only','published',now()
FROM resources r WHERE r.kind='Skill' AND r.name='MinIO 健康诊断' AND r.config->>'owner'='OpsKeeper builtin'
ON CONFLICT (skill_resource_id,version) DO NOTHING;
