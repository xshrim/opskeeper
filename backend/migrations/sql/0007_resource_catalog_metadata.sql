ALTER TABLE resource_schemas
    ADD COLUMN display_name text NOT NULL DEFAULT '',
    ADD COLUMN description text NOT NULL DEFAULT '',
    ADD COLUMN icon text NOT NULL DEFAULT 'resource';

UPDATE resource_schemas
   SET display_name = metadata.display_name,
       description = metadata.description,
       icon = metadata.icon
  FROM (VALUES
    ('KubernetesCluster', 'Kubernetes 集群', '连接并管理 Kubernetes 集群的入口资源。', 'kubernetes'),
    ('BusinessApplication', '业务应用', '面向业务的应用或服务集合。', 'application'),
    ('Endpoint', '服务端点', '业务应用对外提供的访问端点。', 'endpoint'),
    ('CronApplication', '定时应用', '按计划运行的批处理或定时任务应用。', 'schedule'),
    ('PostgreSQL', 'PostgreSQL 数据库', 'PostgreSQL 数据库连接配置。', 'postgresql'),
    ('Redis', 'Redis', 'Redis 缓存或数据服务连接配置。', 'redis'),
    ('Kafka', 'Kafka 集群', 'Kafka 消息集群连接配置。', 'kafka'),
    ('Elasticsearch', 'Elasticsearch', 'Elasticsearch 搜索服务连接配置。', 'search'),
    ('GenericMiddleware', '通用中间件', '无法归入具体类型的中间件或平台服务。', 'middleware'),
    ('LLMProvider', '大模型服务', 'OpenAI 兼容或其他大模型服务提供方。', 'llm'),
    ('MCPServer', 'MCP 服务', '提供受控工具能力的 MCP 服务。', 'mcp'),
    ('Skill', '诊断技能', '可被受控 Runner 调用的诊断技能。', 'skill'),
    ('Prometheus', 'Prometheus', 'Prometheus 指标服务连接配置。', 'metrics'),
    ('Loki', 'Loki', 'Loki 日志服务连接配置。', 'logs'),
    ('Tempo', 'Tempo', 'Tempo 链路追踪服务连接配置。', 'traces'),
    ('Jaeger', 'Jaeger', 'Jaeger 链路追踪服务连接配置。', 'traces'),
    ('Elastic', 'Elastic Observability', 'Elastic 可观测性服务连接配置。', 'search'),
    ('Datadog', 'Datadog', 'Datadog 可观测性服务连接配置。', 'observability'),
    ('GenericAPI', '通用 API', '可通过 HTTP 访问的外部 API。', 'api'),
    ('NotificationChannel', '通知渠道', 'Webhook、邮件或其他通知目标。', 'notification'),
    ('Runbook', '运行手册', '可供运维流程引用的运行手册。', 'runbook'),
    ('ArtifactStore', '制品存储', '保存诊断报告和其他制品的存储服务。', 'storage'),
    ('Credential', '连接凭据', '已由独立凭据模型管理，不应作为资源登记。', 'credential')
  ) AS metadata(kind, display_name, description, icon)
 WHERE resource_schemas.kind = metadata.kind;

UPDATE resource_schemas
   SET schema = CASE kind
       WHEN 'KubernetesCluster' THEN '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"kubeconfig":{"title":"kubeconfig","type":"string","sensitive":true},"context":{"title":"Context","type":"string"},"api_server":{"title":"API Server","type":"string"}}}'::jsonb
       WHEN 'PostgreSQL' THEN '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string"},"port":{"title":"端口","type":"integer"},"database":{"title":"数据库","type":"string"},"username":{"title":"用户名","type":"string"},"password":{"title":"密码","type":"string","sensitive":true}}}'::jsonb
       WHEN 'Redis' THEN '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"host":{"title":"主机","type":"string"},"port":{"title":"端口","type":"integer"},"database":{"title":"数据库编号","type":"integer"},"username":{"title":"用户名","type":"string"},"password":{"title":"密码","type":"string","sensitive":true}}}'::jsonb
       WHEN 'Kafka' THEN '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"brokers":{"title":"Broker 地址","type":"array"},"username":{"title":"用户名","type":"string"},"password":{"title":"密码","type":"string","sensitive":true},"tls":{"title":"启用 TLS","type":"boolean"}}}'::jsonb
       WHEN 'LLMProvider' THEN '{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{"url":{"title":"服务 URL","type":"string"},"model":{"title":"默认模型","type":"string"},"token":{"title":"访问 Token","type":"string","sensitive":true}}}'::jsonb
       ELSE schema
   END
 WHERE kind IN ('KubernetesCluster', 'PostgreSQL', 'Redis', 'Kafka', 'LLMProvider');

UPDATE resource_schemas
   SET status = 'disabled'
 WHERE kind IN ('Namespace', 'Node', 'Workload', 'Pod', 'Service', 'Ingress', 'Model', 'Credential');

