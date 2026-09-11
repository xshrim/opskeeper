import type { DiagnosisEvent, DiagnosisSnapshot } from '../../lib/api';
import {
  diagnosisDurationMilliseconds,
  diagnosisMilliseconds,
  diagnosisStatusLabel
} from './diagnosisUtils';

const sortedDiagnosisEvents = (snapshot: DiagnosisSnapshot) =>
  [...(snapshot.events ?? [])].sort((left, right) => left.id - right.id);

const latestExecutionIndex = (events: DiagnosisEvent[]) =>
  events.reduce(
    (index, event, currentIndex) =>
      event.type === 'execution.started' ? currentIndex : index,
    -1
  );

const diagnosisRunIsTerminal = (
  snapshot: DiagnosisSnapshot,
  events: DiagnosisEvent[],
  executionIndex: number
) => {
  const runEvents = executionIndex >= 0 ? events.slice(executionIndex + 1) : events;
  return Boolean(
    snapshot.session.status === 'succeeded' ||
      snapshot.session.status === 'failed' ||
      snapshot.session.status === 'cancelled' ||
      runEvents.some((event) =>
        event.type === 'assistant.completed' ||
        event.type === 'report.ready' ||
        event.type === 'diagnosis.failed' ||
        event.type === 'diagnosis.cancelled'
      )
  );
};

export type DiagnosisEvidenceTimelineItem = {
  id: string;
  kind: 'turn' | 'analysis' | 'phase' | 'tool-group' | 'tool' | 'observation' | 'status';
  title: string;
  detail?: string;
  status?: string;
  tool?: string;
  input?: string;
  output?: string;
  duration?: string;
  children?: DiagnosisEvidenceTimelineItem[];
  resourceID?: string;
  evidenceIds?: string[];
};

