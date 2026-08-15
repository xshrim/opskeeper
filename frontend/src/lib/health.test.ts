import { describe, expect, it } from 'vitest';
import { appURL, toStatusRows, type HealthReport } from './health';

describe('toStatusRows', () => {
  it('maps dependency health to stable status rows', () => {
    const report: HealthReport = {
      status: 'not_ready',
      service: 'opskeeper-api',
      version: 'test',
      timestamp: '2026-01-01T00:00:00Z',
      checks: {
        postgres: { status: 'up', latency_ms: 4 },
        redis: { status: 'down', latency_ms: 10 }
      }
    };

    expect(toStatusRows(report)).toEqual([
      { name: 'API', status: 'down' },
      { name: 'PostgreSQL', status: 'up', latency: '4 ms' },
      { name: 'Redis', status: 'down', latency: '10 ms' }
    ]);
  });
});

describe('appURL', () => {
  it('resolves application paths from the document base URI', () => {
    const base = document.createElement('base');
    base.href = 'http://localhost:5173/opskeeper/';
    document.head.append(base);

    expect(appURL('/api/v1/teams').toString()).toBe(
      'http://localhost:5173/opskeeper/api/v1/teams'
    );

    base.remove();
  });

  it('resolves application paths from a root base URI', () => {
    const base = document.createElement('base');
    base.href = 'http://localhost:5173/';
    document.head.append(base);

    expect(appURL('/api/v1/teams').toString()).toBe(
      'http://localhost:5173/api/v1/teams'
    );

    base.remove();
  });
});
