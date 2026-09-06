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
    AIProviderAvailability,
    DiagnosisEvidence,
    DiagnosisMessage,
    DiagnosisSession,
    DiagnosisSnapshot,
    Resource
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
  type Provider = AIProviderAvailability;

  let diagnosisHistoryCollapsed = false;
  let diagnosisContextCollapsed = false;
  let diagnosisHistoryWidth = 232;
  let diagnosisContextWidth = 275;
  let diagnosisSessions: DiagnosisSession[] = [];
  let selectedDiagnosisId = '';
  let diagnosisSessionSearch = '';
  export let formatDate: (value: string) => string;
  export let scopeId = '';
  export let runAction: (operation: () => Promise<void>) => Promise<void>;
  export let describeError: (error: unknown, fallback: string) => string;
  export let onError: (message: string) => void;
  let diagnosisSnapshot: DiagnosisSnapshot | null = null;
  export let scopeLabel = '当前级别';
  export let resources: Resource[] = [];
  export let contextResources: Resource[] = [];
  export let resourceInActiveWorkspace: (resource: Resource) => boolean;
  let diagnosisTargetIds: string[] = [];
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
  let diagnosisAvailableProviders: Provider[] = [];
  let selectedProviderId = '';
  let llmModelName = '';
  export let onNotice: (message: string) => void;

  let diagnosisContextTab: 'context' | 'evidence' = 'context';
  export let resourceIcon: (kind: string) => string;
  export let resourceSchemaName: (kind: string) => string;
  export let scopeName: (id: string) => string;

  let diagnosisLoadedScopeId = '';
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
      sessions: diagnosisSessions,
      selectedSessionID: selectedDiagnosisId,
      snapshot: diagnosisSnapshot,
      composerText: diagnosisComposerText,
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
    onError
  });

  async function loadDiagnosis() {
    if (!scopeId) return;
    try {
      diagnosisAvailableProviders = await api.availableAIProviders(scopeId, 'diagnosis');
      if (
        diagnosisAvailableProviders.length > 0 &&
        (!selectedProviderId ||
          !diagnosisAvailableProviders.some((item) => item.provider_resource_id === selectedProviderId))
      ) {
        selectedProviderId = diagnosisAvailableProviders[0].provider_resource_id;
        llmModelName = diagnosisAvailableProviders[0].models[0]?.name ?? '';
      }
    } catch (error) {
      diagnosisAvailableProviders = [];
      onError(describeError(error, '可用模型服务商加载失败'));
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
      if (diagnosisSnapshot.session.ai_provider_resource_id) selectedProviderId = diagnosisSnapshot.session.ai_provider_resource_id;
      if (diagnosisSnapshot.session.model_name) llmModelName = diagnosisSnapshot.session.model_name;
      diagnosisGenerating = isDiagnosisRunning(diagnosisSnapshot.session.status);
      diagnosisSessionController.syncCursor(diagnosisSnapshot);
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

  function handleDiagnosisComposerKeydown(event: KeyboardEvent) {
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
    const provider = diagnosisAvailableProviders.find((item) => item.provider_resource_id === selectedProviderId) ?? diagnosisAvailableProviders[0];
    if (!provider) return;
    selectedProviderId = provider.provider_resource_id;
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
    diagnosisCommands.toggleDiagnosisContext(resourceID);
  }

  function clearDiagnosisHistory() {
    return diagnosisCommands.clearDiagnosisHistory();
  }

  function newDiagnosisSession() {
    diagnosisCommands.newDiagnosisSession();
  }

  function renameDiagnosisSession(session: DiagnosisSession) {
    diagnosisCommands.renameDiagnosisSession(session);
  }

  function deleteDiagnosisSession(session: DiagnosisSession) {
    return diagnosisCommands.deleteDiagnosisSession(session);
  }

  $: if (scopeId && scopeId !== diagnosisLoadedScopeId) {
    diagnosisLoadedScopeId = scopeId;
    diagnosisSessionController.close();
    selectedDiagnosisId = '';
    diagnosisSnapshot = null;
    diagnosisSessions = [];
    diagnosisTargetIds = [];
    resetDiagnosisStreamState();
    void loadDiagnosis();
  }
  $: diagnosisTargets = contextResources.filter(
    (resource) => resource.status === 'active' && resourceInActiveWorkspace(resource)
  );
  $: if (!llmModelName && selectedProviderId) {
    const available = diagnosisAvailableProviders.find((item) => item.provider_resource_id === selectedProviderId);
    llmModelName = String(available?.models[0]?.name ?? '');
  }
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
      diagnosisModelMenuProviderId = selectedProviderId || diagnosisAvailableProviders[0]?.provider_resource_id || '';
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
        item.provider_resource_id ===
        (diagnosisModelMenuProviderId || selectedProviderId)
    ) ??
    diagnosisAvailableProviders.find(
      (item) => item.provider_resource_id === selectedProviderId
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
            class="icon-button"
            aria-label="清空会话历史"
            title="清空会话历史"
            on:click={clearDiagnosisHistory}><Trash2 size={15} /></button
          ><button
            class="icon-button"
            aria-label="新建诊断会话"
            title="新建诊断会话"
            on:click={newDiagnosisSession}><Plus size={16} /></button
          >
        </div>
      </div>
      <input
        class="diagnosis-session-search"
        bind:value={diagnosisSessionSearch}
        placeholder="搜索会话"
        aria-label="搜索会话"
      />
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
                aria-label="重命名会话"
                title="重命名会话"
                on:click|stopPropagation={() => renameDiagnosisSession(session)}
                ><Pencil size={13} /></button
              ><button
                aria-label="删除会话"
                title="删除会话"
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
          title="新建诊断会话"
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
        <textarea
          value={diagnosisComposerText}
          on:input={(event) =>
            (diagnosisComposerText = (
              event.currentTarget as HTMLTextAreaElement
            ).value)}
          on:keydown={handleDiagnosisComposerKeydown}
          placeholder="描述问题，或输入 / 调用 Skill…"
          aria-label="输入诊断问题"
          rows="3"
          maxlength="16000"
        ></textarea>
        <div class="diagnosis-composer-tools">
          <div>
            <button
              type="button"
              class="diagnosis-tool"
              title="添加附件"
              on:click={() => onNotice('附件入口已打开。')}
              ><Paperclip size={15} />附件</button
            ><button
              type="button"
              class="diagnosis-tool"
              title="添加链接"
              on:click={() => onNotice('链接入口已打开。')}
              ><Link2 size={15} />链接</button
            ><button
              type="button"
              class="diagnosis-tool"
              title="选择 Skills"
              on:click={() =>
                onNotice('Skills：指标查询、日志查询、Kubernetes 只读查询。')}
              ><Sparkles size={15} />Skills</button
            ><button
              type="button"
              class="diagnosis-tool"
              title="选择 Agent"
              on:click={() => onNotice('Agent：故障定位 Agent。')}
              ><Bot size={15} />Agent</button
            >
          </div>
          <div class="diagnosis-model-picker">
            <button
              class="diagnosis-model-trigger"
              type="button"
              aria-label="选择模型服务商和模型"
              aria-haspopup="menu"
              aria-expanded={diagnosisModelMenuOpen}
              disabled={diagnosisAvailableProviders.length === 0}
              on:click={toggleDiagnosisModelMenu}
              ><span
                >{#if diagnosisAvailableProviders.find((item) => item.provider_resource_id === selectedProviderId)}{diagnosisAvailableProviders.find(
                    (item) => item.provider_resource_id === selectedProviderId
                  )?.name}{:else}暂无可用模型服务商{/if}{#if llmModelName}
                  · {llmModelName}{/if}</span
              ><ChevronDown size={13} aria-hidden="true" /></button
            >{#if diagnosisModelMenuOpen}<div
                class="diagnosis-model-menu"
                role="menu"
                aria-label="模型服务商和模型"
              >
                <div class="diagnosis-model-provider-list">
                  <small class="diagnosis-model-menu-heading">模型服务商</small
                  >{#each diagnosisAvailableProviders as provider}<button
                      type="button"
                      role="menuitem"
                      class:active={provider.provider_resource_id ===
                        (diagnosisModelMenuProviderId || selectedProviderId)}
                      aria-haspopup="menu"
                      aria-expanded={provider.provider_resource_id ===
                        (diagnosisModelMenuProviderId || selectedProviderId)}
                      on:click={() =>
                        chooseDiagnosisModelProvider(
                          provider.provider_resource_id
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
                          diagnosisMenuProvider.provider_resource_id &&
                          llmModelName === name}
                        class:active={selectedProviderId ===
                          diagnosisMenuProvider.provider_resource_id &&
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
                (!diagnosisComposerText.trim() && !diagnosisGenerating)}
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
        <small class="diagnosis-composer-note"
          ><span>Enter 发送 · Shift + Enter 换行</span><span
            >当前模型支持：文本、工具调用、流式输出</span
          ></small
        >
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
      {diagnosisTargetIds}
      bind:diagnosisContextTab
      {toggleDiagnosisContext}
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