export function diagnosisEvidenceTimeline(snapshot: DiagnosisSnapshot | null): DiagnosisEvidenceTimelineItem[] {
  if (!snapshot) return [];
  const events = sortedDiagnosisEvents(snapshot);
  const items: DiagnosisEvidenceTimelineItem[] = [];
  const tools = new Map<string, DiagnosisEvidenceTimelineItem>();
  let currentToolGroup: DiagnosisEvidenceTimelineItem | null = null;
  let currentTurn: DiagnosisEvidenceTimelineItem | null = null;
  let activeExecutionStarted = false;
  let userMessageCursor = 0;
  const userMessages = [...(snapshot.messages ?? [])].filter((message) => message.role === 'user');
  const jsonText = (value: unknown, fallback = '暂无数据') => {
    if (value === undefined || value === null) return fallback;
    if (typeof value === 'string') {
      try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; }
    }
    try { return JSON.stringify(value, null, 2) ?? fallback; } catch { return fallback; }
  };
  const toolName = (value: unknown) => String(value ?? '').trim() || '受控工具';
  const collectedEvidence = (snapshot.evidence ?? []).map((evidence) => ({
    id: evidence.id,
    sourceResourceID: evidence.source_resource_id ?? evidence.target_resource_id ?? '',
    capability: evidence.capability
  }));
  const compactQuestion = (value: string) => value.replace(/\s+/g, ' ').trim() || '本次诊断';
  const beginTurn = (event: DiagnosisEvent) => {
    if (currentTurn) return currentTurn;
    const message = userMessages[userMessageCursor++];
    currentTurn = {
      id: `chain-${event.id}`,
      kind: 'turn',
      title: compactQuestion(message?.content ?? '本次诊断'),
      detail: '从规划到结论的完整回答过程',
      children: []
    };
    items.push(currentTurn);
    currentToolGroup = null;
    tools.clear();
    return currentTurn;
  };
  const addChild = (item: DiagnosisEvidenceTimelineItem) => {
    beginTurn(events[0] ?? ({ id: 0 } as DiagnosisEvent));
    currentTurn?.children?.push(item);
  };
  for (const event of events) {
    const payload = event.payload ?? {};
    // Planning is the beginning of a response chain. This keeps the plan,
    // actions, observations and terminal state together in their original
    // event order, including events emitted before execution.started.
    if (event.type === 'phase.changed' && String(payload.phase ?? '').trim() === 'planning') {
      if (currentTurn && activeExecutionStarted) {
        currentTurn = null;
        activeExecutionStarted = false;
      }
      if (!currentTurn) beginTurn(event);
    } else if (event.type === 'plan.created') {
      if (!currentTurn) beginTurn(event);
    }
    if (event.type === 'execution.started') {
      // Every execution is a separate answer chain, including follow-up
      // questions in the same diagnosis session.
      const turnBeforeExecution = currentTurn as DiagnosisEvidenceTimelineItem | null;
      if (turnBeforeExecution && (turnBeforeExecution.children ?? []).some((child) => child.kind === 'status')) currentTurn = null;
      if (!currentTurn) beginTurn(event);
      activeExecutionStarted = true;
      currentToolGroup = null;
      tools.clear();
      continue;
    }
    if (event.type === 'model.started') {
      currentToolGroup = null;
      tools.clear();
      addChild({ id: `event-${event.id}`, kind: 'analysis', title: '开始分析', detail: String(payload.detail ?? '正在评估问题并决定下一步行动') });
      continue;
    }
    if (event.type === 'assistant.progress') {
      currentToolGroup = null;
      tools.clear();
      const text = String(payload.text ?? '').trim();
      if (text) addChild({ id: `event-${event.id}`, kind: 'analysis', title: '阶段说明', detail: text });
      continue;
    }
    if (event.type === 'phase.changed') {
      currentToolGroup = null;
      tools.clear();
      const phase = String(payload.phase ?? '').trim();
      if (phase) addChild({ id: `event-${event.id}`, kind: 'phase', title: diagnosisStatusLabel(phase), detail: String(payload.detail ?? '') || undefined });
      continue;
    }
    if (event.type === 'model.resumed') {
      currentToolGroup = null;
      tools.clear();
      const observation = payload.observation ?? payload.observations;
      if (observation !== undefined) addChild({ id: `event-${event.id}`, kind: 'observation', title: '收到工具结果，重新评估', detail: jsonText(observation) });
      continue;
    }
    if (event.type === 'context.compacted') {
      currentToolGroup = null;
      tools.clear();
      const compacted = Array.isArray(payload.compacted_observations)
        ? payload.compacted_observations
        : [];
      const count = Number(payload.evicted_observation_count ?? compacted.length);
      addChild({
        id: `event-${event.id}`,
        kind: 'status',
        title: '已压缩执行上下文',
        detail: jsonText({
          evicted_message_count: Number(payload.evicted_message_count ?? 0),
          evicted_observation_count: count,
          compacted_observations: compacted
        })
      });
      continue;
    }
    if (event.type === 'execution.completed' || event.type === 'execution.failed' || event.type === 'execution.cancelled') {
      activeExecutionStarted = false;
      currentToolGroup = null;
      tools.clear();
      addChild({ id: `event-${event.id}`, kind: 'status', title: event.type === 'execution.completed' ? '本轮执行完成' : event.type === 'execution.cancelled' ? '本轮执行已取消' : '本轮执行失败', detail: String(payload.error ?? payload.message ?? '') || undefined });
      continue;
    }
    if (event.type === 'evidence.collected') continue;
    if (!event.type.startsWith('tool.') || !Object.prototype.hasOwnProperty.call(payload, 'resource_id')) continue;
    const key = String(payload.call_id ?? payload.call_sequence ?? event.id);
    if (!currentToolGroup) {
      currentToolGroup = {
        id: `tool-group-${event.id}`,
        kind: 'tool-group',
        title: '连续调用工具',
        status: '执行中',
        children: []
      };
      addChild(currentToolGroup);
    }
    const children = currentToolGroup.children ?? (currentToolGroup.children = []);
    let item = tools.get(key);
    if (!item) {
      item = { id: `tool-${key}`, kind: 'tool', title: toolName(payload.tool), tool: toolName(payload.tool), status: '等待执行', input: jsonText(payload.arguments), output: '等待工具执行结果…', resourceID: String(payload.resource_id ?? '') };
      tools.set(key, item);
      children.push(item);
    }
    if (Object.prototype.hasOwnProperty.call(payload, 'arguments')) item.input = jsonText(payload.arguments);
    if (event.type === 'tool.requested') item.status = '等待执行';
    if (event.type === 'tool.started') item.status = '执行中';
    if (event.type === 'tool.completed' || event.type === 'tool.failed') {
      item.status = event.type === 'tool.completed' ? '已完成' : '失败';
      item.output = jsonText(payload.output, payload.error ? jsonText({ error: payload.error }) : '暂无出参');
      const duration = Number(payload.duration_ms ?? 0);
      if (duration > 0) item.duration = diagnosisDurationMilliseconds(duration);
    }
    currentToolGroup.status = children.some((child) => child.status === '失败')
      ? '存在失败'
      : children.every((child) => child.status === '已完成') ? '已完成' : '执行中';
    const names = [...new Set(children.map((child) => child.tool ?? child.title).filter(Boolean))];
    currentToolGroup.detail = `${children.length} 个动作${names.length ? ` · ${names.join('、')}` : ''}`;
  }
  // Evidence is persisted after the tool result. Link it back to the most
  // likely tool by source resource and capability, while keeping the
  // evidence drawer useful even when a connector does not expose a call id.
  const toolItems = items.flatMap((item) => item.children?.flatMap((child) => child.kind === 'tool-group' ? child.children ?? [] : []) ?? []);
  for (const evidence of collectedEvidence) {
    const candidate = [...toolItems].reverse().find((tool) =>
      (!evidence.sourceResourceID || tool.resourceID === evidence.sourceResourceID) &&
      (!evidence.capability || tool.tool === evidence.capability || tool.tool?.includes(evidence.capability) || evidence.capability.includes(tool.tool ?? ''))
    ) ?? [...toolItems].reverse().find((tool) => !evidence.sourceResourceID || tool.resourceID === evidence.sourceResourceID);
    if (candidate) candidate.evidenceIds = [...(candidate.evidenceIds ?? []), evidence.id];
  }
  const terminal = diagnosisRunIsTerminal(snapshot, events, latestExecutionIndex(events));
  if (!terminal) return items;
  return items.map((turn) => ({
    ...turn,
    children: (turn.children ?? []).map((child) => {
      if (child.kind !== 'tool-group') return child;
      const children = (child.children ?? []).map((tool) => {
        if (tool.status === '等待执行') return { ...tool, status: '未执行' };
        if (tool.status === '执行中') return { ...tool, status: '已中断' };
        return tool;
      });
      return {
        ...child,
        children,
        status: children.some((tool) => tool.status === '失败') ? '存在失败' : '已完成'
      };
    })
  }));
}

