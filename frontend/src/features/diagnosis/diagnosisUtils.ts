export function diagnosisStatusLabel(status: string) {
  const labels: Record<string, string> = {
    queued: '排队中',
    planning: '规划中',
    collecting: '采集中',
    analyzing: '分析中',
    succeeded: '已完成',
    failed: '失败',
    cancelled: '已取消',
    skipped: '已跳过',
    warning: '需核验'
  };
  return labels[status] ?? status;
}

export function diagnosisDuration(start: string, end?: string) {
  const begin = new Date(start).getTime();
  const finish = end ? new Date(end).getTime() : Date.now();
  if (!Number.isFinite(begin) || !Number.isFinite(finish)) return '—';
  const seconds = Math.max(0, Math.round((finish - begin) / 1000));
  return seconds < 60
    ? `${seconds}s`
    : `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}

export function diagnosisMilliseconds(value: unknown, fallback = 0) {
  const milliseconds = Number(value);
  return Number.isFinite(milliseconds) && milliseconds >= 0 ? milliseconds : fallback;
}

export function diagnosisDurationMilliseconds(milliseconds: number) {
  if (!Number.isFinite(milliseconds) || milliseconds < 0) return '—';
  return milliseconds < 1000
    ? `${Math.max(1, Math.round(milliseconds))}ms`
    : `${(milliseconds / 1000).toFixed(milliseconds >= 10000 ? 1 : 2).replace(/\.0+$/, '').replace(/(\.\d)0$/, '$1')}s`;
}

export function diagnosisActionLabel(item: { label?: string; tool?: string }) {
  return item.label || (item.tool ? `调用工具 ${item.tool}` : '执行动作');
}

export function diagnosisProcessActionCount(snapshot: DiagnosisSnapshot | null) {
  if (!snapshot) return 0;
  return (snapshot.events ?? []).filter(
    (event) => event.type === 'tool.completed' || event.type === 'tool.failed'
  ).length;
}

export function diagnosisProcessDuration(snapshot: DiagnosisSnapshot | null) {
  if (!snapshot) return '—';
  const events = [...(snapshot.events ?? [])].sort((a, b) => a.id - b.id);
  const started = events.find((event) => event.type === 'execution.started');
  if (!started) return '—';
  const finished = [...events].reverse().find((event) =>
    event.type === 'execution.completed' ||
    event.type === 'execution.failed' ||
    event.type === 'execution.cancelled' ||
    event.type === 'report.ready'
  );
  return diagnosisDuration(started.created_at, finished?.created_at);
}
import type { DiagnosisCausalChain, DiagnosisSnapshot } from '../../lib/api';

export function diagnosisCausalNodes(chain: DiagnosisCausalChain) {
  const nodes = new Map(chain.nodes.map((node) => [node.id, node]));
  const incoming = new Set(chain.links.map((link) => link.to));
  const ordered: typeof chain.nodes = [];
  const seen = new Set<string>();
  const visit = (id: string) => {
    if (seen.has(id)) return;
    const node = nodes.get(id);
    if (!node) return;
    seen.add(id);
    ordered.push(node);
    for (const link of chain.links.filter((item) => item.from === id)) visit(link.to);
  };
  for (const node of chain.nodes) if (!incoming.has(node.id)) visit(node.id);
  for (const node of chain.nodes) visit(node.id);
  return ordered;
}

export function diagnosisCausalEvidenceIDs(chain: DiagnosisCausalChain, nodeID: string) {
  const node = chain.nodes.find((item) => item.id === nodeID);
  const related = chain.links.filter((item) => item.from === nodeID || item.to === nodeID);
  return [...new Set([
    ...(node?.evidence_ids ?? []),
    ...related.flatMap((item) => item.evidence_ids ?? [])
  ])];
}

export type DiagnosisAction = {
  id: string;
  icon: 'tool';
  title: string;
  status: string;
  duration: string;
  input: string;
  output: string;
  created_at: string;
  updated_at: string;
  tool: string;
  resourceID: string;
  callID: string;
};

export type DiagnosisActionGroup = {
  id: string;
  title: string;
  status: string;
  duration: string;
  children: DiagnosisAction[];
};

export function diagnosisActionData(snapshot: DiagnosisSnapshot): DiagnosisActionGroup[] {
  const events = [...(snapshot.events ?? [])].sort((a, b) => a.id - b.id);
  const titleForTool = (name: string) => {
    const titles: Record<string, string> = {
      'connector.query_metrics': '查询监控指标',
      'connector.get_alerts': '查询告警',
      'connector.query_logs': '查询日志',
      'connector.inspect_postgresql': '检查 PostgreSQL',
      'connector.read_kubernetes': '查询 Kubernetes'
    };
    return titles[name] ?? `调用 ${name}`;
  };
  const jsonText = (value: unknown, fallback: string) => {
    if (value === undefined) return fallback;
    if (typeof value === 'string') {
      try {
        return JSON.stringify(JSON.parse(value), null, 2);
      } catch {
        return JSON.stringify(value, null, 2);
      }
    }
    try {
      return JSON.stringify(value, null, 2) ?? fallback;
    } catch {
      return fallback;
    }
  };
  const groups: DiagnosisActionGroup[] = [];
  let currentGroup: DiagnosisActionGroup | null = null;

  for (const event of events) {
    // A new model turn or progress summary is a user-visible boundary:
    // tool calls on either side belong to different collapsed groups.
    if (event.type === 'model.started') {
      currentGroup = null;
      continue;
    }
    if (event.type === 'assistant.delta' || event.type === 'assistant.progress') {
      if (String(event.payload?.text ?? '').trim()) currentGroup = null;
      continue;
    }
    if (!event.type.startsWith('tool.')) continue;
    const payload = event.payload ?? {};
    const tool = String(payload.tool ?? '');
    const resourceID = String(payload.resource_id ?? '');
    const callID = String(payload.call_id ?? payload.call_sequence ?? '');
    // Evidence bookkeeping also emits tool.completed, but it is not an
    // AIEngine invocation and therefore must not appear in this trace.
    const isAIEngineToolEvent = Object.prototype.hasOwnProperty.call(payload, 'resource_id') || Boolean(callID);
    if (!tool || !isAIEngineToolEvent) continue;
    if (!currentGroup) {
      currentGroup = {
        id: `tool-group-${event.id}`,
        title: '调用工具',
        status: '进行中',
        duration: '—',
        children: []
      };
      groups.push(currentGroup);
    }
    let action = [...currentGroup.children]
      .reverse()
      .find((item) => item.tool === tool && item.resourceID === resourceID && item.status === '进行中' &&
        (callID ? item.callID === callID : !item.callID));
    if (!action) {
      action = {
        id: `tool-${event.id}`,
        icon: 'tool',
        title: titleForTool(tool),
        status: '进行中',
        duration: '—',
        input: jsonText(payload.arguments, JSON.stringify({ tool, resource_id: resourceID }, null, 2)),
        output: '等待工具执行结果…',
        created_at: event.created_at,
        updated_at: event.created_at,
        tool,
        resourceID,
        callID
      };
      currentGroup.children.push(action);
    }
    action.updated_at = event.created_at;
    if (Object.prototype.hasOwnProperty.call(payload, 'arguments')) {
      action.input = jsonText(payload.arguments, action.input);
    }
    if (event.type === 'tool.completed') {
      action.status = '已完成';
      const duration = Number(payload.duration_ms ?? 0);
      action.duration = duration > 0 ? `${Math.max(1, Math.round(duration / 1000))}s` : diagnosisDuration(action.created_at, event.created_at);
      action.output = jsonText(payload.output, JSON.stringify({ status: 'succeeded', duration_ms: payload.duration_ms ?? 0 }, null, 2));
    } else if (event.type === 'tool.failed') {
      action.status = '失败';
      const duration = Number(payload.duration_ms ?? 0);
      action.duration = duration > 0 ? `${Math.max(1, Math.round(duration / 1000))}s` : diagnosisDuration(action.created_at, event.created_at);
      action.output = jsonText(payload.output, JSON.stringify({ status: 'failed', error: payload.error ?? '工具调用失败' }, null, 2));
    }
    const completed = currentGroup.children.every((item) => item.status !== '进行中');
    currentGroup.status = completed ? (currentGroup.children.some((item) => item.status === '失败') ? '失败' : '已完成') : '进行中';
    currentGroup.duration = diagnosisDuration(currentGroup.children[0].created_at, action.updated_at);
  }
  return groups;
}

export function diagnosisHasRunningActions(snapshot: DiagnosisSnapshot | null) {
  if (snapshot && (snapshot.session.status === 'succeeded' || snapshot.session.status === 'failed' || snapshot.session.status === 'cancelled')) {
    return false;
  }
  return Boolean(
    snapshot && diagnosisActionData(snapshot).some((group) => group.status === '进行中')
  );
}
