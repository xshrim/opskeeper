import { describe, expect, it } from 'vitest';
import { brandNameFor, connectorCapabilityName } from './resourceCatalog';

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
});
