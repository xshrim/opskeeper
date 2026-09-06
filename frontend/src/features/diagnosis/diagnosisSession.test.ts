import { afterEach, describe, expect, it, vi } from 'vitest';
import type { DiagnosisSnapshot } from '../../lib/api';
import {
  appendDiagnosisEvent,
  mergeDiagnosisSnapshot,
  openDiagnosisEventStream,
  reduceDiagnosisStreamEvent,
  type DiagnosisStreamState
} from './diagnosisSession';
import { diagnosisAssistantTimeline, diagnosisLiveTimeline } from './diagnosisTimelines';

class FakeEventSource {
  static current: FakeEventSource | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  listeners = new Map<string, (event: MessageEvent) => void>();
  close = vi.fn();

  constructor(_url: string | URL) {
    FakeEventSource.current = this;
  }

  addEventListener(type: string, listener: EventListener) {
    this.listeners.set(type, listener as (event: MessageEvent) => void);
  }

  emit(type: string) {
    this.listeners.get(type)?.({ type } as MessageEvent);
  }
}

const state: DiagnosisStreamState = {
  text: '',
  turnBase: '',
  startedAt: 0,
  generating: false,
  answerCompleted: false,
  liveProcessExpanded: true,
  interruptedReason: ''
};

function snapshot(
  overrides: Partial<DiagnosisSnapshot> = {}
): DiagnosisSnapshot {
  return {
    session: {
      id: 'session-1',
      scope_id: 'scope-1',
      status: 'analyzing',
      title: '诊断',
      error_code: '',
      error_message: '',
      started_at: '',
      created_at: '',
      updated_at: ''
    },
    targets: [],
    messages: [],
    evidence: [],
    runs: [],
    causal_chains: [],
    hypotheses: [],
    events: [],
    ...overrides
  };
}

