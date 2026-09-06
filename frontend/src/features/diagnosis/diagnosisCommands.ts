import type {
  DiagnosisMessage,
  DiagnosisSession,
  DiagnosisSnapshot
} from '../../lib/api';

type DiagnosisSessionActions = {
  addDiagnosisTarget: (
    sessionID: string,
    resourceID: string
  ) => Promise<unknown>;
  askDiagnosis: (
    sessionID: string,
    content: string
  ) => Promise<DiagnosisMessage>;
  deleteDiagnosis: (sessionID: string) => Promise<void>;
  deleteDiagnosisSessions: (sessionIDs: string[]) => Promise<unknown>;
  submit: <T>(
    operation: () => Promise<T>
  ) => Promise<{ result: T; stopRequested: boolean }>;
  startDiagnosis: (body: {
    scope_id: string;
    question: string;
    target_resource_ids: string[];
    ai_provider_resource_id?: string;
    model_name?: string;
  }) => Promise<DiagnosisSession>;
  refresh: (sessionID?: string) => Promise<void>;
  open: (sessionID: string) => void;
  openForLatestQuestion: (sessionID: string) => void;
  close: () => void;
  stop: () => Promise<void>;
};

export type DiagnosisCommandState = {
  scopeID: string;
  sessions: DiagnosisSession[];
  selectedSessionID: string;
  snapshot: DiagnosisSnapshot | null;
  composerText: string;
  targetIDs: string[];
  providerID: string;
  modelName: string;
  generating: boolean;
  stopRequested: boolean;
  submissionPending: boolean;
  interruptedReason: string;
  hiddenMessageIDs: string[];
  editingMessageID: string;
  editDraft: string;
};

export type DiagnosisCommandOptions = {
  session: DiagnosisSessionActions;
  getState: () => DiagnosisCommandState;
  updateState: (patch: Partial<DiagnosisCommandState>) => void;
  runAction: (operation: () => Promise<void>) => Promise<void>;
  openDiagnosis: (sessionID: string) => Promise<unknown>;
  refreshDiagnosis: (sessionID?: string) => Promise<void>;
  scrollToQuestion: (messageID?: string, content?: string) => Promise<void>;
  confirm: (message: string) => boolean;
  prompt: (message: string, initialValue: string) => string | null;
  onError: (message: string) => void;
  resetStreamState: () => void;
};

