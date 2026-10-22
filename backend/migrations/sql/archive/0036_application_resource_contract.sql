-- Application resources are project-owned and use an explicit instance model.
INSERT INTO resource_schemas (kind, version, schema, status, display_name, description, icon)
VALUES (
    'Application', 2,
    '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true,"required":["access_mode","instances"],"properties":{"access_mode":{"title":"接入方式","type":"string","enum":["virtual_machine","containerized","cloud_native"]},"instances":{"title":"应用实例","type":"array","minItems":1,"items":{"type":"object","additionalProperties":true,"description":"虚拟机实例使用 process_keyword；容器化实例使用 container_name；云原生实例使用 namespace、workload_kind 和 workload_name。"}}}}'::jsonb,
    'active', 'Application', '归属于项目并由 Host、Docker 或 Kubernetes 实例组成的业务应用。', 'application'
)
ON CONFLICT (kind, version) DO UPDATE SET
    schema = EXCLUDED.schema,
    status = EXCLUDED.status,
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    icon = EXCLUDED.icon;

UPDATE resource_schemas
   SET status = 'disabled'
 WHERE kind = 'Application' AND version = 1;
