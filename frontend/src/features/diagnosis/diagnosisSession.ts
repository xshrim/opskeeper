import type { DiagnosisEvent, DiagnosisSnapshot } from '../../lib/api';

export const diagnosisEventTypes = [
  'session.created',
  'phase.changed',
  'plan.created',
  'execution.started',
  'execution.completed',
  'execution.cancelled',
  'execution.failed',
  'model.started',
  'model.resumed',
  'assistant.progress',
  'assistant.delta',
  'assistant.completed',
  'context.compacted',
  'tool.requested',
  'tool.started',
  'tool.completed',
  'tool.failed',
  'evidence.collected',
  'report.ready',
  'diagnosis.failed',
  'diagnosis.cancelled',
  'message.created',
  'target.added'
] as const;

export type DiagnosisEventStream = () => void;

export function openDiagnosisEventStream(
  sessionID: string,
  after: number,
  eventsURL: (sessionID: string, after?: number) => string | URL,
  onEvent: (event: MessageEvent) => void,
  onError: (error: Error) => void = () => {}
): DiagnosisEventStream {
  const stream = new EventSource(eventsURL(sessionID, after));
  let reportedError = false;
  let closed = false;
  let terminalSeen = false;
  const stop = () => {
    if (closed) return;
    closed = true;
    stream.close();
  };
  const dispatch = (event: MessageEvent) => {
    if (
      event.type === 'report.ready' ||
      event.type === 'diagnosis.failed' ||
      event.type === 'diagnosis.cancelled'
    ) {
      terminalSeen = true;
    }
    onEvent(event);
    if (terminalSeen) stop();
  };
  stream.onmessage = dispatch;
  for (const type of diagnosisEventTypes) {
    stream.addEventListener(type, (event) => dispatch(event as MessageEvent));
  }
  stream.onerror = () => {
    // Native EventSource reconnects with the last received event id.
    if (!closed && !terminalSeen && !reportedError) {
      reportedError = true;
      onError(new Error('诊断实时连接暂时不可用，正在重试。'));
    }
  };
  return () => {
    stop();
  };
}

export function appendDiagnosisEvent(
  snapshot: DiagnosisSnapshot | null,
  type: string,
  payload: Record<string, unknown>,
  id: number,
  createdAt = new Date().toISOString()
) {
  if (!snapshot || !id) return snapshot;
  const current = snapshot.events ?? [];
  if (current.some((item) => item.id === id)) return snapshot;
  const item: DiagnosisEvent = {
    id,
    session_id: snapshot.session.id,
    type,
    payload,
    created_at: createdAt
  };
  return {
    ...snapshot,
    events: [...current, item].sort((left, right) => left.id - right.id)
  };
}

export function mergeDiagnosisSnapshot(
  snapshot: DiagnosisSnapshot,
  localSnapshot: DiagnosisSnapshot | null,
  hiddenMessageIDs: string[]
) {
  const localEvents =
    localSnapshot?.session.id === snapshot.session.id
      ? (localSnapshot.events ?? [])
      : [];
  const mergedByID = new Map<number, DiagnosisEvent>();
  for (const event of snapshot.events ?? []) mergedByID.set(event.id, event);
  // SSE events can arrive before the database-backed snapshot catches up.
  // Keep the locally received payload when both sides have the same ID.
  for (const event of localEvents) mergedByID.set(event.id, event);
  const events = [...mergedByID.values()].sort(
    (left, right) => left.id - right.id
  );
  const mergedMessages = new Map(snapshot.messages.map((message) => [message.id, message]));
  if (localSnapshot?.session.id === snapshot.session.id) {
    for (const message of localSnapshot.messages) mergedMessages.set(message.id, message);
  }
  return {
    snapshot: {
      ...snapshot,
      events,
      messages: [...mergedMessages.values()]
        .filter((message) => !hiddenMessageIDs.includes(message.id))
        .sort((left, right) => left.created_at.localeCompare(right.created_at))
    },
    eventCursor: Math.max(0, ...events.map((event) => Number(event.id) || 0))
  };
}