export function createDiagnosisCommands(options: DiagnosisCommandOptions) {
  async function startDiagnosis(question: string) {
    const state = options.getState();
    if (!state.scopeID) {
      options.onError('请先选择一个可用级别。');
      return;
    }
    const content = question.trim();
    if (!content) return;
    await options.runAction(async () => {
      const { result: session, stopRequested } = await options.session.submit(
        () =>
          options.session.startDiagnosis({
            scope_id: state.scopeID,
            question: content,
            target_resource_ids: state.targetIDs,
            ai_provider_resource_id: state.providerID || undefined,
            model_name: state.modelName || undefined
          })
      );
      options.updateState({
        sessions: [session, ...options.getState().sessions],
        targetIDs: [],
        interruptedReason: ''
      });
      await options.openDiagnosis(session.id);
      await options.scrollToQuestion('', content);
      if (stopRequested) {
        options.updateState({ stopRequested: false });
        await options.session.stop();
      }
    });
  }

  async function sendDiagnosisFollowup(question: string) {
    const state = options.getState();
    const content = question.trim();
    if (!state.selectedSessionID || !content) return;
    await options.runAction(async () => {
      const { result: created, stopRequested } = await options.session.submit(
        () => options.session.askDiagnosis(state.selectedSessionID, content)
      );
      const current = options.getState();
      options.updateState({
        interruptedReason: '',
        generating: true,
        snapshot: current.snapshot
          ? {
              ...current.snapshot,
              messages: current.snapshot.messages.some((message) => message.id === created.id)
                ? current.snapshot.messages
                : [...current.snapshot.messages, created]
            }
          : current.snapshot
      });
      await options.refreshDiagnosis();
      options.session.openForLatestQuestion(state.selectedSessionID);
      await options.scrollToQuestion(created.id, content);
      if (stopRequested) {
        options.updateState({ stopRequested: false });
        await options.session.stop();
      }
    });
  }

  async function submitDiagnosisMessage(
    composerText: string,
    setComposerText: (value: string) => void
  ) {
    const content = composerText.trim();
    const state = options.getState();
    if (state.generating || !content) return;
    if (!state.selectedSessionID) {
      await startDiagnosis(content);
    } else {
      await sendDiagnosisFollowup(content);
    }
    setComposerText('');
  }

  function toggleDiagnosisContext(resourceID: string) {
    const state = options.getState();
    if (state.targetIDs.includes(resourceID)) {
      options.updateState({
        targetIDs: state.targetIDs.filter((id) => id !== resourceID)
      });
      return;
    }
    if (state.targetIDs.length >= 20) {
      options.onError('一次诊断最多加载 20 个上下文资源。');
      return;
    }
    options.updateState({ targetIDs: [...state.targetIDs, resourceID] });
    if (state.selectedSessionID) {
      void options.runAction(async () => {
        await options.session.addDiagnosisTarget(
          state.selectedSessionID,
          resourceID
        );
        await options.refreshDiagnosis();
      });
    }
  }

  function newDiagnosisSession() {
    options.session.close();
    options.resetStreamState();
    options.updateState({
      selectedSessionID: '',
      snapshot: null,
      composerText: '',
      targetIDs: [],
      interruptedReason: '',
      generating: false,
      stopRequested: false,
      submissionPending: false,
      editingMessageID: '',
      editDraft: ''
    });
  }

  async function clearDiagnosisHistory() {
    const sessions = options.getState().sessions;
    await options.runAction(async () => {
      await options.session.deleteDiagnosisSessions(
        sessions.map((item) => item.id)
      );
      newDiagnosisSession();
      options.updateState({ sessions: [] });
    });
  }

  function renameDiagnosisSession(session: DiagnosisSession) {
    const title = options.prompt(
      '重命名诊断会话',
      session.title || '未命名诊断'
    );
    if (!title?.trim()) return;
    const nextTitle = title.trim();
    const state = options.getState();
    options.updateState({
      sessions: state.sessions.map((item) =>
        item.id === session.id ? { ...item, title: nextTitle } : item
      ),
      snapshot:
        state.snapshot?.session.id === session.id
          ? {
              ...state.snapshot,
              session: { ...state.snapshot.session, title: nextTitle }
            }
          : state.snapshot
    });
  }

  async function deleteDiagnosisSession(session: DiagnosisSession) {
    if (!options.confirm(`删除“${session.title || '未命名诊断'}”的列表记录？`))
      return;
    await options.runAction(async () => {
      await options.session.deleteDiagnosis(session.id);
      options.updateState({
        sessions: options
          .getState()
          .sessions.filter((item) => item.id !== session.id)
      });
      if (options.getState().selectedSessionID === session.id)
        newDiagnosisSession();
    });
  }

  function beginDiagnosisEdit(message: DiagnosisMessage) {
    options.updateState({
      editingMessageID: message.id,
      editDraft: message.content
    });
  }

  async function saveDiagnosisEdit() {
    const state = options.getState();
    const content = state.editDraft.trim();
    if (!content || !state.selectedSessionID || !state.snapshot) return;
    const originalID = state.editingMessageID;
    await options.runAction(async () => {
      const { result: created, stopRequested } = await options.session.submit(
        () => options.session.askDiagnosis(state.selectedSessionID, content)
      );
      const current = options.getState();
      options.updateState({
        hiddenMessageIDs: [...current.hiddenMessageIDs, originalID],
        snapshot: {
          ...current.snapshot!,
          messages: [
            ...current.snapshot!.messages.filter(
              (message) => message.id !== originalID
            ),
            created
          ]
        },
        editingMessageID: '',
        editDraft: '',
        interruptedReason: '',
        generating: true
      });
      options.session.openForLatestQuestion(state.selectedSessionID);
      await options.scrollToQuestion(created.id, content);
      if (stopRequested) {
        options.updateState({ stopRequested: false });
        await options.session.stop();
      }
    });
  }

  return {
    beginDiagnosisEdit,
    clearDiagnosisHistory,
    deleteDiagnosisSession,
    newDiagnosisSession,
    renameDiagnosisSession,
    saveDiagnosisEdit,
    sendDiagnosisFollowup,
    startDiagnosis,
    submitDiagnosisMessage,
    toggleDiagnosisContext
  };
}
