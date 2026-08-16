DROP TABLE IF EXISTS resource_connection_checks;

UPDATE resource_schemas
   SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb,
       display_name = 'Prometheus',
       description = 'Prometheus 指标服务连接配置。',
       icon = 'metrics'
 WHERE kind = 'Prometheus';

UPDATE resource_schemas
   SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}'::jsonb,
       display_name = 'Loki',
       description = 'Loki 日志服务连接配置。',
       icon = 'logs'
 WHERE kind = 'Loki';