export type DiagnosisStreamState = {
  text: string;
  turnBase: string;
  startedAt: number;
  generating: boolean;
  answerCompleted: boolean;
  liveProcessExpanded: boolean;
  interruptedReason: string;
};

export type DiagnosisStreamTransition = {
  state: DiagnosisStreamState;
  refresh: boolean;
};

function diagnosisFailureReason(
  eventType: string,
  payload: Record<string, unknown>
) {
  const reason = String(
    payload.error ?? payload.message ?? payload.error_message ?? ''
  ).trim();
  const timedOut =
    /(?:context\s+)?deadline exceeded/i.test(reason) ||
    String(payload.error_code ?? payload.code ?? '').toLowerCase() ===
      'timeout';
  const tokenBudget =
    String(payload.error_code ?? payload.code ?? '').toLowerCase() ===
      'token_budget' || /token budget exceeded/i.test(reason);
  if (timedOut) return '诊断执行超时，已保留已生成内容。';
  if (tokenBudget)
    return reason || '模型累计上下文预算已用尽，已保留已生成内容。';
  return (
    reason ||
    (eventType === 'execution.cancelled' || eventType === 'diagnosis.cancelled'
      ? '回答被取消。'
      : '回答生成失败。')
  );
}

export function reduceDiagnosisStreamEvent(
  previous: DiagnosisStreamState,
  eventType: string,
  payload: Record<string, unknown>,
  now = Date.now()
): DiagnosisStreamTransition {
  const state = { ...previous };
  let refresh = false;
  const ensureStarted = () => {
    state.startedAt ||= now;
  };

  if (eventType === 'model.started') {
    state.turnBase = state.text;
    ensureStarted();
    state.generating = true;
    state.interruptedReason = '';
  } else if (eventType === 'assistant.delta') {
    const text = String(payload.text ?? '');
    if (text) {
      state.text += text;
      ensureStarted();
    }
    state.generating = true;
  } else if (eventType === 'assistant.progress') {
    if (String(payload.kind ?? '') === 'tool_decision') {
      state.text = state.turnBase;
    }
    state.generating = true;
    ensureStarted();
  } else if (eventType === 'model.resumed') {
    state.text = '';
    state.turnBase = '';
    state.generating = true;
  } else if (eventType === 'assistant.completed') {
    const finalText = String(payload.text ?? '');
    if (finalText && !state.text) state.text = finalText;
    state.answerCompleted = true;
    state.interruptedReason = '';
    state.liveProcessExpanded = false;
    // The model has finished the visible answer. Report persistence and
    // causal-chain compilation may continue, but that is not answer generation
    // and must not leave the composer showing "正在思考".
    state.generating = false;
    refresh = true;
  } else if (eventType === 'context.compacted') {
    state.generating = true;
  } else if (
    eventType === 'execution.started' ||
    eventType === 'tool.requested' ||
    eventType === 'tool.started' ||
    eventType === 'tool.completed' ||
    eventType === 'tool.failed' ||
    eventType === 'phase.changed'
  ) {
    if (eventType === 'execution.started') {
      state.answerCompleted = false;
      state.interruptedReason = '';
    }
    if (eventType === 'tool.requested') state.text = state.turnBase;
    if (!state.answerCompleted) state.generating = true;
    ensureStarted();
  } else if (
    eventType === 'execution.failed' ||
    eventType === 'execution.cancelled' ||
    eventType === 'diagnosis.failed' ||
    eventType === 'diagnosis.cancelled'
  ) {
    state.interruptedReason = diagnosisFailureReason(eventType, payload);
    state.generating = false;
    state.answerCompleted = false;
    refresh = true;
  } else if (eventType === 'execution.completed') {
    refresh = true;
  } else if (eventType === 'report.ready') {
    // report.ready is the durable terminal marker. If the completion event
    // was lost or reordered, the composer must still leave the thinking state.
    state.generating = false;
    refresh = true;
  }

  return { state, refresh };
}
