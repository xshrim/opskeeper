-- OceanBase uses a single read-only tool set for MySQL and Oracle compatibility modes.
UPDATE resource_schemas
SET schema='{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string","minLength":1},"port":{"title":"端口","type":"integer","minimum":1,"maximum":65535,"default":2881},"mode":{"title":"兼容模式","type":"string","enum":["mysql","oracle"],"default":"mysql"},"database":{"title":"数据库名","type":"string"},"service_name":{"title":"Service Name","type":"string"},"timeout_seconds":{"title":"超时时间（秒）","type":"integer","minimum":1,"maximum":300,"default":10}}}'::jsonb,
    display_name='OceanBase',
    description='OceanBase database resource with unified MySQL and Oracle mode access.',
    status='active'
WHERE kind='OceanBase';

CREATE OR REPLACE FUNCTION validate_skill_version()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM skills WHERE id = NEW.skill_id AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'skill version must belong to an active Skill' USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'UPDATE' AND (
        NEW.skill_id <> OLD.skill_id OR NEW.version <> OLD.version
        OR NEW.manifest <> OLD.manifest OR NEW.input_schema <> OLD.input_schema
        OR NEW.output_schema <> OLD.output_schema OR NEW.tools <> OLD.tools
        OR NEW.risk_level <> OLD.risk_level
    ) THEN
        RAISE EXCEPTION 'skill version content is immutable' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION validate_skill_scope_default()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    skill_scope uuid;
    skill_status text;
    version_skill uuid;
    version_status text;
BEGIN
    SELECT scope_id, status INTO skill_scope, skill_status
      FROM skills WHERE id = NEW.skill_id AND deleted_at IS NULL;
    SELECT skill_id, status INTO version_skill, version_status
      FROM skill_versions WHERE id = NEW.skill_version_id;
    IF skill_status IS DISTINCT FROM 'active'
       OR version_skill IS DISTINCT FROM NEW.skill_id
       OR version_status IS DISTINCT FROM 'published'
       OR NOT resource_scope_contains(skill_scope, NEW.scope_id) THEN
        RAISE EXCEPTION 'default Skill version is unavailable from scope' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;

INSERT INTO skills(id,scope_id,name,identifier,category,maintainer,status)
SELECT gen_random_uuid(),p.scope_id,'OceanBase 健康诊断','oceanbase-health-diagnosis','diagnosis','OpsKeeper builtin','active'
FROM platforms p
WHERE p.code='default'
ON CONFLICT (scope_id,lower(identifier)) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO skill_versions(skill_id,version,manifest,input_schema,output_schema,tools,risk_level,status,published_at)
SELECT s.id,
       COALESCE((SELECT max(v.version)+1 FROM skill_versions v WHERE v.skill_id=s.id),1),
       jsonb_build_object('name',s.name,'description','OceanBase MySQL/Oracle 模式的状态、性能、配置与表结构只读诊断。','instruction','调用 OceanBase 固定只读工具；不要请求或建议写入 SQL。','target_kinds','["OceanBase"]'::jsonb),
       '{}'::jsonb,
       '{}'::jsonb,
       '[{"name":"oceanbase_health"},{"name":"oceanbase_status"},{"name":"oceanbase_performance"},{"name":"oceanbase_database_info"},{"name":"oceanbase_tables"},{"name":"oceanbase_table_columns"},{"name":"oceanbase_table_structure"}]'::jsonb,
       'read_only','published',now()
FROM skills s
WHERE s.identifier='oceanbase-health-diagnosis' AND s.maintainer='OpsKeeper builtin' AND s.deleted_at IS NULL
ON CONFLICT (skill_id,version) DO NOTHING;
