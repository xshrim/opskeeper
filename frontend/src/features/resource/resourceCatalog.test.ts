import { describe, expect, it } from 'vitest';
import type { Resource } from '../../lib/api';
import { resourceHasConnector } from '../../lib/resources';
import { brandNameFor, connectorCapabilityName, relativeConnectionTime, resourceEndpointFor, resourceCategoryOptions } from './resourceCatalog';

describe('resource catalog helpers', () => {
  it('normalizes provider brands for display', () => {
    expect(brandNameFor({ kind: 'AIProvider', config: { provider_type: 'openai_compatible' } })).toBe('OpenAI');
    expect(brandNameFor({ kind: 'Database', subtype: 'PostgreSQL' })).toBe('PostgreSQL');
    expect(brandNameFor({ kind: 'Datadog' })).toBe('Datadog');
    expect(brandNameFor({ kind: 'Unknown' })).toBe('');
  });

  it('recognizes provider brands regardless of casing and separators', () => {
    expect(brandNameFor({ kind: 'elasticsearch' })).toBe('ElasticSearch');
    expect(brandNameFor({ kind: 'Database', subtype: 'Apache Kafka' })).toBe('Kafka');
    expect(brandNameFor({ kind: 'AIProvider', config: { provider_type: 'OpenAI-Compatible' } })).toBe('OpenAI');
    expect(brandNameFor({ kind: 'Database', config: { provider: 'MongoDB' } })).toBe('MongoDB');
    expect(brandNameFor({ kind: 'AIProvider', config: { provider_type: 'gemini' } })).toBe('Gemini');
    expect(brandNameFor({ kind: 'Repository', config: { provider: 'gitea' } })).toBe('Gitea');
    expect(brandNameFor({ kind: 'k8s' })).toBe('Kubernetes');
    expect(brandNameFor({ kind: 'Database', subtype: 'postgres' })).toBe('PostgreSQL');
  });

  it('maps connector capabilities to user-facing labels', () => {
    expect(connectorCapabilityName('query_logs')).toBe('查询日志');
    expect(connectorCapabilityName('kubernetes_read')).toBe('读取 Kubernetes');
  });

  it('includes Docker Unix socket resources in connection checks', () => {
    expect(resourceHasConnector({
      id: 'docker-1',
      scope_id: 'scope-1',
      kind: 'Docker',
      schema_version: 1,
      name: 'local Docker',
      labels: {},
      config: { host: 'unix:///var/run/docker.sock' },
      status: 'active',
      created_at: '',
      updated_at: ''
    } satisfies Resource)).toBe(true);
  });

  it('formats the latest connection check age', () => {
    const now = Date.parse('2026-09-08T00:00:00Z');
    expect(relativeConnectionTime('2026-09-07T23:59:30Z', now)).toBe('刚刚');
    expect(relativeConnectionTime('2026-09-07T23:30:00Z', now)).toBe('30 分前');
    expect(relativeConnectionTime('2026-09-07T20:00:00Z', now)).toBe('4 时前');
    expect(relativeConnectionTime('2026-09-05T00:00:00Z', now)).toBe('3 天前');
    expect(relativeConnectionTime('2026-08-15T00:00:00Z', now)).toBe('3 周前');
    expect(relativeConnectionTime('2026-01-08T00:00:00Z', now)).toBe('8 月前');
    expect(relativeConnectionTime('2024-09-08T00:00:00Z', now)).toBe('2 年前');
  });

  it('formats direct Host endpoints as SSH URIs', () => {
    expect(resourceEndpointFor({ kind: 'Host', subtype: 'Direct', config: { host: '192.0.2.10' } })).toBe('ssh://192.0.2.10');
    expect(resourceEndpointFor({ kind: 'Host', subtype: 'Direct', config: { host: 'host.example', port: 2222 } })).toBe('ssh://host.example:2222');
  });

  it('keeps the requested resource directory order', () => {
    expect(Object.keys(resourceCategoryOptions)).toEqual(['全部','Application','Artifact','Repository','Host','Docker','Kubernetes','Nacos','Nginx','TongHttpServer','PostgreSQL','Oracle','MySQL','OceanBase','Redis','TongRDS','Kafka','RabbitMQ','ElasticSearch','LLM','MCPServer','Skill','Monitor']);
  });
});
