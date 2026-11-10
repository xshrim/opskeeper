import { describe, expect, it } from 'vitest';
import { resourceToolsFor } from './resourceTools';

describe('resourceToolsFor', () => {
  it('returns a purpose-specific description for every registered tool', () => {
    const kinds = [
      'Docker',
      'Kubernetes',
      'Host',
      'PostgreSQL',
      'MySQL',
      'Oracle',
      'OceanBase',
      'RabbitMQ',
      'MinIO',
      'Redis',
      'Nacos',
      'Kafka',
      'Elasticsearch',
      'Repository'
    ];

    for (const kind of kinds) {
      const tools = resourceToolsFor(kind);
      expect(tools.length, kind).toBeGreaterThan(0);
      expect(
        tools.every(
          ({ name, description }) =>
            Boolean(name.trim()) &&
            Boolean(description.trim()) &&
            description !== '只读诊断工具'
        ),
        kind
      ).toBe(true);
    }
  });

  it('returns no tools for an unsupported resource kind', () => {
    expect(resourceToolsFor('Unsupported')).toEqual([]);
  });
});
