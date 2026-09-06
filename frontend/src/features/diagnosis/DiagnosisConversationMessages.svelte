<script lang="ts">
  import { Bot, Check, Copy, Pencil, Stethoscope, Wrench, X } from 'lucide-svelte';
  import type { DiagnosisMessage, DiagnosisSnapshot } from '../../lib/api';
  import { diagnosisDurationMilliseconds } from './diagnosisUtils';

  type DiagnosisLiveTimelineItem = {
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

  export let diagnosisSnapshot: DiagnosisSnapshot | null = null;
  export let diagnosisMessageListElement: HTMLDivElement | null = null;
  export let busy = false;
  export let diagnosisGenerating = false;
  export let diagnosisAnswerCompleted = false;
  export let diagnosisStreamingText = '';
  export let diagnosisInterruptedReason = '';
  export let diagnosisEditingMessageId = '';
  export let diagnosisEditDraft = '';
  export let diagnosisProcessExpanded: Record<string, boolean> = {};
  export let diagnosisActionExpanded: Record<string, boolean> = {};
  export let diagnosisLiveProcessExpanded = false;
  export let diagnosisStreamingStartedAt = 0;
  export let formatDate: (value: string) => string;
  export let renderMarkdown: (text: string) => string;
  export let diagnosisLiveTimeline: (snapshot: DiagnosisSnapshot | null) => DiagnosisLiveTimelineItem[];
  export let diagnosisHistoryTimeline: (snapshot: DiagnosisSnapshot | null, assistantIndex: number) => DiagnosisLiveTimelineItem[] = (snapshot) => diagnosisLiveTimeline(snapshot);
  export let diagnosisStatusLabel: (status: string) => string;
  export let diagnosisHasRunningActions: (snapshot: DiagnosisSnapshot | null) => boolean;
  export let diagnosisHasPersistedNewAnswer: (snapshot: DiagnosisSnapshot | null) => boolean;
  export let diagnosisShouldShowEmptyAnswer: () => boolean = () => false;
  export let diagnosisActionLabel: (item: DiagnosisLiveTimelineItem) => string;
  export let isLastDiagnosisUser: (index: number) => boolean;
  export let beginDiagnosisEdit: (message: DiagnosisMessage) => void;
  export let saveDiagnosisEdit: () => void | Promise<void>;
  export let copyDiagnosisMessage: (message: DiagnosisMessage, processExpanded: boolean) => void;
  export let copyStreamingAnswer: () => void;

  let liveTimeline: DiagnosisLiveTimelineItem[] = [];
  let editingMessageSize: { width: number; height: number } | null = null;
  type DisplayMessage = {
    message: DiagnosisMessage;
    streaming: boolean;
    assistantIndex: number;
  };
  let displayMessages: DisplayMessage[] = [];
  // Keep the derived event view stable within one render pass. The reducer is
  // pure, but rebuilding the same timeline in every branch made streaming
  // updates needlessly expensive and obscured which view was being rendered.
  $: liveTimeline = diagnosisLiveTimeline(diagnosisSnapshot);
  $: {
    const messages: DisplayMessage[] = diagnosisSnapshot
      ? diagnosisSnapshot.messages.map((message, index) => ({
          message,
          streaming: false,
          assistantIndex: diagnosisSnapshot.messages
            .slice(0, index)
            .filter((item) => item.role === 'assistant').length
        }))
      : [];
    if (
      diagnosisSnapshot &&
      !diagnosisHasPersistedNewAnswer(diagnosisSnapshot) &&
      (diagnosisStreamingText.trim() || diagnosisAnswerCompleted || diagnosisGenerating || diagnosisInterruptedReason || diagnosisHasRunningActions(diagnosisSnapshot))
    ) {
      messages.push({
        message: {
          id: '__diagnosis_streaming__',
          session_id: diagnosisSnapshot.session.id,
          role: 'assistant',
          content: diagnosisStreamingText,
          created_at: diagnosisStreamingStartedAt
            ? new Date(diagnosisStreamingStartedAt).toISOString()
            : new Date().toISOString()
        },
        streaming: true,
        assistantIndex: diagnosisSnapshot.messages.filter((item) => item.role === 'assistant').length
      });
    }
    displayMessages = messages;
  }

  $: if (!diagnosisEditingMessageId) editingMessageSize = null;

  function beginMessageEdit(message: DiagnosisMessage, event: MouseEvent) {
    const article = (event.currentTarget as HTMLElement).closest<HTMLElement>('.diagnosis-message-f');
    const bubble = article?.querySelector<HTMLElement>('.diagnosis-bubble-f');
    const content = article?.querySelector<HTMLElement>('.diagnosis-message-content');
    if (bubble && content) {
      const bubbleRect = bubble.getBoundingClientRect();
      const contentWidth = content.getBoundingClientRect().width;
      const extraEditingSpace = Math.max(96, Math.round(contentWidth * 0.2));
      editingMessageSize = {
        width: Math.min(contentWidth, Math.max(contentWidth * 0.25, bubbleRect.width + extraEditingSpace)),
        height: Math.max(34, bubbleRect.height)
      };
    }
    beginDiagnosisEdit(message);
  }

  function cancelMessageEdit() {
    editingMessageSize = null;
    diagnosisEditingMessageId = '';
  }

  async function saveMessageEdit() {
    await saveDiagnosisEdit();
  }

  const processActionCount = (timeline: DiagnosisLiveTimelineItem[]) =>
    timeline.reduce((count, item) => count + (item.kind === 'action' ? (item.actions?.length ?? 1) : 0), 0);

  const processDuration = (timeline: DiagnosisLiveTimelineItem[]) => {
    const first = timeline.find((item) => item.createdAt)?.createdAt;
    const last = [...timeline].reverse().find((item) => item.updatedAt)?.updatedAt;
    if (!first || !last) return '—';
    const milliseconds = Math.max(0, new Date(last).getTime() - new Date(first).getTime());
    return diagnosisDurationMilliseconds(milliseconds);
  };

  const toggleExpanded = (key: string) => {
    diagnosisActionExpanded = {
      ...diagnosisActionExpanded,
      [key]: !diagnosisActionExpanded[key]
    };
  };
</script>

<div class="diagnosis-message-list-f" bind:this={diagnosisMessageListElement}>
  {#if !diagnosisSnapshot && !displayMessages.length}
    <div class="diagnosis-welcome">
      <span class="diagnosis-welcome-icon"><Stethoscope size={23} /></span>
      <h2>从一个问题开始</h2>
      <p>
        可直接提问；如需查询资源状态、内容或性能，请在右侧选择已授权的上下文资源。
        AIEngine 会按需调用受控只读工具。
      </p>
    </div>
  {/if}
  {#if displayMessages.length}
    {#each displayMessages as item, index}
      {@const message = item.message}
      {@const isStreaming = item.streaming}
      {@const processTimeline = message.role === 'assistant'
        ? isStreaming ? liveTimeline : diagnosisHistoryTimeline(diagnosisSnapshot, item.assistantIndex)
        : []}
      <article
        class="diagnosis-message-f {message.role} {isStreaming ? 'diagnosis-streaming-message' : ''}"
        data-diagnosis-message-id={isStreaming ? undefined : message.id}
        data-diagnosis-message-content={!isStreaming && message.role === 'user' ? message.content : undefined}
      >
          <span class="diagnosis-message-avatar">{#if message.role === 'assistant'}<Bot size={15} strokeWidth={1.8} aria-hidden="true" />{:else}你{/if}</span>
          <div class="diagnosis-message-content">
            <div class="diagnosis-message-meta"><small>{formatDate(message.created_at)}</small></div>
            {#if diagnosisEditingMessageId === message.id}
              <div
                class="diagnosis-edit-box"
                style={`width:${editingMessageSize ? `${editingMessageSize.width}px` : '25%'};height:${editingMessageSize?.height ?? 34}px`}
              >
                <textarea bind:value={diagnosisEditDraft} aria-label="编辑诊断问题" rows="1"></textarea>
                <div class="diagnosis-edit-actions">
                  <button class="secondary" aria-label="取消编辑" title="取消编辑" on:click={cancelMessageEdit}><X size={13} /></button
                  ><button class="primary" aria-label="重新发送" title="重新发送" on:click={() => void saveMessageEdit()} disabled={busy}><Check size={13} /></button>
                </div>
              </div>
              {#if message.role === 'user'}
                <div class="diagnosis-user-actions diagnosis-user-actions-placeholder" aria-hidden="true"></div>
              {/if}
            {:else}
              {#if message.role === 'assistant' && processTimeline.length}
                <svelte:element
                    this={isStreaming && !diagnosisAnswerCompleted ? 'div' : 'details'}
                    class="diagnosis-process"
                    open={isStreaming ? diagnosisAnswerCompleted && diagnosisLiveProcessExpanded : diagnosisProcessExpanded[message.id]}
                    on:toggle={(event: Event) => {
                      const details = event.currentTarget as HTMLDetailsElement;
                      if (isStreaming) diagnosisLiveProcessExpanded = details.open;
                      else diagnosisProcessExpanded = { ...diagnosisProcessExpanded, [message.id]: details.open };
                    }}
                  >
                    {#if !isStreaming || diagnosisAnswerCompleted}<summary class="diagnosis-process-summary">
                      <span class="diagnosis-process-title">执行过程</span>
                      <span class="diagnosis-process-meta">总耗时 {processDuration(processTimeline)} · {#if processActionCount(processTimeline)}{processActionCount(processTimeline)} 个动作 · {/if}{isStreaming ? diagnosisStatusLabel(diagnosisSnapshot?.session.status ?? '') : '已完成'}</span>
                    </summary>
                    {/if}
                    <div class="diagnosis-process-body">
                      <div class="diagnosis-live-timeline" aria-label="诊断执行过程">
                        {#each processTimeline as item (item.id)}
                          {#if item.kind === 'analysis'}
                            {@const analysisText = item.text ?? ''}
                            {#key analysisText}<div class="diagnosis-live-analysis diagnosis-markdown">{@html renderMarkdown(analysisText)}</div>{/key}
                          {:else}
                            {@const actions = item.actions ?? [item]}
                            {#if actions.length > 1}
                              <div class="diagnosis-live-action-wrap">
                                <button class="diagnosis-live-action summary" aria-expanded={Boolean(diagnosisActionExpanded[`process-action-${item.id}`])} on:click={() => toggleExpanded(`process-action-${item.id}`)}>
                                  <span class="diagnosis-live-action-chevron" aria-hidden="true">{diagnosisActionExpanded[`process-action-${item.id}`] ? '⌄' : '›'}</span><span class="diagnosis-live-action-icon" aria-hidden="true"><Wrench size={13} /></span><span class="diagnosis-live-action-label">调用工具 · {actions.length} 个动作</span><span class="diagnosis-live-action-status" class:error={item.status !== '已完成'}>{item.status}</span><small>{item.duration}</small><small>总耗时 {item.elapsed}</small>
                                </button>
                                {#if diagnosisActionExpanded[`process-action-${item.id}`]}
                                  <div class="diagnosis-live-action-children">
                                    {#each actions as action (action.id)}
                                      {@const actionKey = `process-action-child-${action.id}`}
                                      <div class="diagnosis-live-action-child-wrap">
                                        <button class="diagnosis-live-action child" aria-expanded={Boolean(diagnosisActionExpanded[actionKey])} on:click={() => toggleExpanded(actionKey)}>
                                          <span class="diagnosis-live-action-chevron" aria-hidden="true">{diagnosisActionExpanded[actionKey] ? '⌄' : '›'}</span><span class="diagnosis-live-action-icon" aria-hidden="true"><Wrench size={12} /></span><span class="diagnosis-live-action-label">{diagnosisActionLabel(action)}</span><span class="diagnosis-live-action-status" class:error={action.status !== '已完成'}>{action.status}</span><small>{action.duration}</small>
                                        </button>
                                        {#if diagnosisActionExpanded[actionKey]}<div class="diagnosis-live-action-detail child-detail"><div><small>入参</small><pre>{action.input ?? '暂无入参'}</pre></div><div><small>出参</small><pre>{action.output ?? '暂无出参'}</pre></div></div>{/if}
                                      </div>
                                    {/each}
                                  </div>
                                {/if}
                              </div>
                            {:else}
                              {#each actions as action (action.id)}
                                {@const actionKey = `process-action-${action.id}`}
                                <div class="diagnosis-live-action-wrap">
                                  <button class="diagnosis-live-action" aria-expanded={Boolean(diagnosisActionExpanded[actionKey])} on:click={() => toggleExpanded(actionKey)}><span class="diagnosis-live-action-chevron" aria-hidden="true">{diagnosisActionExpanded[actionKey] ? '⌄' : '›'}</span><span class="diagnosis-live-action-icon" aria-hidden="true"><Wrench size={13} /></span><span class="diagnosis-live-action-label">{diagnosisActionLabel(action)}</span><span class="diagnosis-live-action-status" class:error={action.status !== '已完成'}>{action.status}</span><small>{action.duration}</small><small>总耗时 {action.elapsed}</small></button>
                                  {#if diagnosisActionExpanded[actionKey]}<div class="diagnosis-live-action-detail"><div><small>入参</small><pre>{action.input ?? '暂无入参'}</pre></div><div><small>出参</small><pre>{action.output ?? '暂无出参'}</pre></div></div>{/if}
                                </div>
                              {/each}
                            {/if}
                          {/if}
                        {/each}
                      </div>
                    </div>
                </svelte:element>
              {/if}
              <div class="diagnosis-bubble-f">
                {#if message.content}<div class="diagnosis-markdown">{@html renderMarkdown(message.content)}</div>{/if}
                {#if diagnosisInterruptedReason && isStreaming}<span class="diagnosis-interruption"><i></i>回答已中断：{diagnosisInterruptedReason}</span>{/if}
              </div>
              {#if message.role === 'user'}
                <div class="diagnosis-user-actions">
                  {#if isLastDiagnosisUser(index)}<button class="diagnosis-user-action" aria-label="编辑并重新发送" title="编辑并重新发送" on:click={(event) => beginMessageEdit(message, event)}><Pencil size={13} /></button>{/if}
                  <button class="diagnosis-user-action" aria-label="复制问题" title="复制问题" on:click={() => copyDiagnosisMessage(message, false)}><Copy size={13} /></button>
                </div>
              {/if}
              {#if message.role === 'assistant' && (!isStreaming || diagnosisAnswerCompleted)}
                <div class="diagnosis-answer-actions"><button class="diagnosis-answer-copy" aria-label="复制回答" title="复制回答" on:click={() => isStreaming ? copyStreamingAnswer() : copyDiagnosisMessage(message, diagnosisProcessExpanded[message.id] ?? false)}><Copy size={14} />复制回答</button></div>
              {/if}
              {#if isStreaming && (diagnosisGenerating || (!diagnosisAnswerCompleted && diagnosisHasRunningActions(diagnosisSnapshot)))}
                <span class="diagnosis-thinking diagnosis-thinking-inline" aria-live="polite"><i></i><i></i><i></i><span>正在思考</span></span>
              {:else if isStreaming && !diagnosisAnswerCompleted && !diagnosisStreamingText && !processTimeline.length && !diagnosisInterruptedReason}
                <span class="diagnosis-thinking" aria-live="polite"><i></i><i></i><i></i><span>正在思考</span></span>
              {/if}
              {#if isStreaming && diagnosisShouldShowEmptyAnswer()}
                <span class="diagnosis-interruption"><i></i>模型未返回可展示的回答。</span>
              {/if}
            {/if}
          </div>
        </article>
    {/each}
  {/if}
</div>
