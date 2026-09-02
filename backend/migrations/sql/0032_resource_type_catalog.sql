-- Resource type catalog v2: concrete platforms are first-class resources and
-- connection mode is represented by the Direct/Agent subtype.

-- Retire the former catch-all resource kinds. New resources must use one of
-- the concrete kinds below; connection-mode labels are normalized below.
UPDATE resource_schemas
   SET status = 'disabled'
 WHERE kind IN ('GenericMiddleware', 'GenericAPI', 'Database');

UPDATE resource_schemas
   SET display_name = 'Application',
       description = 'Application resource.',
       icon = 'application'
 WHERE kind = 'Application';

UPDATE resource_schemas
   SET display_name = 'Artifact',
       description = 'Artifact repository resource.',
       icon = 'storage'
 WHERE kind = 'Artifact';

UPDATE resource_schemas
   SET display_name = 'Repository',
       description = 'Source code repository resource.',
       icon = 'repository'
 WHERE kind = 'Repository';

-- The generic schema is intentionally permissive: connector-specific
-- validation belongs to the corresponding connector, while subtype selects
-- whether the connection is Direct or Agent based.
INSERT INTO resource_schemas (kind, version, schema, status, display_name, description, icon)
VALUES
 ('Host', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'Host', 'Host connection resource.', 'host'),
 ('Docker', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'Docker', 'Docker Engine resource.', 'docker'),
 ('Kubernetes', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'Kubernetes', 'Kubernetes cluster resource.', 'kubernetes'),
 ('Redis', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'Redis', 'Redis resource.', 'redis'),
 ('TongRDS', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'TongRDS', 'TongRDS database resource.', 'database'),
 ('Kafka', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'Kafka', 'Kafka resource.', 'kafka'),
 ('RabbitMQ', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'RabbitMQ', 'RabbitMQ resource.', 'rabbitmq'),
 ('Elasticsearch', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'Elasticsearch', 'Elasticsearch resource.', 'search'),
 ('OceanBase', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'OceanBase', 'OceanBase database resource.', 'database'),
 ('Oracle', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'Oracle', 'Oracle database resource.', 'database'),
 ('MySQL', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'MySQL', 'MySQL database resource.', 'database'),
 ('PostgreSQL', 1, '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb, 'active', 'PostgreSQL', 'PostgreSQL database resource.', 'postgresql')
ON CONFLICT (kind, version) DO UPDATE SET
  status = EXCLUDED.status,
  display_name = EXCLUDED.display_name,
  description = EXCLUDED.description,
  icon = EXCLUDED.icon;

-- Explicitly retire the old Kubernetes schema alias after the canonical kind
-- has been ensured above.
UPDATE resource_schemas SET status = 'disabled' WHERE kind = 'KubernetesCluster';

UPDATE resources
   SET subtype = CASE
         WHEN lower(btrim(subtype)) = 'api' THEN 'Direct'
         WHEN btrim(subtype) <> '' THEN btrim(subtype)
         WHEN config ->> 'subtype' IN ('Direct', 'Agent') THEN config ->> 'subtype'
         ELSE 'Direct'
       END,
       config = CASE
         WHEN jsonb_typeof(config) = 'object'
              AND lower(btrim(COALESCE(config ->> 'subtype', ''))) IN ('', 'api')
           THEN (config - 'subtype') || jsonb_build_object('subtype', 'Direct')
         ELSE config
       END,
       updated_at = now()
 WHERE kind IN ('Host', 'Docker', 'Kubernetes', 'Redis', 'TongRDS', 'Kafka',
                'RabbitMQ', 'Elasticsearch', 'OceanBase', 'Oracle', 'MySQL', 'PostgreSQL')
   AND lower(btrim(subtype)) IN ('', 'api');