export type DiagnosisLiveTimelineItem = {
  id: number;
  kind: 'analysis' | 'action';
  text?: string;
  tool?: string;
  label?: string;
  status?: string;
  duration?: string;
  elapsed?: string;
  iteration?: number;
  input?: string;
  output?: string;
  actions?: DiagnosisLiveTimelineItem[];
  createdAt?: string;
  updatedAt?: string;
};

export function diagnosisLiveTimeline(snapshot: DiagnosisSnapshot | null): DiagnosisLiveTimelineItem[] {
  if (!snapshot) return [];
  const events = sortedDiagnosisEvents(snapshot);
  const executionIndex = latestExecutionIndex(events);
  const latestMessageIndex = events.reduce(
    (index, event, currentIndex) =>
      event.type === 'message.created' ? currentIndex : index,
    -1
  );
  // A follow-up message is persisted before its background run starts. During
  // that gap, the last execution belongs to the previous answer and must not
  // be rendered as the new live process.
  if (latestMessageIndex > executionIndex) return [];
  const activeEvents =
    executionIndex < 0 ? events : events.slice(executionIndex);
  const items: DiagnosisLiveTimelineItem[] = [];
  // A model delta is initially ambiguous: it can become the final answer or
  // the short user-visible explanation that precedes a tool call. Resolve
  // that ambiguity from the complete event batch without changing the text.
  // Deltas belonging to an iteration that contains tool lifecycle events are
  // rendered in the execution timeline in their original event position.
  const toolIterations = new Set(
    activeEvents
      .filter((event) => event.type.startsWith('tool.'))
      .map((event) => Number(event.payload?.iteration ?? 0) || 0)
      .filter((iteration) => iteration > 0)
  );
  const analysisByIteration = new Map<number, DiagnosisLiveTimelineItem>();
  const startedAt = activeEvents.find((event) => event.type === 'execution.started')?.created_at;
  const toolStarted = new Map<string, string>();
  const toolRequested = new Map<string, string>();
  let currentGroup: DiagnosisLiveTimelineItem | null = null;
  let lastAnalysis = '';
  const toolName = (event: DiagnosisEvent) => String(event.payload?.tool ?? '').trim();
  const toolKey = (event: DiagnosisEvent) => String(event.payload?.call_id ?? event.payload?.call_sequence ?? event.id);
  const actionLabel = (tool: string) => {
    const normalized = tool.toLowerCase();
    return /(?:^|[._-])(exec|command|shell|terminal|run)(?:$|[._-])/.test(normalized)
      ? `执行命令 ${tool}`
      : `调用工具 ${tool}`;
  };
  const jsonText = (value: unknown, fallback = '{}') => {
    if (value === undefined || value === null) return fallback;
    if (typeof value === 'string') {
      try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; }
    }
    try { return JSON.stringify(value, null, 2) ?? fallback; } catch { return fallback; }
  };
  const ensureGroup = (event: DiagnosisEvent) => {
    if (currentGroup) return currentGroup;
    currentGroup = {
      id: event.id,
      kind: 'action',
      tool: '',
      status: '执行中',
      duration: '—',
      elapsed: '—',
      iteration: Number(event.payload?.iteration ?? 0) || undefined,
      actions: [],
      createdAt: event.created_at,
      updatedAt: event.created_at
    };
    items.push(currentGroup);
    return currentGroup;
  };
  for (const event of activeEvents) {
    const payload = event.payload ?? {};
    const iteration = Number(payload.iteration ?? 0) || undefined;
    if (event.type === 'assistant.delta') {
      if (iteration && toolIterations.has(iteration)) {
        let analysis = analysisByIteration.get(iteration);
        if (!analysis) {
          analysis = { id: event.id, kind: 'analysis', text: '', iteration };
          analysisByIteration.set(iteration, analysis);
          items.push(analysis);
        }
        analysis.text = `${analysis.text ?? ''}${String(payload.text ?? '')}`;
      }
      continue;
    }
    if (event.type === 'assistant.progress') {
      currentGroup = null;
      const text = String(payload.text ?? '').trim();
      if (String(payload.kind ?? '') === 'tool_decision' && iteration && toolIterations.has(iteration)) {
        // The same sentence may have been emitted as deltas before the
        // function call was finalized. Keep the delta version only once.
        const analysis = analysisByIteration.get(iteration);
        if (analysis && analysis.text) continue;
      }
      if (text && text !== lastAnalysis) {
        items.push({ id: event.id, kind: 'analysis', text, iteration });
        lastAnalysis = text;
      }
      continue;
    }
    if (event.type === 'model.started' || event.type === 'model.resumed') {
      currentGroup = null;
      continue;
    }
    if (event.type === 'context.compacted') {
      currentGroup = null;
      const compacted = Array.isArray(payload.compacted_observations)
        ? payload.compacted_observations
        : [];
      items.push({
        id: event.id,
        kind: 'action',
        tool: 'context.compacted',
        label: '压缩执行上下文',
        status: '已完成',
        duration: diagnosisDurationMilliseconds(Number(payload.duration_ms ?? 0)),
        elapsed: diagnosisDurationMilliseconds(Number(payload.elapsed_ms ?? 0)),
        iteration,
        input: jsonText({ evicted_message_count: Number(payload.evicted_message_count ?? 0) }),
        output: jsonText({ compacted_observations: compacted }),
        createdAt: event.created_at,
        updatedAt: event.created_at
      });
      continue;
    }
    if (!event.type.startsWith('tool.')) continue;
    const tool = toolName(event);
    if (!tool || !Object.prototype.hasOwnProperty.call(payload, 'resource_id')) continue;
    const key = toolKey(event);
    const group = ensureGroup(event);
    const actions = group.actions ?? (group.actions = []);
    let action = actions.find((item) => item.id === Number(key) || item.tool === tool && item.createdAt === toolRequested.get(key));
    if (!action) {
      action = {
        id: event.id,
        kind: 'action',
        tool,
        label: actionLabel(tool),
        status: '等待执行',
        duration: '—',
        elapsed: '—',
        iteration,
        input: payload.arguments === undefined ? undefined : jsonText(payload.arguments),
        output: '等待工具执行结果…',
        createdAt: event.created_at,
        updatedAt: event.created_at
      };
      actions.push(action);
    }
    action.updatedAt = event.created_at;
    if (payload.arguments !== undefined) action.input = jsonText(payload.arguments);
    if (event.type === 'tool.requested') {
      toolRequested.set(key, event.created_at);
      action.status = '等待执行';
    } else if (event.type === 'tool.started') {
      toolStarted.set(key, event.created_at);
      action.status = '执行中';
    } else if (event.type === 'tool.completed' || event.type === 'tool.failed') {
      const started = toolStarted.get(key) ?? toolRequested.get(key) ?? event.created_at;
      const inferredDuration = Math.max(0, new Date(event.created_at).getTime() - new Date(started).getTime());
      const duration = diagnosisMilliseconds(payload.duration_ms, inferredDuration);
      const elapsedFallback = startedAt
        ? Math.max(0, new Date(event.created_at).getTime() - new Date(startedAt).getTime())
        : 0;
      action.status = event.type === 'tool.completed' ? '已完成' : '失败，正在重新评估';
      action.duration = diagnosisDurationMilliseconds(duration);
      action.elapsed = diagnosisDurationMilliseconds(diagnosisMilliseconds(payload.elapsed_ms, elapsedFallback));
      action.output = payload.output === undefined
        ? jsonText(payload.error ? { error: payload.error } : {})
        : jsonText(payload.output);
    }
    group.tool = actions.length === 1 ? tool : undefined;
    group.status = actions.some((item) => item.status === '失败，正在重新评估')
      ? '失败'
      : actions.every((item) => item.status === '已完成' || item.status === '失败，正在重新评估')
        ? '已完成'
        : '执行中';
    group.duration = diagnosisDurationMilliseconds(
      Math.max(0, new Date(event.created_at).getTime() - new Date(group.createdAt ?? event.created_at).getTime())
    );
    group.elapsed = diagnosisDurationMilliseconds(startedAt
      ? Math.max(0, new Date(event.created_at).getTime() - new Date(startedAt).getTime())
      : 0);
    group.updatedAt = event.created_at;
  }
  // A terminal session can be observed before the final tool lifecycle event
  // reaches the client. Do not leave stale requested actions labelled as
  // "等待执行" next to an already persisted answer.
  const terminal = diagnosisRunIsTerminal(snapshot, events, executionIndex);
  if (!terminal) return items;
  return items.flatMap((item) => {
    if (item.kind === 'analysis' || item.tool === 'context.compacted') return [item];
    const actions = (item.actions ?? []).filter((action) => action.status !== '等待执行' && action.status !== '执行中');
    if (!actions.length) return [];
    return [{
      ...item,
      actions,
      tool: actions.length === 1 ? actions[0].tool : undefined,
      status: actions.some((action) => action.status === '失败，正在重新评估') ? '失败' : '已完成'
    }];
  });
}

