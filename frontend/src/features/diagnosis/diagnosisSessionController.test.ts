import { afterEach, describe, expect, it, vi } from 'vitest';
import type {
  DiagnosisMessage,
  DiagnosisSession,
  DiagnosisSnapshot
} from '../../lib/api';
import { createDiagnosisSessionController } from './diagnosisSessionController';

class ControllerEventSource {
  static current: ControllerEventSource | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  listeners = new Map<string, (event: MessageEvent) => void>();
  close = vi.fn();

  constructor(_url: string | URL) {
    ControllerEventSource.current = this;
  }

  addEventListener(type: string, listener: EventListener) {
    this.listeners.set(type, listener as (event: MessageEvent) => void);
  }

  emit(type: string, id: number, payload: Record<string, unknown>) {
    const event = {
      type,
      lastEventId: String(id),
      data: JSON.stringify(payload)
    } as MessageEvent;
    (this.listeners.get(type) ?? this.onmessage)?.(event);
  }
}

function message(id: string, role: DiagnosisMessage['role'], content: string): DiagnosisMessage {
  return { id, session_id: 'session-1', role, content, created_at: `2026-01-01T00:00:${id.slice(-1).padStart(2, '0')}Z` };
}

function snapshot(overrides: Partial<DiagnosisSnapshot> = {}): DiagnosisSnapshot {
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

function createHarness(remote: DiagnosisSnapshot) {
  let current = snapshot();
  let streamState = {
    text: '',
    turnBase: '',
    startedAt: 0,
    generating: true,
    answerCompleted: false,
    liveProcessExpanded: true,
    interruptedReason: ''
  };
  const updatedSessions: DiagnosisSession[] = [];
  const api = {
    diagnosisSessions: vi.fn(async () => []),
    diagnosisSession: vi.fn(async () => remote),
    diagnosisEventsURL: vi.fn(() => '/events'),
    cancelDiagnosis: vi.fn(async () => {}),
    startDiagnosis: vi.fn(),
    askDiagnosis: vi.fn(),
    addDiagnosisTarget: vi.fn(),
    deleteDiagnosis: vi.fn()
  };
  const controller = createDiagnosisSessionController({
    api,
    getSelectedDiagnosisId: () => 'session-1',
    getSnapshot: () => current,
    setSnapshot: (value) => (current = value),
    getHiddenMessageIds: () => [],
    getAssistantMessageBaseline: () => 0,
    setAssistantMessageBaseline: vi.fn(),
    resetStreamState: () => {},
    setSubmissionPending: () => {},
    isSubmissionPending: () => false,
    getStopRequested: () => false,
    setStopRequested: () => {},
    getStreamState: () => streamState,
    setStreamState: (value) => (streamState = value),
    updateSession: (value) => updatedSessions.push(value),
    onError: vi.fn(),
    formatError: (error) => String(error),
    isDiagnosisRunning: (status) => ['queued', 'planning', 'collecting', 'analyzing'].includes(status)
  });
  return { controller, api, getSnapshot: () => current, getStreamState: () => streamState, updatedSessions };
}

describe('diagnosis session controller lifecycle', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    ControllerEventSource.current = null;
  });

  it('reconciles a durable report terminal event and stops generation', async () => {
    vi.stubGlobal('EventSource', ControllerEventSource);
    const remote = snapshot({
      session: { ...snapshot().session, status: 'analyzing' },
      messages: [message('q1', 'user', '检查状态'), message('a1', 'assistant', '已完成')],
      runs: [{ id: 'run-1', session_id: 'session-1', sequence: 1, question_message_id: 'q1', status: 'succeeded', started_at: '' }],
      events: [
        { id: 1, session_id: 'session-1', type: 'execution.started', payload: {}, created_at: '' },
        { id: 2, session_id: 'session-1', type: 'assistant.completed', payload: { text: '已完成' }, created_at: '' },
        { id: 3, session_id: 'session-1', type: 'report.ready', payload: {}, created_at: '' }
      ]
    });
    const harness = createHarness(remote);
    harness.controller.syncCursor(snapshot({ events: remote.events?.slice(0, 1) }));
    harness.controller.open('session-1');
    ControllerEventSource.current?.emit('report.ready', 3, {});
    await vi.waitFor(() => expect(harness.getStreamState().generating).toBe(false));
    expect(harness.getSnapshot().session.status).toBe('succeeded');
    expect(harness.updatedSessions.at(-1)?.status).toBe('succeeded');
    expect(ControllerEventSource.current?.close).toHaveBeenCalled();
  });

  it('reopens the stream when an old terminal event races with a queued follow-up', async () => {
    vi.stubGlobal('EventSource', ControllerEventSource);
    const remote = snapshot({
      session: { ...snapshot().session, status: 'queued' },
      messages: [message('q1', 'user', '旧问题'), message('a1', 'assistant', '旧回答'), message('q2', 'user', '追问')],
      runs: [{ id: 'run-1', session_id: 'session-1', sequence: 1, question_message_id: 'q1', status: 'succeeded', started_at: '' }],
      events: [
        { id: 1, session_id: 'session-1', type: 'execution.started', payload: {}, created_at: '' },
        { id: 2, session_id: 'session-1', type: 'diagnosis.failed', payload: { message: '旧轮次失败' }, created_at: '' }
      ]
    });
    const harness = createHarness(remote);
    const initial = snapshot({
      messages: remote.messages,
      runs: remote.runs,
      events: remote.events?.slice(0, 1)
    });
    harness.controller.syncCursor(initial);
    harness.controller.open('session-1');
    const firstStream = ControllerEventSource.current;
    firstStream?.emit('diagnosis.failed', 2, { message: '旧轮次失败' });
    await vi.waitFor(() => expect(harness.getStreamState().generating).toBe(true));
    expect(harness.getStreamState().interruptedReason).toBe('');
    expect(ControllerEventSource.current).not.toBe(firstStream);
  });
});
