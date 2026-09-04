<script lang="ts">
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
    DiagnosisMessage,
    DiagnosisSession,
    DiagnosisSnapshot,
    Resource
  } from '../../lib/api';
  import DiagnosisContextPanel from './DiagnosisContextPanel.svelte';
  import DiagnosisConversationMessages from './DiagnosisConversationMessages.svelte';

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

  export let diagnosisHistoryCollapsed = false;
  export let diagnosisContextCollapsed = false;
  export let diagnosisHistoryWidth = 280;
  export let diagnosisContextWidth = 320;
  export let diagnosisSessions: DiagnosisSession[] = [];
  export let selectedDiagnosisId = '';
  export let diagnosisSessionSearch = '';
  export let diagnosisStatusLabel: (status: string) => string;
  export let formatDate: (value: string) => string;
  export let clearDiagnosisHistory: () => void;
  export let newDiagnosisSession: () => void;
  export let openDiagnosis: (sessionID: string) => Promise<unknown>;
  export let renameDiagnosisSession: (session: DiagnosisSession) => void;
  export let deleteDiagnosisSession: (session: DiagnosisSession) => void;
  export let startDiagnosisResize: (
    panel: 'history' | 'context',
    event: PointerEvent
  ) => void;
  export let diagnosisSnapshot: DiagnosisSnapshot | null = null;
  export let scopeLabel = '当前级别';
  export let diagnosisTargets: Resource[] = [];
  export let diagnosisTargetIds: string[] = [];
  export let diagnosisGenerating = false;
  export let busy = false;
  export let diagnosisMessageListElement: HTMLDivElement | null = null;
  export let diagnosisAnswerCompleted = false;
  export let diagnosisStreamingText = '';
  export let diagnosisInterruptedReason = '';
  export let diagnosisEditingMessageId = '';
  export let diagnosisEditDraft = '';
  export let diagnosisProcessExpanded: Record<string, boolean> = {};
  export let diagnosisActionExpanded: Record<string, boolean> = {};
  export let diagnosisLiveProcessExpanded = false;
  export let diagnosisStreamingStartedAt = 0;
  export let renderMarkdown: (text: string) => string;
  export let diagnosisLiveTimeline: (
    snapshot: DiagnosisSnapshot | null
  ) => LiveTimelineItem[];
  export let diagnosisProcessDuration: (
    snapshot: DiagnosisSnapshot | null
  ) => string;
  export let diagnosisProcessActionCount: (
    snapshot: DiagnosisSnapshot | null
  ) => number;
  export let diagnosisHasRunningActions: (
    snapshot: DiagnosisSnapshot | null
  ) => boolean;
  export let diagnosisHasPersistedNewAnswer: (
    snapshot: DiagnosisSnapshot | null
  ) => boolean;
  export let diagnosisActionLabel: (item: LiveTimelineItem) => string;
  export let deferFinalDiagnosisMessage: (
    message: DiagnosisMessage,
    index: number
  ) => boolean;
  export let isLastDiagnosisUser: (index: number) => boolean;
  export let beginDiagnosisEdit: (message: DiagnosisMessage) => void;
  export let saveDiagnosisEdit: () => void | Promise<void>;
  export let copyDiagnosisMessage: (
    message: DiagnosisMessage,
    processExpanded: boolean
  ) => void;
  export let copyStreamingAnswer: () => void;
  export let diagnosisComposerText = '';
  export let diagnosisAvailableProviders: Provider[] = [];
  export let selectedProviderId = '';
  export let llmModelName = '';
  export let diagnosisModelMenuOpen = false;
  export let diagnosisModelMenuProviderId = '';
  export let handleDiagnosisComposerKeydown: (event: KeyboardEvent) => void;
  export let submitDiagnosisMessage: () => Promise<unknown>;
  export let stopDiagnosisGeneration: () => void;
  export let onNotice: (message: string) => void;
  export let toggleDiagnosisModelMenu: () => void;
  export let chooseDiagnosisModelProvider: (providerID: string) => void;
  export let chooseDiagnosisModel: (modelName: string) => void;
  export let diagnosisContextTab: 'context' | 'evidence' = 'context';
  export let toggleDiagnosisContext: (resourceID: string) => void;
  export let resourceIcon: (kind: string) => string;
  export let resourceSchemaName: (kind: string) => string;
  export let scopeName: (id: string) => string;
  export let diagnosisActiveCausalChain: (
    snapshot: DiagnosisSnapshot | null
  ) => any;
  export let diagnosisCausalNodes: (chain: any) => any[];
  export let diagnosisCausalEvidenceIDs: (
    chain: any,
    nodeID: string
  ) => string[];
  export let scrollToDiagnosisEvidence: (id: string) => void;
  export let diagnosisEvidenceTimeline: (
    snapshot: DiagnosisSnapshot | null
  ) => any[];
  export let diagnosisEvidenceSourceTools: (
    snapshot: DiagnosisSnapshot,
    evidence: any
  ) => string[];
  export let diagnosisEvidenceSummary: (evidence: any) => string;
  export let diagnosisResourceName: (resourceID?: string) => string;

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
      {renderMarkdown}
      {diagnosisLiveTimeline}
      {diagnosisProcessDuration}
      {diagnosisProcessActionCount}
      {diagnosisStatusLabel}
      {diagnosisHasRunningActions}
      {diagnosisHasPersistedNewAnswer}
      {diagnosisActionLabel}
      {deferFinalDiagnosisMessage}
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
