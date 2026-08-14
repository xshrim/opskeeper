export type CheckStatus = 'up' | 'down' | 'unknown';

export interface DependencyCheck {
  status: CheckStatus;
  latency_ms?: number;
  error?: string;
}

export interface HealthReport {
  status: 'ready' | 'not_ready';
  service: string;
  version: string;
  timestamp: string;
  checks: Record<string, DependencyCheck>;
}

export interface StatusRow {
  name: string;
  status: CheckStatus;
  latency?: string;
}

export function toStatusRows(report: HealthReport | null): StatusRow[] {
  const dependencies = ['postgres', 'redis'];
  return [
    {
      name: 'API',
      status: report ? (report.status === 'ready' ? 'up' : 'down') : 'unknown'
    },
    ...dependencies.map((name) => {
      const check = report?.checks[name];
      return {
        name: name === 'postgres' ? 'PostgreSQL' : 'Redis',
        status: check?.status ?? 'unknown',
        latency:
          check?.latency_ms === undefined ? undefined : `${check.latency_ms} ms`
      } satisfies StatusRow;
    })
  ];
}

export async function fetchHealth(signal?: AbortSignal): Promise<HealthReport> {
  const response = await fetch('/health/ready', {
    headers: { Accept: 'application/json' },
    signal
  });
  const report = (await response.json()) as HealthReport;
  if (!response.ok && response.status !== 503) {
    throw new Error(`Health request failed with status ${response.status}`);
  }
  return report;
}