/** Build the execution trace belonging to one persisted assistant answer. */
export function diagnosisAssistantTimeline(
  snapshot: DiagnosisSnapshot | null,
  assistantIndex: number
): DiagnosisLiveTimelineItem[] {
  if (!snapshot || assistantIndex < 0) return [];
  const events = sortedDiagnosisEvents(snapshot);
  const starts = events
    .map((event, index) => (event.type === 'execution.started' ? index : -1))
    .filter((index) => index >= 0);
  // Edited questions can hide an older assistant message from the visible
  // conversation while its execution remains in the event history. Match the
  // answer to the run through its preceding user question.
  const assistant = snapshot.messages.filter((message) => message.role === 'assistant')[assistantIndex];
  const answerPosition = assistant
    ? snapshot.messages.findIndex((message) => message.id === assistant.id)
    : -1;
  const question = snapshot.messages
    .slice(0, answerPosition)
    .reverse()
    .find((message) => message.role === 'user');
  const runs = [...snapshot.runs].sort((left, right) => left.sequence - right.sequence);
  let runIndex = assistantIndex;
  const matchingRun = runs.findIndex(
    (run) => String(run.question_message_id ?? '') === String(question?.id ?? '')
  );
  if (matchingRun >= 0) runIndex = matchingRun;
  const start = starts[runIndex];
  if (start === undefined) return [];
  const end = starts[runIndex + 1] ?? events.length;
  return diagnosisLiveTimeline({
    ...snapshot,
    session: { ...snapshot.session, status: 'succeeded' },
    events: events
      .slice(start, end)
      .filter((event) => event.type !== 'message.created')
  });
}
