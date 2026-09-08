import type { ConnectorCapability, Resource, ResourceSchema } from '../../lib/api';
export { brandNameFor } from '../../lib/resources';

export function relativeConnectionTime(value: string, now = Date.now()) {
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return '未测试';
  const elapsed = Math.max(now - timestamp, 0);
  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;
  const week = 7 * day;
  const month = 30 * day;
  const year = 365 * day;
  if (elapsed < minute) return '刚刚';
  if (elapsed < hour) return `${Math.floor(elapsed / minute)} 分前`;
  if (elapsed < day) return `${Math.floor(elapsed / hour)} 时前`;
  if (elapsed < week) return `${Math.floor(elapsed / day)} 天前`;
  if (elapsed < month) return `${Math.floor(elapsed / week)} 周前`;
  if (elapsed < year) return `${Math.floor(elapsed / month)} 月前`;
  return `${Math.floor(elapsed / year)} 年前`;
}

export const resourceCategoryOptions: Record<string, string[]> = {
  全部: [],
  Application: ['虚拟机', '容器化', '云原生'],
  Artifact: ['Generic', 'Docker', 'Helm'],
  Repository: ['Git', 'Bundle'],
  Host: ['Direct', 'Agent'],
  Docker: ['Direct', 'Agent'],
  Kubernetes: ['Direct', 'Agent'],
  Redis: ['Direct', 'Agent'],
  TongRDS: ['Direct', 'Agent'],
  Kafka: ['Direct', 'Agent'],
  RabbitMQ: ['Direct', 'Agent'],
  Elasticsearch: ['Direct', 'Agent'],
  OceanBase: ['Direct', 'Agent'],
  Oracle: ['Direct', 'Agent'],
  MySQL: ['Direct', 'Agent'],
  PostgreSQL: ['Direct', 'Agent'],
  MCPServer: ['StreamHTTP', 'SSE'],
  Skill: ['诊断', '监控', '优化', '维护'],
  LLM: ['Provider'],
  监控: ['指标', '日志', '链路', '告警']
};

type ResourceShape = {
  kind: string;
  subtype?: string;
  config?: Record<string, unknown>;
};

export function connectorCapabilityName(capability: ConnectorCapability) {
  const names: Record<ConnectorCapability, string> = {
    kubernetes_read: '读取 Kubernetes',
    query_metrics: '查询指标',
    query_logs: '查询日志',
    query_traces: '查询链路',
    get_alerts: '读取告警'
  };
  return names[capability] ?? capability;
}

export function resourceCategoryFor(resource: ResourceShape) {
  if (resource.kind === 'AIProvider') return 'LLM';
  if (resource.kind === 'MCPServer') return 'MCPServer';
  if (['Application', 'Artifact', 'Repository', 'Host'].includes(resource.kind)) return resource.kind;
  if (['Redis', 'Kafka', 'Elasticsearch', 'RabbitMQ', 'TongRDS', 'OceanBase', 'Oracle', 'MySQL', 'PostgreSQL', 'Docker', 'Kubernetes', 'Skill'].includes(resource.kind)) return resource.kind;
  if (['Prometheus', 'Loki', 'Tempo', 'Jaeger', 'Elastic', 'Datadog', 'Alertmanager'].includes(resource.kind)) return '监控';
  return resource.kind;
}

export function resourceSubtypeFor(resource: ResourceShape) {
  const fallback: Record<string, string> = {
    Application: '虚拟机', Artifact: 'Generic', Kubernetes: 'Direct', Host: 'Direct',
    Docker: 'Direct', Redis: 'Direct', Kafka: 'Direct', Elasticsearch: 'Direct',
    RabbitMQ: 'Direct', TongRDS: 'Direct', OceanBase: 'Direct', Oracle: 'Direct',
    MySQL: 'Direct', PostgreSQL: 'Direct', Repository: 'Git', MCPServer: 'StreamHTTP',
    AIProvider: 'Provider', Prometheus: '指标', Loki: '日志', Tempo: '链路',
    Alertmanager: '告警'
  };
  const explicit = String(resource.subtype || resource.config?.subtype || '');
  if (resource.kind === 'AIProvider') return 'Provider';
  if (['Host', 'Docker', 'Kubernetes', 'Redis', 'TongRDS', 'Kafka', 'RabbitMQ', 'Elasticsearch', 'OceanBase', 'Oracle', 'MySQL', 'PostgreSQL'].includes(resource.kind)) return explicit || 'Direct';
  return String(explicit || resource.config?.provider || fallback[resource.kind] || resource.kind);
}

export function resourceSubtypeOptionsFor(resource: Resource) {
  const current = resourceSubtypeFor(resource);
  const options = resourceCategoryOptions[resourceCategoryFor(resource)] ?? [];
  return options.includes(current) ? options : [current, ...options];
}

export function resourceCategoryIcon(category: string) {
  const icons: Record<string, string> = {
    全部: '◇', Application: '⌘', Artifact: '▤', Repository: '⌘', Host: '▣', Docker: '◈',
    Kubernetes: '⬡', Redis: '◒', TongRDS: '◒', Kafka: '◒', RabbitMQ: '◒',
    Elasticsearch: '◒', OceanBase: '◉', Oracle: '◉', MySQL: '◉', PostgreSQL: '◉',
    MCPServer: '⌁', Skill: '✧', LLM: '✦', 监控: '◌'
  };
  return icons[category] ?? '◇';
}

export function resourceSchemaForSelection(schemas: ResourceSchema[], category: string, subtype: string) {
  return schemas.find((schema) => resourceCategoryFor(schema) === category && resourceSubtypeFor(schema) === subtype)
    ?? schemas.find((schema) => resourceCategoryFor(schema) === category)
    ?? schemas[0]
    ?? null;
}

export function resourceEndpointFor(resource: Resource) {
  if (resource.kind === 'AIProvider') return String(resource.config?.base_url ?? '未设置服务地址');
  if (resource.kind === 'Docker') {
    const mode = String(resource.subtype ?? '').toLowerCase();
    if (mode === 'agent') return 'MCPServer 代理';
    return String(resource.config?.host ?? '本机默认 Unix Socket');
  }
  return String(resource.config?.url ?? resource.config?.endpoint ?? resource.config?.host ?? '未设置端点');
}

export function resourceLabelsText(resource: Resource) {
  return Object.entries(resource.labels ?? {}).map(([key, value]) => value ? `${key}=${value}` : key).join(', ');
}
