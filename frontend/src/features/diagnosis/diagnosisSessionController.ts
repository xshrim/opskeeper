import type {
  DiagnosisMessage,
  DiagnosisSession,
  DiagnosisSnapshot,
  DiagnosisTarget
} from '../../lib/api';
import {
  appendDiagnosisEvent,
  mergeDiagnosisSnapshot,
  openDiagnosisEventStream,
  reduceDiagnosisStreamEvent,
  type DiagnosisEventStream,
  type DiagnosisStreamState
} from './diagnosisSession';

type DiagnosisSessionAPI = {
  diagnosisSessions: (scopeID: string) => Promise<DiagnosisSession[]>;
  diagnosisSession: (id: string) => Promise<DiagnosisSnapshot>;
  diagnosisEventsURL: (id: string, after?: number) => string | URL;
  cancelDiagnosis: (id: string) => Promise<void>;
  startDiagnosis: (body: {
    scope_id: string;
    title?: string;
    question: string;
    target_resource_ids: string[];
    ai_provider_resource_id?: string;
    model_name?: string;
  }) => Promise<DiagnosisSession>;
  askDiagnosis: (id: string, content: string) => Promise<DiagnosisMessage>;
  addDiagnosisTarget: (
    sessionID: string,
    resourceID: string
  ) => Promise<DiagnosisTarget>;
  deleteDiagnosis: (id: string) => Promise<void>;
};

export type DiagnosisSessionControllerOptions = {
  api: DiagnosisSessionAPI;
  getSelectedDiagnosisId: () => string;
  getSnapshot: () => DiagnosisSnapshot | null;
  setSnapshot: (snapshot: DiagnosisSnapshot) => void;
  getHiddenMessageIds: () => string[];
  getAssistantMessageBaseline: () => number;
  setAssistantMessageBaseline: (count: number) => void;
  resetStreamState: () => void;
  setSubmissionPending: (pending: boolean) => void;
  isSubmissionPending: () => boolean;
  getStopRequested: () => boolean;
  setStopRequested: (requested: boolean) => void;
  getStreamState: () => DiagnosisStreamState;
  setStreamState: (state: DiagnosisStreamState) => void;
  updateSession: (session: DiagnosisSession) => void;
  onError: (error: unknown, fallback: string) => void;
  formatError: (error: unknown, fallback: string) => string;
  isDiagnosisRunning: (status: string) => boolean;
};

/**
 * Owns the asynchronous part of a diagnosis session while the diagnosis page
 * keeps the page-facing state. Keeping the callbacks narrow prevents the app
 * shell from knowing about SSE reconnection, event cursors, or refresh races.
 */
