UPDATE resource_schemas
   SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["url"],"properties":{"url":{"title":"服务 URL","type":"string","format":"uri"},"username":{"title":"用户名","type":"string","sensitive":true},"password":{"title":"密码","type":"string","sensitive":true},"token":{"title":"访问 Token","type":"string","sensitive":true}}}'::jsonb,
       display_name = 'Prometheus',
       description = 'Prometheus 指标查询和告警来源。',
       icon = 'metrics'
 WHERE kind = 'Prometheus';

UPDATE resource_schemas
   SET schema = '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["url"],"properties":{"url":{"title":"服务 URL","type":"string","format":"uri"},"tenant_id":{"title":"租户 ID","type":"string"},"username":{"title":"用户名","type":"string","sensitive":true},"password":{"title":"密码","type":"string","sensitive":true},"token":{"title":"访问 Token","type":"string","sensitive":true}}}'::jsonb,
       display_name = 'Loki',
       description = 'Loki 日志查询来源。',
       icon = 'logs'
 WHERE kind = 'Loki';

CREATE TABLE resource_connection_checks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('succeeded', 'failed')),
    error_category text NOT NULL DEFAULT '' CHECK (
        error_category IN ('', 'configuration', 'authentication', 'timeout', 'rate_limited', 'response_too_large', 'upstream', 'unsupported', 'internal')
    ),
    message text NOT NULL DEFAULT '' CHECK (length(message) <= 500),
    latency_ms bigint NOT NULL DEFAULT 0 CHECK (latency_ms >= 0),
    capabilities text[] NOT NULL DEFAULT '{}',
    checked_by uuid REFERENCES users(id) ON DELETE SET NULL,
    checked_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX resource_connection_checks_latest_idx
    ON resource_connection_checks(resource_id, checked_at DESC, id DESC);
