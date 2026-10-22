UPDATE teams
SET icon = CASE WHEN icon = 'lucide:UsersRound' THEN 'team' ELSE regexp_replace(icon, '^lucide:', '') END;

UPDATE projects
SET icon = CASE WHEN icon = 'lucide:FolderKanban' THEN 'project' ELSE regexp_replace(icon, '^lucide:', '') END;

UPDATE platforms
SET icon = CASE WHEN icon = 'lucide:Building2' THEN 'platform' ELSE regexp_replace(icon, '^lucide:', '') END;

UPDATE applications SET icon = regexp_replace(icon, '^lucide:', '');
UPDATE engines SET icon = regexp_replace(icon, '^lucide:', '');

UPDATE providers
SET config = jsonb_set(config, '{icon}', to_jsonb(regexp_replace(config->>'icon', '^lucide:', '')), true)
WHERE jsonb_typeof(config) = 'object' AND config ? 'icon';

UPDATE resource_schemas
SET icon = CASE kind
  WHEN 'Kubernetes' THEN 'kubernetes'
  WHEN 'KubernetesCluster' THEN 'kubernetes'
  WHEN 'PostgreSQL' THEN 'postgresql'
  WHEN 'Redis' THEN 'redis'
  WHEN 'Kafka' THEN 'kafka'
  WHEN 'RabbitMQ' THEN 'rabbitmq'
  WHEN 'Docker' THEN 'docker'
  WHEN 'MinIO' THEN 'minio'
  WHEN 'MongoDB' THEN 'mongodb'
  WHEN 'Elasticsearch' THEN 'elasticsearch'
  WHEN 'Elastic' THEN 'elastic'
  WHEN 'Application' THEN 'application'
  WHEN 'Artifact' THEN 'artifact'
  WHEN 'Repository' THEN 'repository'
  WHEN 'Host' THEN 'host'
  WHEN 'Nacos' THEN 'nacos'
  WHEN 'LLM' THEN 'llm'
  WHEN 'LLMProvider' THEN 'llm'
  WHEN 'AIEngine' THEN 'engine'
  WHEN 'MCPServer' THEN 'mcp'
  WHEN 'Skill' THEN 'skill'
  WHEN 'Prometheus' THEN 'metrics'
  WHEN 'Loki' THEN 'logs'
  WHEN 'Tempo' THEN 'traces'
  WHEN 'Jaeger' THEN 'traces'
  WHEN 'Datadog' THEN 'observability'
  WHEN 'Endpoint' THEN 'endpoint'
  WHEN 'NotificationChannel' THEN 'notification'
  WHEN 'Runbook' THEN 'runbook'
  WHEN 'ArtifactStore' THEN 'storage'
  ELSE icon
END;
