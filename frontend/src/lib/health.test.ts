import { describe, expect, it } from 'vitest';
import { toStatusRows, type HealthReport } from './health';

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
