<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import {
    Bot,
    ChevronDown,
    ChevronLeft,
    ChevronRight,
    Copy,
    Link2,
    MessageSquarePlus,
    Paperclip,
    Pencil,
    Plus,
    Send,
    Sparkles,
    Square,
    Stethoscope,
    Trash2,
    Wrench
  } from 'lucide-svelte';
  import type {
    ProviderAvailability,
    DiagnosisEvidence,
    DiagnosisMessage,
    DiagnosisSession,
    DiagnosisSnapshot,
    Application,
    Resource,
    SkillCatalogItem,
    PersonaCatalogItem,
    DiagnosisAttachment
  } from '../../lib/api';
  import {
    api,
    type DiagnosisStatus
  } from '../../lib/api';
  import { providerModelSelection } from '../../lib/workspace';
  import { renderDiagnosisMarkdown } from './diagnosisMarkdown';
  import { createDiagnosisSessionController } from './diagnosisSessionController';
  import {
    createDiagnosisCommands,
    type DiagnosisCommandState
  } from './diagnosisCommands';
  import DiagnosisContextPanel from './DiagnosisContextPanel.svelte';
  import SearchInput from '../../components/SearchInput.svelte';
  import DiagnosisConversationMessages from './DiagnosisConversationMessages.svelte';
  import {
    diagnosisActionLabel,
    diagnosisCausalEvidenceIDs,
    diagnosisCausalNodes,
    diagnosisHasRunningActions,
    diagnosisStatusLabel
  } from './diagnosisUtils';
  import {
    diagnosisEvidenceTimeline,
    diagnosisLiveTimeline,
    diagnosisAssistantTimeline
  } from './diagnosisTimelines';
  import {
    diagnosisEvidenceSourceTools as getDiagnosisEvidenceSourceTools,
    diagnosisEvidenceSummary as getDiagnosisEvidenceSummary,
    diagnosisHasPersistedNewAnswer as getDiagnosisHasPersistedNewAnswer,
    diagnosisProcessText as getDiagnosisProcessText,
    diagnosisResourceName as getDiagnosisResourceName,
    activeDiagnosisCausalChain,
    decodeDiagnosisCode,
    diagnosisMessageClipboardText,
    diagnosisStreamingClipboardText,
    isLastDiagnosisUser as getIsLastDiagnosisUser,
    shouldShowEmptyDiagnosisAnswer
  } from './diagnosisPresentation';
  import {
    installDiagnosisDocumentListeners,
    startDiagnosisPanelResize,
    copyDiagnosisText
  } from './diagnosisInteraction';

  type LiveTimelineItem = {
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
    actions?: LiveTimelineItem[];
  };
  type Provider = ProviderAvailability;

  let diagnosisHistoryCollapsed = false;
  let diagnosisContextCollapsed = false;
  let diagnosisHistoryWidth = 232;
  let diagnosisContextWidth = 275;
  let diagnosisSessions: DiagnosisSession[] = [];
  let selectedDiagnosisId = '';
  let diagnosisSessionSearch = '';
  export let formatDate: (value: string) => string;
  export let scopeId = '';
  export let applicationId = '';
  export let runAction: (operation: () => Promise<void>) => Promise<void>;
  export let describeError: (error: unknown, fallback: string) => string;
  export let onError: (message: string) => void;
  let diagnosisSnapshot: DiagnosisSnapshot | null = null;
  export let scopeLabel = '当前级别';
  export let resources: Resource[] = [];
  export let contextResources: Resource[] = [];
  export let projectId = '';
  export let resourceInActiveWorkspace: (resource: Resource) => boolean;
  let diagnosisTargetIds: string[] = [];
  let diagnosisApplicationIds: string[] = [];
  let diagnosisApplications: Application[] = [];
  let diagnosisApplicationCandidates: Application[] = [];
  let selectedApplicationResourceIDs = new Set<string>();
  let diagnosisGenerating = false;
  export let busy = false;
  let diagnosisMessageListElement: HTMLDivElement | null = null;
  let diagnosisAnswerCompleted = false;
  let diagnosisStreamingText = '';
  let diagnosisInterruptedReason = '';
  let diagnosisEditingMessageId = '';
  let diagnosisEditDraft = '';
  let diagnosisProcessExpanded: Record<string, boolean> = {};
  let diagnosisActionExpanded: Record<string, boolean> = {};
  let diagnosisLiveProcessExpanded = false;
  let diagnosisStreamingStartedAt = 0;
  let diagnosisStreamingAssistantBaseline = 0;
  let diagnosisComposerText = '';
  let diagnosisComposerMenu: 'skill' | 'persona' | '' = '';
  let diagnosisComposerMenuSearch = '';
  let diagnosisComposerTokenStart = -1;
  let diagnosisComposerTextarea: HTMLTextAreaElement | null = null;
  let diagnosisComposerFileInput: HTMLInputElement | null = null;
  let diagnosisComposerSkillID = '';
  let diagnosisComposerPersonaID = '';
  let diagnosisComposerLinks: string[] = [];
  let diagnosisComposerAttachments: DiagnosisAttachment[] = [];
  let diagnosisAvailableProviders: Provider[] = [];
  let selectedProviderId = '';
  let llmModelName = '';
  let selectedDiagnosisModel: Provider['models'][number] | undefined;
  export let onNotice: (message: string) => void;
  export let skills: SkillCatalogItem[] = [];
  export let personas: PersonaCatalogItem[] = [];

  let diagnosisContextTab: 'context' | 'evidence' = 'context';
  export let resourceIcon: (kind: string) => string;
  export let resourceSchemaName: (kind: string) => string;
  export let scopeName: (id: string) => string;

  let diagnosisLoadedScopeId = '';
  let diagnosisLoadedProjectId = '';
  let diagnosisTargets: Resource[] = [];
  let diagnosisStopRequested = false;
  let diagnosisSubmissionPending = false;
  let diagnosisStreamingTurnBase = '';
  let diagnosisAutoScrollKey = '';
  let diagnosisAutoScrollEnabled = true;
  let diagnosisScrollListenerElement: HTMLDivElement | null = null;
  let diagnosisScrollListenerCleanup: (() => void) | null = null;
  let diagnosisProgrammaticScrollUntil = 0;
  let diagnosisQuestionAnchorPending = false;
  let diagnosisQuestionAnchorActive = false;
  let diagnosisHiddenMessageIds: string[] = [];

  function isDiagnosisRunning(status: DiagnosisStatus | string) {
    return ['queued', 'planning', 'collecting', 'analyzing'].includes(status);
  }

  function resetDiagnosisStreamState() {
    diagnosisAnswerCompleted = false;
    diagnosisLiveProcessExpanded = false;
    diagnosisStreamingText = '';
    diagnosisStreamingTurnBase = '';
    diagnosisStreamingStartedAt = 0;
    diagnosisQuestionAnchorActive = false;
    diagnosisAutoScrollEnabled = true;
  }

  const diagnosisSessionController = createDiagnosisSessionController({
    api,
    getSelectedDiagnosisId: () => selectedDiagnosisId,
    getSnapshot: () => diagnosisSnapshot,
    setSnapshot: (snapshot) => (diagnosisSnapshot = snapshot),
    getHiddenMessageIds: () => diagnosisHiddenMessageIds,
    getAssistantMessageBaseline: () => diagnosisStreamingAssistantBaseline,
    setAssistantMessageBaseline: (count) => (diagnosisStreamingAssistantBaseline = count),
    resetStreamState: resetDiagnosisStreamState,
    setSubmissionPending: (pending) => (diagnosisSubmissionPending = pending),
    getStreamState: () => ({
      text: diagnosisStreamingText,
      turnBase: diagnosisStreamingTurnBase,
      startedAt: diagnosisStreamingStartedAt,
      generating: diagnosisGenerating,
      answerCompleted: diagnosisAnswerCompleted,
      liveProcessExpanded: diagnosisLiveProcessExpanded,
      interruptedReason: diagnosisInterruptedReason
    }),
    setStreamState: (state) => {
      diagnosisStreamingText = state.text;
      diagnosisStreamingTurnBase = state.turnBase;
      diagnosisStreamingStartedAt = state.startedAt;
      diagnosisGenerating = state.generating;
      diagnosisAnswerCompleted = state.answerCompleted;
      diagnosisLiveProcessExpanded = state.liveProcessExpanded;
      diagnosisInterruptedReason = state.interruptedReason;
    },
    updateSession: (session) => {
      diagnosisSessions = diagnosisSessions.map((item) =>
        item.id === session.id ? session : item
      );
    },
    onError: (error, fallback) => onError(describeError(error, fallback)),
    formatError: describeError,
    isSubmissionPending: () => diagnosisSubmissionPending,
    getStopRequested: () => diagnosisStopRequested,
    setStopRequested: (requested) => (diagnosisStopRequested = requested),
    isDiagnosisRunning
  });

  function getDiagnosisCommandState(): DiagnosisCommandState {
    return {
      scopeID: scopeId,
      applicationID: applicationId,
      sessions: diagnosisSessions,
      selectedSessionID: selectedDiagnosisId,
      snapshot: diagnosisSnapshot,
      composerText: diagnosisComposerText,
      skillID: diagnosisComposerSkillID,
      personaID: diagnosisComposerPersonaID,
      links: diagnosisComposerLinks,
      attachments: diagnosisComposerAttachments,
      targetIDs: diagnosisTargetIds,
      providerID: selectedProviderId,
      modelName: llmModelName,
      generating: diagnosisGenerating,
      stopRequested: diagnosisStopRequested,
      submissionPending: diagnosisSubmissionPending,
      interruptedReason: diagnosisInterruptedReason,
      hiddenMessageIDs: diagnosisHiddenMessageIds,
      editingMessageID: diagnosisEditingMessageId,
      editDraft: diagnosisEditDraft
    };
  }

  function updateDiagnosisCommandState(patch: Partial<DiagnosisCommandState>) {
    if (patch.sessions !== undefined) diagnosisSessions = patch.sessions;
    if (patch.selectedSessionID !== undefined) selectedDiagnosisId = patch.selectedSessionID;
    if ('snapshot' in patch) diagnosisSnapshot = patch.snapshot ?? null;
    if (patch.composerText !== undefined) diagnosisComposerText = patch.composerText;
    if (patch.skillID !== undefined) diagnosisComposerSkillID = patch.skillID;
    if (patch.personaID !== undefined) diagnosisComposerPersonaID = patch.personaID;
    if (patch.links !== undefined) diagnosisComposerLinks = patch.links;
    if (patch.attachments !== undefined) diagnosisComposerAttachments = patch.attachments;
    if (patch.targetIDs !== undefined) diagnosisTargetIds = patch.targetIDs;
    if (patch.providerID !== undefined) selectedProviderId = patch.providerID;
    if (patch.modelName !== undefined) llmModelName = patch.modelName;
    if (patch.generating !== undefined) diagnosisGenerating = patch.generating;
    if (patch.stopRequested !== undefined) diagnosisStopRequested = patch.stopRequested;
    if (patch.submissionPending !== undefined) diagnosisSubmissionPending = patch.submissionPending;
    if (patch.interruptedReason !== undefined) diagnosisInterruptedReason = patch.interruptedReason;
    if (patch.hiddenMessageIDs !== undefined) diagnosisHiddenMessageIds = patch.hiddenMessageIDs;
    if (patch.editingMessageID !== undefined) diagnosisEditingMessageId = patch.editingMessageID;
    if (patch.editDraft !== undefined) diagnosisEditDraft = patch.editDraft;
  }

  const diagnosisCommands = createDiagnosisCommands({
    session: diagnosisSessionController,
    getState: getDiagnosisCommandState,
    updateState: updateDiagnosisCommandState,
    runAction,
    openDiagnosis,
    refreshDiagnosis,
    scrollToQuestion: scrollDiagnosisQuestionIntoView,
    resetStreamState: resetDiagnosisStreamState,
    confirm: (message) => window.confirm(message),
    prompt: (message, initialValue) => window.prompt(message, initialValue),
    onError,
    setAssistantMessageBaseline: (count) => (diagnosisStreamingAssistantBaseline = count)
  });

  async function loadDiagnosis() {
    if (!scopeId) return;
    if (projectId) {
      try {
        const result = await api.applications(projectId);
        diagnosisApplicationCandidates = result.items.filter((item) => item.status !== 'disabled');
        if (applicationId && !diagnosisApplicationCandidates.some((item) => item.id === applicationId)) applicationId = '';
      } catch (error) {
        diagnosisApplicationCandidates = [];
        onError(describeError(error, '可用应用加载失败'));
      }
    } else {
      diagnosisApplicationCandidates = [];
    }
    try {
      diagnosisAvailableProviders = await api.availableProviders(scopeId, 'diagnosis');
      if (
        diagnosisAvailableProviders.length > 0 &&
        (!selectedProviderId ||
          !diagnosisAvailableProviders.some((item) => item.provider_id === selectedProviderId))
      ) {
        selectedProviderId = diagnosisAvailableProviders[0].provider_id;
        llmModelName = diagnosisAvailableProviders[0].models[0]?.name ?? '';
      }
    } catch (error) {
      diagnosisAvailableProviders = [];
      onError(describeError(error, '可用渠道加载失败'));
    }
    await loadDiagnosisSessions();
  }

  async function loadDiagnosisSessions() {
    if (!scopeId) return;
    try {
      diagnosisSessions = await diagnosisSessionController.loadSessions(scopeId);
      if (!selectedDiagnosisId && diagnosisSessions[0]) await openDiagnosis(diagnosisSessions[0].id);
    } catch (error) {
      onError(describeError(error, '诊断会话历史加载失败'));
    }
  }

  async function openDiagnosis(id: string) {
    diagnosisSessionController.close();
    selectedDiagnosisId = id;
    diagnosisEditingMessageId = '';
    diagnosisEditDraft = '';
    diagnosisInterruptedReason = '';
    resetDiagnosisStreamState();
    try {
      diagnosisSnapshot = await diagnosisSessionController.loadSnapshot(id);
      diagnosisSnapshot = {
        ...diagnosisSnapshot,
        messages: diagnosisSnapshot.messages.filter((message) => !diagnosisHiddenMessageIds.includes(message.id))
      };
      diagnosisStreamingAssistantBaseline = diagnosisSnapshot.messages.filter((message) => message.role === 'assistant').length;
      diagnosisTargetIds = diagnosisSnapshot.targets.map((target) => target.resource_id);
      diagnosisApplicationIds = diagnosisSnapshot.session.application_id ? [diagnosisSnapshot.session.application_id] : [];
      if (diagnosisSnapshot.session. provider_id) selectedProviderId = diagnosisSnapshot.session. provider_id;
      if (diagnosisSnapshot.session.model_name) llmModelName = diagnosisSnapshot.session.model_name;
      applicationId = diagnosisSnapshot.session.application_id ?? '';
      diagnosisGenerating = isDiagnosisRunning(diagnosisSnapshot.session.status);
      diagnosisSessionController.syncCursor(diagnosisSnapshot);
      // Reconcile a snapshot that may already contain a terminal event while
      // its session row still reports an active phase, then subscribe for any
      // events written after this baseline.
      await diagnosisSessionController.refresh(id);
      diagnosisSessionController.open(id);
    } catch (error) {
      onError(describeError(error, '诊断详情加载失败'));
    }
  }

  async function refreshDiagnosis(id = selectedDiagnosisId) {
    await diagnosisSessionController.refresh(id);
  }

  async function submitDiagnosisMessage() {
    diagnosisAutoScrollEnabled = true;
    diagnosisQuestionAnchorPending = true;
    try {
      await diagnosisCommands.submitDiagnosisMessage(diagnosisComposerText, (value) => (diagnosisComposerText = value));
    } finally {
      diagnosisQuestionAnchorPending = false;
    }
  }

  function updateDiagnosisComposer(value: string, element: HTMLTextAreaElement) {
    diagnosisComposerText = value;
    const before = value.slice(0, element.selectionStart);
    const match = before.match(/(?:^|\s)([\/@])([^\s]*)$/);
    if (!match) {
      diagnosisComposerMenu = '';
      diagnosisComposerMenuSearch = '';
      diagnosisComposerTokenStart = -1;
      return;
    }
    diagnosisComposerMenu = match[1] === '/' ? 'skill' : 'persona';
    diagnosisComposerMenuSearch = match[2];
    diagnosisComposerTokenStart = before.length - match[0].length + (match[0].startsWith(' ') ? 1 : 0);
  }

  function chooseComposerItem(kind: 'skill' | 'persona', item: SkillCatalogItem | PersonaCatalogItem) {
    const textarea = diagnosisComposerTextarea;
    const start = diagnosisComposerTokenStart;
    if (!textarea || start < 0) return;
    const skill = kind === 'skill' ? item as SkillCatalogItem : null;
    const persona = kind === 'persona' ? item as PersonaCatalogItem : null;
    const tokenValue = skill?.identifier ?? persona?.name.replace(/\s+/g, '-') ?? '';
    const token = `${kind === 'skill' ? '/' : '@'}${tokenValue}`;
    const cursor = textarea.selectionStart;
    const selectedID = kind === 'skill' ? diagnosisComposerSkillID : diagnosisComposerPersonaID;
    const previous = kind === 'skill'
      ? skills.find((candidate) => candidate.id === selectedID)
      : personas.find((candidate) => candidate.id === selectedID);
    const previousToken = kind === 'skill'
      ? previous ? `/${(previous as SkillCatalogItem).identifier}` : ''
      : previous ? `@${(previous as PersonaCatalogItem).name.replace(/\s+/g, '-')}` : '';
    const currentSelected = selectedID === item.id;
    let baseText = diagnosisComposerText;
    let insertion = Math.max(0, start >= 0 ? start : cursor);
    if (previousToken) {
      const previousIndex = baseText.indexOf(previousToken);
      if (previousIndex >= 0) {
        baseText = `${baseText.slice(0, previousIndex)}${baseText.slice(previousIndex + previousToken.length)}`;
        if (previousIndex < insertion) insertion -= previousToken.length;
      }
    }
    if (start >= 0 && cursor >= start) {
      const query = baseText.slice(start, cursor);
      if (/^[/@]/.test(query)) {
        baseText = `${baseText.slice(0, start)}${baseText.slice(cursor)}`;
        insertion = start;
      }
    }
    const nextText = baseText.replace(/\s{2,}/g, ' ').trimStart();
    if (currentSelected) {
      diagnosisComposerText = nextText;
      if (kind === 'skill') diagnosisComposerSkillID = '';
      else diagnosisComposerPersonaID = '';
    } else {
      const insertAt = Math.min(insertion, nextText.length);
      diagnosisComposerText = `${nextText.slice(0, insertAt)}${token} ${nextText.slice(insertAt)}`;
      if (kind === 'skill') diagnosisComposerSkillID = item.id;
      else diagnosisComposerPersonaID = item.id;
      diagnosisComposerTokenStart = insertAt + token.length + 1;
    }
    diagnosisComposerMenu = '';
    diagnosisComposerMenuSearch = '';
    requestAnimationFrame(() => {
      if (!currentSelected) {
        textarea.focus();
        textarea.setSelectionRange(diagnosisComposerTokenStart, diagnosisComposerTokenStart);
      }
    });
  }

  function addDiagnosisLink() {
    const value = window.prompt('输入链接地址');
    if (!value?.trim()) return;
    if (!/^https?:\/\//i.test(value.trim())) {
      onError('链接地址必须以 http:// 或 https:// 开头。');
      return;
    }
    diagnosisComposerLinks = [...diagnosisComposerLinks, value.trim()];
  }

  async function addDiagnosisAttachment(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    input.value = '';
    const accepted = /\.(txt|log|json|ya?ml|md|csv|xml|sql|toml|ini|conf)$/i;
    const currentSize = diagnosisComposerAttachments.reduce((sum, item) => sum + item.content.length, 0);
    if (diagnosisComposerAttachments.length + files.length > 10) {
      onError('每条诊断消息最多添加 10 个附件。');
      return;
    }
    let usedBytes = currentSize;
    const additions: DiagnosisAttachment[] = [];
    for (const file of files) {
      if (!accepted.test(file.name)) {
        onError(`${file.name} 不是支持的文本、日志或配置文件。`);
        continue;
      }
      if (usedBytes + file.size > 128 * 1024) {
        onError('文本附件总大小不能超过 128 KiB。');
        break;
      }
      const content = await file.text();
      usedBytes += content.length;
      additions.push({ name: file.name, media_type: file.type, content });
    }
    diagnosisComposerAttachments = [...diagnosisComposerAttachments, ...additions];
  }

  function modelCapabilityTooltip() {
    const labels: Record<string, string> = { text: '文本', stream: '流式', tool_calling: '工具调用', reasoning: '推理', embedding: '向量', audio: '音频', vision: '视觉', structured_output: '结构化输出' };
    const tags: string[] = selectedDiagnosisModel?.capabilities ?? [];
    return tags.length ? `模型能力：${tags.map((tag) => labels[tag] ?? tag).join('、')}` : '模型能力：未声明';
  }

  function handleDiagnosisComposerKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && diagnosisComposerMenu) {
      event.preventDefault();
      diagnosisComposerMenu = '';
      diagnosisComposerMenuSearch = '';
      return;
    }
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      void submitDiagnosisMessage();
    }
  }

  async function scrollDiagnosisQuestionIntoView(messageID = '', content = '') {
    diagnosisAutoScrollEnabled = true;
    diagnosisQuestionAnchorPending = true;
    try {
      await tick();
      if (typeof requestAnimationFrame === 'function') await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
      const list = diagnosisMessageListElement ?? document.querySelector<HTMLDivElement>('.diagnosis-message-list-f');
      if (!list) return;
      const candidates = Array.from(list.querySelectorAll<HTMLElement>('[data-diagnosis-message-id]'));
      const target = [...candidates].reverse().find((item) =>
        (messageID && item.dataset.diagnosisMessageId === messageID) ||
        Boolean(content && item.dataset.diagnosisMessageContent === content)
      );
      if (!target) return;
      list.style.removeProperty('padding-bottom');
      const listRect = list.getBoundingClientRect();
      const targetRect = target.getBoundingClientRect();
      const targetOffset = list.clientHeight / 6;
      const nextTop = list.scrollTop + targetRect.top - listRect.top - targetOffset;
      const maxScroll = Math.max(0, list.scrollHeight - list.clientHeight);
      if (nextTop > maxScroll) list.style.paddingBottom = `${Math.ceil(nextTop - maxScroll)}px`;
      const reachableMax = Math.max(0, list.scrollHeight - list.clientHeight);
      diagnosisProgrammaticScrollUntil = performance.now() + 50;
      list.scrollTo({ top: Math.min(Math.max(0, nextTop), reachableMax), behavior: 'auto' });
      diagnosisQuestionAnchorActive = true;
    } finally {
      diagnosisQuestionAnchorPending = false;
    }
  }

  async function scrollDiagnosisAnswerToBottom() {
    await tick();
    if (typeof requestAnimationFrame === 'function') await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
    const list = diagnosisMessageListElement ?? document.querySelector<HTMLDivElement>('.diagnosis-message-list-f');
    if (!list || !diagnosisGenerating || !diagnosisAutoScrollEnabled) return;
    const streamingMessage = list.querySelector<HTMLElement>('.diagnosis-streaming-message');
    if (streamingMessage) {
      const listRect = list.getBoundingClientRect();
      const answerRect = streamingMessage.getBoundingClientRect();
      if (diagnosisQuestionAnchorActive && answerRect.bottom <= listRect.bottom + 1) return;
      diagnosisQuestionAnchorActive = false;
      if (list.style.paddingBottom) list.style.removeProperty('padding-bottom');
    }
    diagnosisProgrammaticScrollUntil = performance.now() + 50;
    list.scrollTo({ top: Math.max(0, list.scrollHeight - list.clientHeight), behavior: 'auto' });
  }

  function stopDiagnosisGeneration() {
    return diagnosisSessionController.stop();
  }

  function selectDiagnosisModelProvider(providerID: string) {
    const selection = providerModelSelection(diagnosisAvailableProviders, providerID, llmModelName);
    selectedProviderId = selection.providerId;
    llmModelName = selection.modelName;
  }

  function selectDiagnosisModel(modelName: string) {
    const provider = diagnosisAvailableProviders.find((item) => item.provider_id === selectedProviderId) ?? diagnosisAvailableProviders[0];
    if (!provider) return;
    selectedProviderId = provider.provider_id;
    llmModelName = modelName;
  }

  function beginDiagnosisEdit(message: DiagnosisMessage) {
    diagnosisEditingMessageId = message.id;
    diagnosisEditDraft = message.content;
  }

  function saveDiagnosisEdit() {
    return diagnosisCommands.saveDiagnosisEdit();
  }

  function toggleDiagnosisContext(resourceID: string) {
    if (selectedApplicationResourceIDs.has(resourceID)) return;
    diagnosisCommands.toggleDiagnosisContext(resourceID);
  }

  function toggleDiagnosisApplication(application: Application) {
    const selected = diagnosisApplicationIds.includes(application.id);
    const nextApplicationIDs = selected
      ? diagnosisApplicationIds.filter((id) => id !== application.id)
      : [...diagnosisApplicationIds, application.id];
    diagnosisApplicationIds = nextApplicationIDs;
    applicationId = nextApplicationIDs[0] ?? '';
    const associated = applicationResourceIDs(application);
    const remainingAssociated = new Set(
      diagnosisApplications
        .filter((item) => nextApplicationIDs.includes(item.id))
        .flatMap(applicationResourceIDs)
    );
    diagnosisTargetIds = selected
      ? diagnosisTargetIds.filter((id) => remainingAssociated.has(id) || !associated.includes(id))
      : [...new Set([...diagnosisTargetIds, ...associated])];
  }

  function applicationResourceIDs(application: Application) {
    return [...application.instances, ...application.dependencies]
      .map((item) => item.target_resource_id)
      .filter((id) => diagnosisTargets.some((resource) => resource.id === id));
  }

  function isDiagnosisContextExcluded(_resource: Resource) {
    return false;
  }

  function clearDiagnosisHistory() {
    return diagnosisCommands.clearDiagnosisHistory();
  }

  function newDiagnosisSession() {
    diagnosisCommands.newDiagnosisSession();
    diagnosisApplicationIds = [];
    applicationId = '';
  }

  function renameDiagnosisSession(session: DiagnosisSession) {
    diagnosisCommands.renameDiagnosisSession(session);
  }

  function deleteDiagnosisSession(session: DiagnosisSession) {
    return diagnosisCommands.deleteDiagnosisSession(session);
  }

  $: if (scopeId && (scopeId !== diagnosisLoadedScopeId || projectId !== diagnosisLoadedProjectId)) {
    diagnosisLoadedScopeId = scopeId;
    diagnosisLoadedProjectId = projectId;
    diagnosisSessionController.close();
    selectedDiagnosisId = '';
    diagnosisSnapshot = null;
    diagnosisSessions = [];
    diagnosisTargetIds = [];
    diagnosisApplicationIds = [];
    resetDiagnosisStreamState();
    void loadDiagnosis();
  }
  $: diagnosisTargets = contextResources.filter(
    (resource) => resourceInActiveWorkspace(resource) && !isDiagnosisContextExcluded(resource)
  );
  $: {
    const usableResourceIDs = new Set(diagnosisTargets.map((resource) => resource.id));
    diagnosisApplications = diagnosisApplicationCandidates.filter((application) =>
      [...application.instances, ...application.dependencies].some((item) => usableResourceIDs.has(item.target_resource_id))
    );
  }
  $: selectedApplicationResourceIDs = new Set(
    diagnosisApplications
      .filter((application) => diagnosisApplicationIds.includes(application.id))
      .flatMap(applicationResourceIDs)
  );
  $: if (!selectedDiagnosisId && !diagnosisSnapshot && applicationId) {
    const initialApplication = diagnosisApplications.find((item) => item.id === applicationId);
    if (initialApplication && !diagnosisApplicationIds.includes(initialApplication.id)) {
      diagnosisApplicationIds = [initialApplication.id];
      diagnosisTargetIds = applicationResourceIDs(initialApplication);
    }
  }
  $: if (!llmModelName && selectedProviderId) {
    const available = diagnosisAvailableProviders.find((item) => item.provider_id === selectedProviderId);
    llmModelName = String(available?.models[0]?.name ?? '');
  }
  $: selectedDiagnosisModel = diagnosisAvailableProviders
    .find((item) => item.provider_id === selectedProviderId)
    ?.models.find((item) => item.name === llmModelName);
  $: filteredComposerSkills = skills.filter((item) => {
    const query = diagnosisComposerMenuSearch.trim().toLowerCase();
    return !query || `${item.name} ${item.identifier}`.toLowerCase().includes(query);
  });
  $: filteredComposerPersonas = personas.filter((item) => {
    const query = diagnosisComposerMenuSearch.trim().toLowerCase();
    return !query || `${item.name} ${item.description}`.toLowerCase().includes(query);
  });
  function escapeComposerHTML(value: string) {
    return value.replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char] ?? char));
  }
  $: composerMarkup = escapeComposerHTML(diagnosisComposerText)
    .replace(/(^|\s)(\/[^\s]+)/g, '$1<span class="composer-skill-token">$2</span>')
    .replace(/(^|\s)(@[^\s]+)/g, '$1<span class="composer-persona-token">$2</span>')
    .replace(/\n/g, '<br>');
  $: diagnosisContextUsage = {
    used: Math.max(1, Math.ceil((
      diagnosisComposerText.length
      + (diagnosisSnapshot?.messages ?? []).reduce((sum, message) => sum + message.content.length, 0)
      + diagnosisTargetIds.reduce((sum, id) => {
        const resource = diagnosisTargets.find((item) => item.id === id);
        return sum + JSON.stringify(resource?.config ?? {}).length + 160;
      }, 0)
      + diagnosisApplicationIds.length * 120
      + (diagnosisSnapshot?.events ?? [])
        .filter((event) => /skill|persona|agent/i.test(event.type))
        .reduce((sum, event) => sum + JSON.stringify(event.payload ?? {}).length + 320, 0)
    ) / 4)),
    total: Number(selectedDiagnosisModel?.context_window_tokens ?? 128000)
  };
  $: {
    const list = diagnosisMessageListElement;
    if (list !== diagnosisScrollListenerElement) {
      diagnosisScrollListenerCleanup?.();
      diagnosisScrollListenerElement = list;
      if (list) {
        const onScroll = () => {
          if (performance.now() < diagnosisProgrammaticScrollUntil) return;
          const distanceFromBottom = list.scrollHeight - list.clientHeight - list.scrollTop;
          diagnosisAutoScrollEnabled = distanceFromBottom <= 8;
        };
        list.addEventListener('scroll', onScroll, { passive: true });
        diagnosisScrollListenerCleanup = () => list.removeEventListener('scroll', onScroll);
      }
    }
  }

  onDestroy(() => {
    diagnosisSessionController.close();
    diagnosisScrollListenerCleanup?.();
  });
  $: {
    const nextKey = `${diagnosisGenerating}:${diagnosisStreamingText.length}`;
    if (nextKey !== diagnosisAutoScrollKey) {
      diagnosisAutoScrollKey = nextKey;
      if (diagnosisGenerating && diagnosisAutoScrollEnabled && !diagnosisQuestionAnchorPending && diagnosisStreamingText) void scrollDiagnosisAnswerToBottom();
    }
  }

  let diagnosisModelMenuOpen = false;
  let diagnosisModelMenuProviderId = '';

  onMount(() => {
    return installDiagnosisDocumentListeners({
      isModelMenuOpen: () => diagnosisModelMenuOpen,
      closeModelMenu: () => (diagnosisModelMenuOpen = false),
      copyCode: copyDiagnosisCode
    });
  });

  function toggleDiagnosisModelMenu() {
    diagnosisModelMenuOpen = !diagnosisModelMenuOpen;
    if (diagnosisModelMenuOpen) {
      diagnosisModelMenuProviderId = selectedProviderId || diagnosisAvailableProviders[0]?.provider_id || '';
    }
  }

  function chooseDiagnosisModelProvider(providerID: string) {
    diagnosisModelMenuProviderId = providerID;
    selectDiagnosisModelProvider(providerID);
  }

  function chooseDiagnosisModel(modelName: string) {
    selectDiagnosisModel(modelName);
    diagnosisModelMenuOpen = false;
  }

  const diagnosisResourceName = (resourceID?: string) => getDiagnosisResourceName(resources, diagnosisTargets, resourceID);
  const isLastDiagnosisUser = (index: number) => getIsLastDiagnosisUser(diagnosisSnapshot, index);
  const diagnosisHasPersistedNewAnswer = (snapshot: DiagnosisSnapshot | null) => getDiagnosisHasPersistedNewAnswer(snapshot, diagnosisStreamingAssistantBaseline);
  const diagnosisLiveTimelineForDisplay = (snapshot: DiagnosisSnapshot | null) => diagnosisLiveTimeline(snapshot);
  const diagnosisHistoryTimelineForDisplay = (snapshot: DiagnosisSnapshot | null, assistantIndex: number) =>
    diagnosisAssistantTimeline(snapshot, assistantIndex);
  const diagnosisEvidenceSummary = (evidence: DiagnosisEvidence) => getDiagnosisEvidenceSummary(evidence);
  const diagnosisEvidenceSourceTools = (snapshot: DiagnosisSnapshot, evidence: DiagnosisEvidence) => getDiagnosisEvidenceSourceTools(snapshot, evidence);
  const diagnosisShouldShowEmptyAnswer = () => shouldShowEmptyDiagnosisAnswer(diagnosisSnapshot, diagnosisAnswerCompleted, diagnosisStreamingText, diagnosisGenerating);

  const diagnosisProcessText = (snapshot: DiagnosisSnapshot | null) => getDiagnosisProcessText(snapshot);

  function copyDiagnosisMessage(message: DiagnosisMessage, processExpanded: boolean) {
    copyDiagnosisText(
      diagnosisMessageClipboardText(message, processExpanded, diagnosisSnapshot),
      message.role === 'user' ? '问题已复制。' : '回答已复制。',
      onNotice
    );
  }

  function copyStreamingAnswer() {
    copyDiagnosisText(diagnosisStreamingClipboardText(diagnosisStreamingText, diagnosisSnapshot), '回答已复制。', onNotice);
  }

  function copyDiagnosisCode(encoded: string) {
    copyDiagnosisText(decodeDiagnosisCode(encoded), '代码已复制。', onNotice);
  }


  const diagnosisActiveCausalChain = activeDiagnosisCausalChain;

  function scrollToDiagnosisEvidence(id: string) {
    document.getElementById(`evidence-${id}`)?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  function startDiagnosisResize(side: 'history' | 'context', event: PointerEvent) {
    startDiagnosisPanelResize(
      side,
      event,
      () => ({ history: diagnosisHistoryWidth, context: diagnosisContextWidth }),
      (resizingSide, width) => {
        if (resizingSide === 'history') diagnosisHistoryWidth = width;
        else diagnosisContextWidth = width;
      }
    );
  }

  $: diagnosisMenuProvider =
    diagnosisAvailableProviders.find(
      (item) =>
        item.provider_id ===
        (diagnosisModelMenuProviderId || selectedProviderId)
    ) ??
    diagnosisAvailableProviders.find(
      (item) => item.provider_id === selectedProviderId
    ) ??
    diagnosisAvailableProviders[0];
</script>

<section
  class="diagnosis-workbench-f"
  class:history-collapsed={diagnosisHistoryCollapsed}
  class:context-collapsed={diagnosisContextCollapsed}
  style={`--diagnosis-history-width:${diagnosisHistoryWidth}px;--diagnosis-context-width:${diagnosisContextWidth}px`}
>
  {#if !diagnosisHistoryCollapsed}
    <aside class="diagnosis-history-panel">
      <div class="diagnosis-panel-top">
        <div>
          <h2>会话历史</h2>
          <small>{diagnosisSessions.length} 个会话</small>
        </div>
        <div class="diagnosis-heading-actions">
          <button
            class="icon-button delete-action"
            aria-label="清空会话历史"
            data-tooltip="清空会话历史"
            on:click={clearDiagnosisHistory}><Trash2 size={15} /></button
          ><button
            class="icon-button"
            aria-label="新建诊断会话"
            data-tooltip="新建诊断会话"
            on:click={newDiagnosisSession}><Plus size={16} /></button
          >
        </div>
      </div>
      <SearchInput className="diagnosis-session-search" bind:value={diagnosisSessionSearch} width="100%" height="31px" placeholder="搜索会话" ariaLabel="搜索会话" />
      <div class="diagnosis-session-list-f">
        {#each diagnosisSessions.filter((session) => !diagnosisSessionSearch.trim() || (session.title || '')
              .toLowerCase()
              .includes(diagnosisSessionSearch.toLowerCase())) as session}<div
            class="diagnosis-session-item"
          >
            <button
              class:active={selectedDiagnosisId === session.id}
              class="diagnosis-session-row-f"
              on:click={() => void openDiagnosis(session.id)}
              ><strong class="diagnosis-session-title-f"
                >{session.title || '未命名诊断'}</strong
              ><span class="diagnosis-session-meta-f"
                ><small>{formatDate(session.created_at)}</small><em
                  class={`diagnosis-session-status-f ${session.status}`}
                  >{diagnosisStatusLabel(session.status)}</em
                ></span
              ></button
            >
            <div class="diagnosis-session-actions">
              <button
                class="edit-action"
                aria-label="重命名会话"
                data-tooltip="重命名会话"
                on:click|stopPropagation={() => renameDiagnosisSession(session)}
                ><Pencil size={13} /></button
              ><button
                class="delete-action"
                aria-label="删除会话"
                data-tooltip="删除会话"
                on:click|stopPropagation={() => deleteDiagnosisSession(session)}
                ><Trash2 size={13} /></button
              >
            </div>
          </div>{:else}<p class="diagnosis-empty">还没有诊断会话。</p>{/each}
      </div>
    </aside>
  {/if}
  <div
    class="diagnosis-splitter left"
    role="separator"
    aria-orientation="vertical"
    on:pointerdown={(event) => startDiagnosisResize('history', event)}
  >
    <button
      aria-label={diagnosisHistoryCollapsed ? '展开会话历史' : '折叠会话历史'}
      on:click={() => (diagnosisHistoryCollapsed = !diagnosisHistoryCollapsed)}
      >{#if diagnosisHistoryCollapsed}<ChevronRight
          size={16}
        />{:else}<ChevronLeft size={16} />{/if}</button
    >
  </div>
  <section class="diagnosis-conversation-f">
    <header class="diagnosis-conversation-head">
      <div class="diagnosis-conversation-title">
        <h1>{diagnosisSnapshot?.session.title || '新建诊断会话'}</h1>
        <small>{scopeLabel} · 只读证据链</small>
      </div>
      <div class="diagnosis-loaded-context">
        <span class="diagnosis-loaded-context-label">已加载上下文</span>
        <div class="diagnosis-context-resources">
          {#each diagnosisTargets
            .filter((resource) => diagnosisTargetIds.includes(resource.id))
            .slice(0, 3) as resource}<span class="diagnosis-context-chip"
              >{resource.name}</span
            >{/each}{#if diagnosisTargetIds.length > 3}<span
              class="diagnosis-context-chip accent"
              >+{diagnosisTargetIds.length - 3}</span
            >{/if}{#if diagnosisTargetIds.length === 0}<span
              class="diagnosis-context-chip muted">未选择</span
            >{/if}
        </div>
      </div>
      <div class="diagnosis-head-actions">
        <span class="diagnosis-head-status"
          ><i class:running={diagnosisGenerating}></i>{diagnosisGenerating
            ? '正在生成回答'
            : diagnosisSnapshot
              ? diagnosisStatusLabel(diagnosisSnapshot.session.status)
              : '等待提问'}</span
        ><button
          class="icon-button"
          type="button"
          aria-label="新建诊断会话"
          data-tooltip="新建诊断会话"
          on:click={newDiagnosisSession}><MessageSquarePlus size={16} /></button
        >
      </div>
    </header>
    <DiagnosisConversationMessages
      bind:diagnosisMessageListElement
      {diagnosisSnapshot}
      {busy}
      bind:diagnosisEditingMessageId
      bind:diagnosisEditDraft
      bind:diagnosisProcessExpanded
      bind:diagnosisActionExpanded
      bind:diagnosisLiveProcessExpanded
      {diagnosisGenerating}
      {diagnosisAnswerCompleted}
      {diagnosisStreamingText}
      {diagnosisInterruptedReason}
      {diagnosisStreamingStartedAt}
      {formatDate}
      renderMarkdown={renderDiagnosisMarkdown}
      diagnosisLiveTimeline={diagnosisLiveTimelineForDisplay}
      diagnosisHistoryTimeline={diagnosisHistoryTimelineForDisplay}
      {diagnosisStatusLabel}
      {diagnosisHasRunningActions}
      {diagnosisHasPersistedNewAnswer}
      diagnosisShouldShowEmptyAnswer={diagnosisShouldShowEmptyAnswer}
      {diagnosisActionLabel}
      {isLastDiagnosisUser}
      {beginDiagnosisEdit}
      {saveDiagnosisEdit}
      {copyDiagnosisMessage}
      {copyStreamingAnswer}
    />
    <form
      class="diagnosis-composer-f"
      on:submit|preventDefault={() => void submitDiagnosisMessage()}
    >
      <div class="diagnosis-composer-shell">
        <div class="diagnosis-composer-highlight" aria-hidden="true">{@html composerMarkup}</div>
        <textarea
          bind:this={diagnosisComposerTextarea}
          value={diagnosisComposerText}
          on:input={(event) => updateDiagnosisComposer((event.currentTarget as HTMLTextAreaElement).value, event.currentTarget as HTMLTextAreaElement)}
          on:keydown={handleDiagnosisComposerKeydown}
          placeholder="描述问题，Enter 发送 · Shift + Enter 换行"
          aria-label="输入诊断问题"
          rows="3"
          maxlength="16000"
        ></textarea>
        {#if diagnosisComposerMenu}<div class="diagnosis-composer-token-menu" role="listbox">
          <SearchInput bind:value={diagnosisComposerMenuSearch} width="100%" height="28px" placeholder={diagnosisComposerMenu === 'skill' ? '搜索技能' : '搜索专家'} ariaLabel={diagnosisComposerMenu === 'skill' ? '搜索技能' : '搜索专家'} />
          <div class="diagnosis-composer-token-list">
            {#if diagnosisComposerMenu === 'skill'}
              {#each filteredComposerSkills as item}<button type="button" role="option" aria-selected={diagnosisComposerSkillID === item.id} on:click={() => chooseComposerItem('skill', item)}><strong>/{item.identifier}</strong><span>{diagnosisComposerSkillID === item.id ? '已选择' : item.name}</span></button>{:else}<span>暂无可用技能</span>{/each}
            {:else}
              {#each filteredComposerPersonas as item}<button type="button" role="option" aria-selected={diagnosisComposerPersonaID === item.id} on:click={() => chooseComposerItem('persona', item)}><strong>@{item.name}</strong><span>{diagnosisComposerPersonaID === item.id ? '已选择' : item.description || '专家'}</span></button>{:else}<span>暂无可用专家</span>{/each}
            {/if}
          </div>
        </div>{/if}
        <div class="diagnosis-composer-tools">
          <div>
            <input bind:this={diagnosisComposerFileInput} class="diagnosis-hidden-file-input" type="file" multiple accept=".txt,.log,.json,.yaml,.yml,.md,.csv,.xml,.sql,.toml,.ini,.conf" on:change={addDiagnosisAttachment} />
            <button
              type="button"
              class="diagnosis-tool"
              data-tooltip="添加附件"
              on:click={() => diagnosisComposerFileInput?.click()}
              ><Paperclip size={15} />附件</button
            ><button
              type="button"
              class="diagnosis-tool"
              data-tooltip="添加链接"
              on:click={addDiagnosisLink}
              ><Link2 size={15} />链接</button
            ><button
              type="button"
              class="diagnosis-tool"
              data-tooltip="选择技能"
              on:click={() => { diagnosisComposerMenu = diagnosisComposerMenu === 'skill' ? '' : 'skill'; diagnosisComposerMenuSearch = ''; diagnosisComposerTokenStart = diagnosisComposerTextarea?.selectionStart ?? diagnosisComposerText.length; }}
              ><Sparkles size={15} />技能</button
            ><button
              type="button"
              class="diagnosis-tool"
              data-tooltip="选择专家"
              on:click={() => { diagnosisComposerMenu = diagnosisComposerMenu === 'persona' ? '' : 'persona'; diagnosisComposerMenuSearch = ''; diagnosisComposerTokenStart = diagnosisComposerTextarea?.selectionStart ?? diagnosisComposerText.length; }}
              ><Bot size={15} />专家</button
            >
          </div>
          {#if diagnosisComposerAttachments.length || diagnosisComposerLinks.length}<div class="diagnosis-composer-attached-items">
            {#each diagnosisComposerAttachments as attachment, index}<span class="diagnosis-composer-attachment">{attachment.name}<button type="button" aria-label={`移除附件 ${attachment.name}`} on:click={() => diagnosisComposerAttachments = diagnosisComposerAttachments.filter((_, itemIndex) => itemIndex !== index)}>×</button></span>{/each}
            {#each diagnosisComposerLinks as link, index}<span class="diagnosis-composer-attachment diagnosis-composer-link">{link}<button type="button" aria-label="移除链接" on:click={() => diagnosisComposerLinks = diagnosisComposerLinks.filter((_, itemIndex) => itemIndex !== index)}>×</button></span>{/each}
          </div>{/if}
          <div class="diagnosis-model-picker">
            <button
              class="diagnosis-model-trigger"
              type="button"
              aria-label="选择渠道和模型"
              aria-haspopup="menu"
              aria-expanded={diagnosisModelMenuOpen}
              data-tooltip={modelCapabilityTooltip()}
              disabled={diagnosisAvailableProviders.length === 0}
              on:click={toggleDiagnosisModelMenu}
              ><span
                >{#if diagnosisAvailableProviders.find((item) => item.provider_id === selectedProviderId)}{diagnosisAvailableProviders.find(
                    (item) => item.provider_id === selectedProviderId
                  )?.name}{:else}暂无可用渠道{/if}{#if llmModelName}
                  · {llmModelName}{/if}</span
              ><ChevronDown size={13} aria-hidden="true" /></button
            >{#if diagnosisModelMenuOpen}<div
                class="diagnosis-model-menu"
                role="menu"
                aria-label="渠道和模型"
              >
                <div class="diagnosis-model-provider-list">
                  <small class="diagnosis-model-menu-heading">渠道</small
                  >{#each diagnosisAvailableProviders as provider}<button
                      type="button"
                      role="menuitem"
                      class:active={provider.provider_id ===
                        (diagnosisModelMenuProviderId || selectedProviderId)}
                      aria-haspopup="menu"
                      aria-expanded={provider.provider_id ===
                        (diagnosisModelMenuProviderId || selectedProviderId)}
                      on:click={() =>
                        chooseDiagnosisModelProvider(
                          provider.provider_id
                        )}
                ><span>{provider.name}</span><ChevronRight
                        size={12}
                        aria-hidden="true"
                      /></button
                    >{/each}
                </div>
                <div
                  class="diagnosis-model-option-list"
                  role="menu"
                  aria-label="模型"
                >
                  <small class="diagnosis-model-menu-heading">模型</small
                  >{#if diagnosisMenuProvider}{#each diagnosisMenuProvider.models as model}{@const name =
                        String(model.name ?? '')}<button
                        type="button"
                        role="menuitemradio"
                        aria-checked={selectedProviderId ===
                          diagnosisMenuProvider.provider_id &&
                          llmModelName === name}
                        class:active={selectedProviderId ===
                          diagnosisMenuProvider.provider_id &&
                          llmModelName === name}
                        on:click={() => chooseDiagnosisModel(name)}
                        ><span>{name}</span></button
                      >{/each}{:else}<span class="diagnosis-model-empty"
                      >暂无可用模型</span
                    >{/if}
                </div>
              </div>{/if}<button
              class="primary diagnosis-send-button"
              type="button"
              disabled={busy ||
                (!diagnosisComposerText.trim() && !diagnosisComposerAttachments.length && !diagnosisComposerLinks.length && !diagnosisGenerating)}
              on:click={() =>
                diagnosisGenerating
                  ? stopDiagnosisGeneration()
                  : void submitDiagnosisMessage()}
              >{#if diagnosisGenerating}<Square
                  size={14}
                  fill="currentColor"
                />停止{:else}<Send size={14} />发送{/if}</button
            >
          </div>
        </div>
      </div>
    </form>
  </section>
  <div
    class="diagnosis-splitter right"
    role="separator"
    aria-orientation="vertical"
    on:pointerdown={(event) => startDiagnosisResize('context', event)}
  >
    <button
      aria-label={diagnosisContextCollapsed
        ? '展开诊断上下文'
        : '折叠诊断上下文'}
      on:click={() => (diagnosisContextCollapsed = !diagnosisContextCollapsed)}
      >{#if diagnosisContextCollapsed}<ChevronLeft
          size={16}
        />{:else}<ChevronRight size={16} />{/if}</button
    >
  </div>
  {#if !diagnosisContextCollapsed}<DiagnosisContextPanel
      {diagnosisSnapshot}
      {diagnosisTargets}
      {diagnosisApplications}
      {diagnosisApplicationIds}
      {diagnosisTargetIds}
      bind:diagnosisContextTab
      {toggleDiagnosisContext}
      {toggleDiagnosisApplication}
      contextUsage={diagnosisContextUsage}
      {resourceIcon}
      {resourceSchemaName}
      {scopeName}
      {diagnosisActiveCausalChain}
      {diagnosisCausalNodes}
      {diagnosisCausalEvidenceIDs}
      {scrollToDiagnosisEvidence}
      {diagnosisEvidenceTimeline}
      {diagnosisEvidenceSourceTools}
      {diagnosisEvidenceSummary}
      {diagnosisResourceName}
      {formatDate}
    />{/if}
</section>