describe('diagnosis session event handling', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    FakeEventSource.current = null;
  });

  it('does not report normal terminal stream closure as an error', () => {
    const onError = vi.fn();
    vi.stubGlobal('EventSource', FakeEventSource);
    const close = openDiagnosisEventStream(
      'session-1',
      0,
      () => '/events',
      () => {},
      onError
    );
    FakeEventSource.current?.emit('report.ready');
    FakeEventSource.current?.onerror?.();
    expect(onError).not.toHaveBeenCalled();
    close();
    expect(FakeEventSource.current?.close).toHaveBeenCalledOnce();
  });

  it('reports an unexpected stream error once', () => {
    const onError = vi.fn();
    vi.stubGlobal('EventSource', FakeEventSource);
    openDiagnosisEventStream('session-1', 0, () => '/events', () => {}, onError);
    FakeEventSource.current?.onerror?.();
    FakeEventSource.current?.onerror?.();
    expect(onError).toHaveBeenCalledOnce();
  });

  it('deduplicates SSE events and keeps causal data in snapshots', () => {
    const causal = {
      id: 'chain-1'
    } as DiagnosisSnapshot['causal_chains'][number];
    const current = snapshot({ causal_chains: [causal] });
    const next = appendDiagnosisEvent(
      current,
      'phase.changed',
      { phase: 'collecting' },
      2
    );
    expect(next?.events).toHaveLength(1);
    expect(appendDiagnosisEvent(next, 'phase.changed', {}, 2)).toBe(next);
    expect(next?.causal_chains).toEqual([causal]);
  });

  it('merges local live events and filters hidden edited messages', () => {
    const local = snapshot({
      messages: [
        {
          id: 'hidden',
          session_id: 'session-1',
          role: 'user',
          content: 'old',
          created_at: ''
        }
      ],
      events: [
        {
          id: 4,
          session_id: 'session-1',
          type: 'report.ready',
          payload: {},
          created_at: ''
        }
      ]
    });
    const remote = snapshot({
      session: { ...local.session, status: 'succeeded' },
      messages: [
        {
          id: 'hidden',
          session_id: 'session-1',
          role: 'user',
          content: 'old',
          created_at: ''
        }
      ],
      events: [
        {
          id: 3,
          session_id: 'session-1',
          type: 'assistant.delta',
          payload: {},
          created_at: ''
        }
      ]
    });
    const merged = mergeDiagnosisSnapshot(remote, local, ['hidden']);
    expect(merged.eventCursor).toBe(4);
    expect(merged.snapshot.events?.map((event) => event.id)).toEqual([3, 4]);
    expect(merged.snapshot.messages).toEqual([]);
  });

  it('keeps a locally appended follow-up when the refresh response is stale', () => {
    const local = snapshot({
      messages: [
        { id: 'question-1', session_id: 'session-1', role: 'user', content: 'first', created_at: '2026-01-01T00:00:00Z' },
        { id: 'question-2', session_id: 'session-1', role: 'user', content: 'follow-up', created_at: '2026-01-01T00:00:01Z' }
      ]
    });
    const remote = snapshot({
      session: { ...local.session, status: 'analyzing' },
      messages: local.messages.slice(0, 1)
    });
    const merged = mergeDiagnosisSnapshot(remote, local, []);
    expect(merged.snapshot.messages.map((message) => message.id)).toEqual(['question-1', 'question-2']);
  });

  it('preserves tool rollback, context compaction, and report completion transitions', () => {
    const started = reduceDiagnosisStreamEvent(state, 'model.started', {}, 100);
    const withCandidate = reduceDiagnosisStreamEvent(
      { ...started.state, text: '候选回答' },
      'assistant.progress',
      { kind: 'tool_decision' },
      110
    );
    expect(withCandidate.state.text).toBe('');
    expect(
      reduceDiagnosisStreamEvent(withCandidate.state, 'context.compacted', {})
        .state.generating
    ).toBe(true);
    const completed = reduceDiagnosisStreamEvent(
      withCandidate.state,
      'assistant.completed',
      {}
    );
    expect(completed.state.answerCompleted).toBe(true);
    expect(completed.state.generating).toBe(false);
    expect(completed.refresh).toBe(true);

    const reportReady = reduceDiagnosisStreamEvent(
      completed.state,
      'report.ready',
      {}
    );
    expect(reportReady.state.generating).toBe(false);
    expect(reportReady.state.answerCompleted).toBe(true);

    const reportWhileGenerating = reduceDiagnosisStreamEvent(
      { ...state, generating: true },
      'report.ready',
      {}
    );
    expect(reportWhileGenerating.state.generating).toBe(true);
    expect(reportWhileGenerating.state.answerCompleted).toBe(false);
  });

  it('keeps user cancellation distinct from ordinary failures', () => {
    const cancelled = reduceDiagnosisStreamEvent(
      state,
      'diagnosis.cancelled',
      {}
    );
    expect(cancelled.state.generating).toBe(false);
    expect(cancelled.state.answerCompleted).toBe(false);
    expect(cancelled.state.interruptedReason).toBe('回答被取消。');
    expect(cancelled.refresh).toBe(true);
  });

  it('does not show the previous execution while a follow-up is queued', () => {
    const current = snapshot({
      messages: [
        {
          id: 'question-1',
          session_id: 'session-1',
          role: 'user',
          content: 'first question',
          created_at: ''
        },
        {
          id: 'question-2',
          session_id: 'session-1',
          role: 'user',
          content: 'follow-up question',
          created_at: ''
        }
      ],
      events: [
        { id: 1, session_id: 'session-1', type: 'execution.started', payload: {}, created_at: '' },
        { id: 2, session_id: 'session-1', type: 'tool.requested', payload: { tool: 'old_tool', resource_id: 'r1', iteration: 1 }, created_at: '' },
        { id: 3, session_id: 'session-1', type: 'message.created', payload: { message_id: 'question-2', role: 'user' }, created_at: '' }
      ]
    });
    expect(diagnosisLiveTimeline(current)).toEqual([]);
  });

  it('keeps the previous tool execution available while a follow-up runs', () => {
    const current = snapshot({
      session: { ...snapshot().session, status: 'analyzing' },
      messages: [
        { id: 'question-1', session_id: 'session-1', role: 'user', content: 'first', created_at: '2026-01-01T00:00:00Z' },
        { id: 'answer-1', session_id: 'session-1', role: 'assistant', content: 'first answer', created_at: '2026-01-01T00:00:01Z' },
        { id: 'question-2', session_id: 'session-1', role: 'user', content: 'follow-up', created_at: '2026-01-01T00:00:02Z' }
      ],
      events: [
        { id: 1, session_id: 'session-1', type: 'execution.started', payload: {}, created_at: '2026-01-01T00:00:00Z' },
        { id: 2, session_id: 'session-1', type: 'tool.requested', payload: { tool: 'old_tool', resource_id: 'r1', iteration: 1 }, created_at: '2026-01-01T00:00:00Z' },
        { id: 3, session_id: 'session-1', type: 'tool.completed', payload: { tool: 'old_tool', resource_id: 'r1', iteration: 1, output: { ok: true } }, created_at: '2026-01-01T00:00:01Z' },
        { id: 4, session_id: 'session-1', type: 'assistant.completed', payload: { text: 'first answer' }, created_at: '2026-01-01T00:00:01Z' },
        { id: 5, session_id: 'session-1', type: 'message.created', payload: { message_id: 'question-2', role: 'user' }, created_at: '2026-01-01T00:00:02Z' },
        { id: 6, session_id: 'session-1', type: 'execution.started', payload: {}, created_at: '2026-01-01T00:00:03Z' },
        { id: 7, session_id: 'session-1', type: 'tool.requested', payload: { tool: 'new_tool', resource_id: 'r1', iteration: 1 }, created_at: '2026-01-01T00:00:03Z' },
        { id: 8, session_id: 'session-1', type: 'tool.completed', payload: { tool: 'new_tool', resource_id: 'r1', iteration: 1, output: { ok: true } }, created_at: '2026-01-01T00:00:04Z' }
      ]
    });
    const previous = diagnosisAssistantTimeline(current, 0);
    expect(previous.some((item) => item.actions?.some((action) => action.tool === 'old_tool'))).toBe(true);
    expect(previous.some((item) => item.actions?.some((action) => action.tool === 'new_tool'))).toBe(false);
    const currentAnswer = diagnosisAssistantTimeline({
      ...current,
      messages: [...current.messages, { id: 'answer-2', session_id: 'session-1', role: 'assistant', content: 'follow-up answer', created_at: '2026-01-01T00:00:04Z' }],
      session: { ...current.session, status: 'succeeded' }
    }, 1);
    expect(currentAnswer.some((item) => item.actions?.some((action) => action.tool === 'new_tool'))).toBe(true);
  });

});
