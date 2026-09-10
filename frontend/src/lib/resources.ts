import type { Resource, ResourceSchema } from './api';
import { iconGlyph } from './icons';

type ResourceShape = {
  kind: string;
  subtype?: string;
  config?: Record<string, unknown>;
};

export const endpointResourceKinds = ['Kubernetes', 'Prometheus', 'Loki', 'PostgreSQL', 'Redis', 'Kafka'] as const;

export function resourceSupportsEndpointTimeout(kind: string) {
  return (endpointResourceKinds as readonly string[]).includes(kind);
}

export function resourceHasConnector(resource: Resource) {
  if (resource.kind === 'MCPServer') return true;
  if (resource.kind === 'Docker' || resource.kind === 'Kubernetes') {
    // The connector endpoint only performs Direct checks. Agent resources
    // are exercised through their linked MCPServer and must not fall back to
    // a local or default endpoint.
    return String(resource.subtype ?? '').toLowerCase() !== 'agent';
  }
  return ['AIProvider', 'Kubernetes', 'Prometheus', 'Loki', 'PostgreSQL', 'Redis', 'Kafka'].includes(resource.kind);
}

export function resourceSchemaName(kind: string, schemas: ResourceSchema[]) {
  if (kind === 'AIProvider') return 'Provider';
  const schema = schemas.find((item) => item.kind === kind);
  return schema?.display_name || schema?.kind || kind;
}

export function resourceIcon(kind: string, schemas: ResourceSchema[]) {
  return iconGlyph(schemas.find((item) => item.kind === kind)?.icon);
}

export function brandNameFor(resource: ResourceShape) {
  const normalizedNames: Record<string, string> = {
    redis: 'Redis', kafka: 'Kafka', apachekafka: 'Kafka', rabbitmq: 'RabbitMQ',
    elastic: 'Elastic', elasticsearch: 'ElasticSearch', elasticstack: 'Elastic',
    datadog: 'Datadog', jaeger: 'Jaeger', postgresql: 'PostgreSQL', postgres: 'PostgreSQL',
    mysql: 'MySQL', docker: 'Docker', kubernetes: 'Kubernetes', kubernetescluster: 'Kubernetes', k8s: 'Kubernetes', git: 'Git',
    github: 'GitHub', gitlab: 'GitLab', helm: 'Helm', prometheus: 'Prometheus',
    grafana: 'Grafana', openai: 'OpenAI', openaicompatible: 'OpenAI',
    anthropic: 'Anthropic', deepseek: 'DeepSeek', qwen: 'Qwen', ollama: 'Ollama',
    gemini: 'Gemini', googlegemini: 'Gemini', kimi: 'Kimi', minimax: 'MiniMax', minimaxai: 'MiniMax', openrouter: 'OpenRouter',
    moonshot: 'Moonshot', moonshotai: 'Moonshot',
    minio: 'MinIO', mongodb: 'MongoDB', gitea: 'Gitea', bitbucket: 'Bitbucket',
    harbor: 'Harbor'
  };
  const candidates = [
    resource.subtype,
    resource.config?.subtype,
    resource.config?.provider,
    resource.config?.provider_type,
    resource.kind
  ];
  for (const candidate of candidates) {
    const normalized = String(candidate ?? '').toLowerCase().replace(/[^a-z0-9]/g, '');
    const brand = normalizedNames[normalized];
    if (brand) return brand;
  }
  return '';
}