export function createDiagnosisSessionController(
  options: DiagnosisSessionControllerOptions
) {
  let eventStream: DiagnosisEventStream | null = null;
  let eventCursor = 0;
  const handledEventIds = new Set<number>();
  let refreshToken = 0;
  let stopping = false;

  function close() {
    eventStream?.();
    eventStream = null;
  }

  function syncCursor(snapshot: DiagnosisSnapshot | null) {
    if (!snapshot) return;
    eventCursor = Math.max(
      0,
      ...(snapshot.events ?? []).map((event) => Number(event.id) || 0)
    );
    handledEventIds.clear();
    for (const event of snapshot.events ?? []) {
      if (event.id > 0) handledEventIds.add(event.id);
    }
  }

  function syncCursorToLatestQuestion(snapshot: DiagnosisSnapshot | null = options.getSnapshot()) {
    if (!snapshot) return;
    const latestQuestion = [...snapshot.messages]
      .reverse()
      .find((message) => message.role === 'user');
    if (!latestQuestion) {
      syncCursor(snapshot);
      return;
    }
    const marker = [...(snapshot.events ?? [])]
      .filter((event) => {
        if (event.type !== 'message.created') return false;
        return String(event.payload?.message_id ?? '') === latestQuestion.id;
      })
      .sort((left, right) => left.id - right.id)
      .at(-1);
    if (marker) {
      eventCursor = Math.max(0, marker.id);
      handledEventIds.clear();
      for (const event of snapshot.events ?? []) {
        if (event.id > 0 && event.id <= eventCursor) handledEventIds.add(event.id);
      }
      return;
    }
    // Event persistence can lag the message row briefly. Keep the existing
    // cursor in that case instead of jumping past the current execution.
  }

  function hasPendingQuestion(snapshot: DiagnosisSnapshot | null) {
    if (!snapshot) return false;
    const latestMessage = [...snapshot.messages].reverse().find((message) => message.role === 'user');
    const latestRun = [...snapshot.runs].sort((left, right) => right.sequence - left.sequence)[0];
    return Boolean(latestMessage && latestMessage.id !== latestRun?.question_message_id);
  }

  function hasUnconsumedExecution(snapshot: DiagnosisSnapshot | null) {
    return Boolean(
      snapshot?.events?.some(
        (event) =>
          event.id > eventCursor &&
          (event.type === 'execution.started' || event.type === 'model.started')
      )
    );
  }

  function hasActiveLatestRun(snapshot: DiagnosisSnapshot | null) {
    if (!snapshot) return false;
    const latestQuestion = [...snapshot.messages]
      .reverse()
      .find((message) => message.role === 'user');
    const latestRun = [...snapshot.runs].sort(
      (left, right) => right.sequence - left.sequence
    )[0];
    return Boolean(
      latestQuestion &&
        latestRun?.status === 'running' &&
        latestRun.question_message_id === latestQuestion.id
    );
  }

  function terminalStatusFromEvents(snapshot: DiagnosisSnapshot | null) {
    if (!snapshot) return '' as DiagnosisSession['status'] | '';
    const latestExecutionStart = [...(snapshot.events ?? [])]
      .filter((item) => item.type === 'execution.started')
      .sort((left, right) => right.id - left.id)[0];
    const event = [...(snapshot.events ?? [])]
      .sort((left, right) => right.id - left.id)
      .find((item) =>
        ['report.ready', 'diagnosis.failed', 'diagnosis.cancelled'].includes(item.type)
      );
    // A terminal marker from an earlier run must not turn off the current
    // answer while its newer execution is still producing events.
    if (!event || (latestExecutionStart && event.id <= latestExecutionStart.id)) {
      return '' as DiagnosisSession['status'] | '';
    }
    if (event.type === 'report.ready') return 'succeeded' as const;
    if (event.type === 'diagnosis.failed') return 'failed' as const;
    return 'cancelled' as const;
  }

  function applyTerminalStatus(snapshot: DiagnosisSnapshot | null) {
    const status = terminalStatusFromEvents(snapshot);
    if (!snapshot || !status || hasPendingQuestion(snapshot) || hasActiveLatestRun(snapshot)) {
      return snapshot;
    }
    if (snapshot.session.status === status) return snapshot;
    return { ...snapshot, session: { ...snapshot.session, status } };
  }

  function handleEvent(sessionID: string, event: MessageEvent) {
    if (sessionID !== options.getSelectedDiagnosisId()) return;
    const eventID = Number(event.lastEventId) || 0;
    if (eventID > 0 && handledEventIds.has(eventID)) return;
    if (eventID > 0) {
      handledEventIds.add(eventID);
      eventCursor = Math.max(eventCursor, eventID);
    }
    let payload: Record<string, unknown> = {};
    try {
      payload = event.data ? JSON.parse(event.data) : {};
    } catch {
      payload = {};
    }
    const eventType = event.type || 'message';
    const terminal =
      eventType === 'report.ready' ||
      eventType === 'diagnosis.failed' ||
      eventType === 'diagnosis.cancelled';
    const nextSnapshot = appendDiagnosisEvent(
      options.getSnapshot(),
      eventType,
      payload,
      eventCursor
    );
    if (nextSnapshot && nextSnapshot !== options.getSnapshot()) {
      options.setSnapshot(nextSnapshot);
    }
    if (terminal) {
      const terminalSnapshot = applyTerminalStatus(options.getSnapshot());
      if (terminalSnapshot && terminalSnapshot !== options.getSnapshot()) {
        options.setSnapshot(terminalSnapshot);
        options.updateSession(terminalSnapshot.session);
      }
    }

    const transition = reduceDiagnosisStreamEvent(
      options.getStreamState(),
      eventType,
      payload
    );
    options.setStreamState(transition.state);
    if (!transition.refresh) return;

    const refreshPromise = refresh(sessionID);
    if (terminal) {
      void refreshPromise.finally(() => {
        const snapshot = options.getSnapshot();
        // The terminal event can belong to the previous run. Ask may already
        // have queued a follow-up, or the orchestrator may have claimed it by
        // the time refresh completes. Keep/reopen SSE in both cases.
        if (
          hasPendingQuestion(snapshot) ||
          hasActiveLatestRun(snapshot) ||
          hasUnconsumedExecution(snapshot)
        ) {
          options.setStreamState({
            ...options.getStreamState(),
            generating: true,
            interruptedReason: ''
          });
          // The current SSE connection closes on the terminal event. Reopen
          // after refresh so a queued follow-up can deliver its new run.
          open(sessionID);
          return;
        }
        // Terminal events close the transport. Answer state is owned by the
        // reducer: report.ready must not alter the conversation.
        close();
      });
    }
  }

  function open(sessionID: string) {
    close();
    eventStream = openDiagnosisEventStream(
      sessionID,
      eventCursor,
      options.api.diagnosisEventsURL,
      (event) => handleEvent(sessionID, event),
      (error) => options.onError(error, '诊断实时连接失败，请检查 API 服务和网络配置。')
    );
  }

  function openForLatestQuestion(sessionID: string) {
    syncCursorToLatestQuestion();
    open(sessionID);
  }

  async function refresh(sessionID = options.getSelectedDiagnosisId()) {
    if (!sessionID || sessionID !== options.getSelectedDiagnosisId()) return;
    const currentRefreshToken = ++refreshToken;
    try {
      const snapshot = await options.api.diagnosisSession(sessionID);
      if (
        currentRefreshToken !== refreshToken ||
        sessionID !== options.getSelectedDiagnosisId()
      ) {
        return;
      }
      const merged = mergeDiagnosisSnapshot(
        snapshot,
        options.getSnapshot(),
        options.getHiddenMessageIds()
      );
      // Do not advance the live SSE cursor from a database refresh. The
      // snapshot may already contain events that arrived between the user's
      // follow-up request and this refresh, but those events have not passed
      // through `handleEvent` and therefore have not updated stream state.
      // Advancing here would make the next EventSource start after the
      // current turn and leave the UI stuck in its pre-refresh state. The
      // cursor is advanced only by consumed SSE events (or `syncCursor` when
      // a session is initially opened).
      options.setSnapshot(merged.snapshot);

      const durableTerminal = applyTerminalStatus(merged.snapshot);
      if (durableTerminal && durableTerminal !== merged.snapshot) {
        options.setSnapshot(durableTerminal);
        merged.snapshot = {
          ...merged.snapshot,
          ...durableTerminal,
          events: durableTerminal.events ?? merged.snapshot.events
        };
      }

      const state = options.getStreamState();
      // The page stores the baseline outside this controller. A persisted
      // answer is detected by the live answer state rather than resetting the
      // stream text during the refresh race.
      // A refresh can observe the durable session a little before the
      // orchestrator changes its status. Once assistant.completed was seen,
      // that stale `analyzing` status must not reopen the thinking indicator.
      const pendingQuestion = hasPendingQuestion(merged.snapshot);
      const activeLatestRun = hasActiveLatestRun(merged.snapshot);
      const hasExecutionStart = (merged.snapshot.events ?? []).some(
        (event) => event.type === 'execution.started'
      );
      const terminalSession =
        !activeLatestRun &&
        ['succeeded', 'failed', 'cancelled'].includes(merged.snapshot.session.status) &&
        (!hasExecutionStart || Boolean(terminalStatusFromEvents(merged.snapshot)));
      const activeGeneration =
        !terminalSession &&
        !state.answerCompleted &&
        (options.isDiagnosisRunning(merged.snapshot.session.status) || state.generating);
      const nextGenerating =
        !state.interruptedReason &&
        (pendingQuestion || hasUnconsumedExecution(merged.snapshot) || activeGeneration);
      options.setStreamState({ ...state, generating: nextGenerating });
      options.updateSession(merged.snapshot.session);

      if (
        merged.snapshot.session.status === 'succeeded' ||
        merged.snapshot.session.status === 'failed' ||
        merged.snapshot.session.status === 'cancelled'
      ) {
        // A follow-up is stored before its queued run starts. Do not close the
        // stream on the previous run's terminal snapshot while that question
        // is still waiting to be claimed.
        if (pendingQuestion || hasUnconsumedExecution(merged.snapshot)) return;
        close();
      }
    } catch (error) {
      options.onError(error, '诊断状态刷新失败');
    }
  }

  async function stop() {
    const state = options.getStreamState();
    if (!state.generating || stopping) return;
    const interruptedState = {
      ...state,
      generating: false,
      interruptedReason: '用户手动停止了当前回答。'
    };
    if (options.isSubmissionPending()) {
      options.setStopRequested(true);
      options.setStreamState(interruptedState);
      return;
    }
    const sessionID = options.getSelectedDiagnosisId();
    if (!sessionID) {
      options.setStopRequested(true);
      options.setStreamState(interruptedState);
      return;
    }
    stopping = true;
    close();
    options.setStreamState(interruptedState);
    try {
      await options.api.cancelDiagnosis(sessionID);
    } catch (error) {
      options.setStreamState({
        ...options.getStreamState(),
        interruptedReason: `停止请求失败：${options.formatError(
          error,
          '无法停止后台执行'
        )}`
      });
    } finally {
      stopping = false;
    }
  }

  function prepareSubmission() {
    const snapshot = options.getSnapshot();
    options.setAssistantMessageBaseline(
      snapshot
        ? snapshot.messages.filter((message) => message.role === 'assistant')
            .length
        : 0
    );
    options.resetStreamState();
    options.setStopRequested(false);
    options.setSubmissionPending(true);
    options.setStreamState({
      ...options.getStreamState(),
      generating: true
    });
  }

  async function submit<T>(operation: () => Promise<T>) {
    prepareSubmission();
    try {
      const result = await operation();
      options.setSubmissionPending(false);
      return {
        result,
        stopRequested: options.getStopRequested()
      };
    } catch (error) {
      options.setSubmissionPending(false);
      options.setStreamState({
        ...options.getStreamState(),
        generating: false
      });
      throw error;
    }
  }

  function startDiagnosis(
    body: Parameters<DiagnosisSessionAPI['startDiagnosis']>[0]
  ) {
    return options.api.startDiagnosis(body);
  }

  function loadSessions(scopeID: string) {
    return options.api.diagnosisSessions(scopeID);
  }

  function loadSnapshot(sessionID: string) {
    return options.api.diagnosisSession(sessionID);
  }

  function askDiagnosis(sessionID: string, content: string) {
    return options.api.askDiagnosis(sessionID, content);
  }

  function addDiagnosisTarget(sessionID: string, resourceID: string) {
    return options.api.addDiagnosisTarget(sessionID, resourceID);
  }

  function deleteDiagnosis(sessionID: string) {
    return options.api.deleteDiagnosis(sessionID);
  }

  function deleteDiagnosisSessions(sessionIDs: string[]) {
    return Promise.all(
      sessionIDs.map((sessionID) => deleteDiagnosis(sessionID))
    );
  }

  return {
    addDiagnosisTarget,
    askDiagnosis,
    close,
    deleteDiagnosis,
    deleteDiagnosisSessions,
    loadSessions,
    loadSnapshot,
    open,
    openForLatestQuestion,
    refresh,
    stop,
    submit,
    startDiagnosis,
    syncCursor,
    syncCursorToLatestQuestion
  };
}
