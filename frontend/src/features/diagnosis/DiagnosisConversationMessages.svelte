<script lang="ts">
  import { Bot, Copy, Pencil, Stethoscope, Wrench } from 'lucide-svelte';
  import type { DiagnosisMessage, DiagnosisSnapshot } from '../../lib/api';

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
  export let diagnosisProcessDuration: (snapshot: DiagnosisSnapshot | null) => string;
  export let diagnosisProcessActionCount: (snapshot: DiagnosisSnapshot | null) => number;
  export let diagnosisStatusLabel: (status: string) => string;
  export let diagnosisHasRunningActions: (snapshot: DiagnosisSnapshot | null) => boolean;
  export let diagnosisHasPersistedNewAnswer: (snapshot: DiagnosisSnapshot | null) => boolean;
  export let diagnosisActionLabel: (item: DiagnosisLiveTimelineItem) => string;
  export let deferFinalDiagnosisMessage: (message: DiagnosisMessage, index: number) => boolean;
  export let isLastDiagnosisUser: (index: number) => boolean;
  export let beginDiagnosisEdit: (message: DiagnosisMessage) => void;
  export let saveDiagnosisEdit: () => void | Promise<void>;
  export let copyDiagnosisMessage: (message: DiagnosisMessage, processExpanded: boolean) => void;
  export let copyStreamingAnswer: () => void;

  const toggleExpanded = (key: string) => {
    diagnosisActionExpanded = {
      ...diagnosisActionExpanded,
      [key]: !diagnosisActionExpanded[key]
    };
  };
</script>

<div class="diagnosis-message-list-f" bind:this={diagnosisMessageListElement}>
  {#if !diagnosisSnapshot}
    <div class="diagnosis-welcome">
      <span class="diagnosis-welcome-icon"><Stethoscope size={23} /></span>
      <h2>从一个问题开始</h2>
      <p>
        可直接提问；如需查询资源状态、内容或性能，请在右侧选择已授权的上下文资源。
        AIEngine 会按需调用受控只读工具。
      </p>
    </div>
  {/if}
  {#if diagnosisSnapshot}
    {#each diagnosisSnapshot.messages as message, index}
      {#if !deferFinalDiagnosisMessage(message, index)}
        <article
          class="diagnosis-message-f {message.role}"
          data-diagnosis-message-id={message.id}
          data-diagnosis-message-content={message.role === 'user' ? message.content : undefined}
        >
          <span class="diagnosis-message-avatar">{#if message.role === 'assistant'}<Bot size={15} strokeWidth={1.8} aria-hidden="true" />{:else}你{/if}</span>
          <div class="diagnosis-message-content">
            <div class="diagnosis-message-meta"><small>{formatDate(message.created_at)}</small></div>
            {#if diagnosisEditingMessageId === message.id}
              <div class="diagnosis-edit-box">
                <textarea bind:value={diagnosisEditDraft} aria-label="编辑诊断问题" rows="3"></textarea>
                <div>
                  <button class="secondary" on:click={() => (diagnosisEditingMessageId = '')}>取消</button
                  ><button class="primary" on:click={() => void saveDiagnosisEdit()} disabled={busy}>重新发送</button>
                </div>
              </div>
            {:else}
              {#if message.role === 'assistant' && !diagnosisGenerating && index === diagnosisSnapshot.messages.map((item) => item.role).lastIndexOf('assistant')}
                {#if diagnosisLiveTimeline(diagnosisSnapshot).length}
                  <details
                    class="diagnosis-process"
                    on:toggle={(event) => {
                      const details = event.currentTarget as HTMLDetailsElement;
                      diagnosisProcessExpanded = { ...diagnosisProcessExpanded, [message.id]: details.open };
                    }}
                  >
                    <summary class="diagnosis-process-summary">
                      <span class="diagnosis-process-title">执行过程</span>
                      <span class="diagnosis-process-meta">总耗时 {diagnosisProcessDuration(diagnosisSnapshot)} · {#if diagnosisProcessActionCount(diagnosisSnapshot)}{diagnosisProcessActionCount(diagnosisSnapshot)} 个动作 · {/if}{diagnosisStatusLabel(diagnosisSnapshot.session.status)}</span>
                    </summary>
                    <div class="diagnosis-process-body">
                      <div class="diagnosis-live-timeline" aria-label="诊断执行过程">
                        {#each diagnosisLiveTimeline(diagnosisSnapshot) as item (item.id)}
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
                  </details>
                {/if}
              {/if}
              <div class="diagnosis-bubble-f">
                <div class="diagnosis-markdown">{@html renderMarkdown(message.content)}</div>
                {#if diagnosisInterruptedReason && message.role === 'assistant' && index === diagnosisSnapshot.messages.map((item) => item.role).lastIndexOf('assistant')}<span class="diagnosis-interruption"><i></i>回答已中断：{diagnosisInterruptedReason}</span>{/if}
                {#if message.role === 'user' && isLastDiagnosisUser(index)}<button class="bubble-icon edit" aria-label="编辑并重新发送" title="编辑并重新发送" on:click={() => beginDiagnosisEdit(message)}><Pencil size={14} /></button>{/if}
              </div>
              {#if message.role === 'assistant'}
                <div class="diagnosis-answer-actions"><button class="diagnosis-answer-copy" aria-label="复制回答" title="复制回答" on:click={() => copyDiagnosisMessage(message, diagnosisProcessExpanded[message.id] ?? false)}><Copy size={14} />复制回答</button></div>
              {/if}
            {/if}
          </div>
        </article>
      {/if}
    {/each}
  {/if}
  {#if diagnosisAnswerCompleted || (diagnosisGenerating || diagnosisStreamingText || diagnosisInterruptedReason || diagnosisHasRunningActions(diagnosisSnapshot)) && !diagnosisHasPersistedNewAnswer(diagnosisSnapshot)}
    <article class="diagnosis-message-f assistant diagnosis-streaming-message">
      <span class="diagnosis-message-avatar"><Bot size={15} strokeWidth={1.8} aria-hidden="true" /></span>
      <div class="diagnosis-message-content">
        <div class="diagnosis-message-meta"><small>{formatDate(diagnosisStreamingStartedAt ? new Date(diagnosisStreamingStartedAt).toISOString() : new Date().toISOString())}</small></div>
        <div class="diagnosis-bubble-f diagnosis-streaming-bubble">
          {#if diagnosisLiveTimeline(diagnosisSnapshot).length}
            <svelte:element
              this={diagnosisAnswerCompleted ? 'details' : 'div'}
              class="diagnosis-live-process"
              class:final={diagnosisAnswerCompleted}
              open={diagnosisAnswerCompleted ? diagnosisLiveProcessExpanded : undefined}
              on:toggle={(event: Event) => {
                if (diagnosisAnswerCompleted) diagnosisLiveProcessExpanded = (event.currentTarget as HTMLDetailsElement).open;
              }}
            >
              {#if diagnosisAnswerCompleted}<summary class="diagnosis-process-summary"><span class="diagnosis-process-title">执行过程</span><span class="diagnosis-process-meta">总耗时 {diagnosisProcessDuration(diagnosisSnapshot)} · {#if diagnosisProcessActionCount(diagnosisSnapshot)}{diagnosisProcessActionCount(diagnosisSnapshot)} 个动作 · {/if}{diagnosisStatusLabel(diagnosisSnapshot?.session.status ?? '')}</span></summary>{/if}
              <div class="diagnosis-live-timeline" aria-live="polite">
                {#each diagnosisLiveTimeline(diagnosisSnapshot) as item (item.id)}
                  {#if item.kind === 'analysis'}
                    {@const analysisText = item.text ?? ''}
                    {#key analysisText}<div class="diagnosis-live-analysis diagnosis-markdown">{@html renderMarkdown(analysisText)}</div>{/key}
                  {:else}
                    {@const actions = item.actions ?? [item]}
                    {#if actions.length > 1}
                      {@const groupKey = `live-group-${item.id}`}
                      <div class="diagnosis-live-action-wrap diagnosis-live-action-group">
                        <button class="diagnosis-live-action summary" aria-expanded={Boolean(diagnosisActionExpanded[groupKey])} on:click={() => toggleExpanded(groupKey)}><span class="diagnosis-live-action-chevron" aria-hidden="true">{diagnosisActionExpanded[groupKey] ? '⌄' : '›'}</span><span class="diagnosis-live-action-icon" aria-hidden="true"><Wrench size={13} /></span><span class="diagnosis-live-action-label">调用工具 · {actions.length} 个动作</span><span class="diagnosis-live-action-status" class:error={item.status !== '已完成'}>{item.status}</span><small>{item.duration}</small><small>总耗时 {item.elapsed}</small></button>
                        {#if diagnosisActionExpanded[groupKey]}<div class="diagnosis-live-action-children">{#each actions as action (action.id)}{@const actionKey = `live-action-${action.id}`}<div class="diagnosis-live-action-child-wrap"><button class="diagnosis-live-action child" aria-expanded={Boolean(diagnosisActionExpanded[actionKey])} on:click={() => toggleExpanded(actionKey)}><span class="diagnosis-live-action-chevron" aria-hidden="true">{diagnosisActionExpanded[actionKey] ? '⌄' : '›'}</span><span class="diagnosis-live-action-icon" aria-hidden="true"><Wrench size={12} /></span><span class="diagnosis-live-action-label">{diagnosisActionLabel(action)}</span><span class="diagnosis-live-action-status" class:error={action.status !== '已完成'}>{action.status}</span><small>{action.duration}</small></button>{#if diagnosisActionExpanded[actionKey]}<div class="diagnosis-live-action-detail child-detail"><div><small>入参</small><pre>{action.input ?? '暂无入参'}</pre></div><div><small>出参</small><pre>{action.output ?? '暂无出参'}</pre></div></div>{/if}</div>{/each}</div>{/if}
                      </div>
                    {:else}
                      {#each actions as action (action.id)}{@const actionKey = `live-action-${action.id}`}<div class="diagnosis-live-action-wrap"><button class="diagnosis-live-action" aria-expanded={Boolean(diagnosisActionExpanded[actionKey])} on:click={() => toggleExpanded(actionKey)}><span class="diagnosis-live-action-chevron" aria-hidden="true">{diagnosisActionExpanded[actionKey] ? '⌄' : '›'}</span><span class="diagnosis-live-action-icon" aria-hidden="true"><Wrench size={13} /></span><span class="diagnosis-live-action-label">{diagnosisActionLabel(action)}</span><span class="diagnosis-live-action-status" class:error={action.status !== '已完成'}>{action.status}</span><small>{action.duration}</small><small>总耗时 {action.elapsed}</small></button>{#if diagnosisActionExpanded[actionKey]}<div class="diagnosis-live-action-detail"><div><small>入参</small><pre>{action.input ?? '暂无入参'}</pre></div><div><small>出参</small><pre>{action.output ?? '暂无出参'}</pre></div></div>{/if}</div>{/each}
                    {/if}
                  {/if}
                {/each}
              </div>
            </svelte:element>
          {/if}
          {#if diagnosisStreamingText}{#key diagnosisStreamingText}<div class="diagnosis-markdown">{@html renderMarkdown(diagnosisStreamingText)}</div>{/key}{/if}
          {#if diagnosisGenerating || diagnosisHasRunningActions(diagnosisSnapshot)}
            <span class="diagnosis-thinking diagnosis-thinking-inline" aria-live="polite"><i></i><i></i><i></i><span>正在思考</span></span>
          {:else if !diagnosisStreamingText && !diagnosisLiveTimeline(diagnosisSnapshot).length && !diagnosisInterruptedReason}
            <span class="diagnosis-thinking" aria-live="polite"><i></i><i></i><i></i><span>正在思考</span></span>
          {/if}
          {#if diagnosisInterruptedReason}<span class="diagnosis-interruption"><i></i>回答已中断：{diagnosisInterruptedReason}</span>{/if}
        </div>
        {#if diagnosisAnswerCompleted}<div class="diagnosis-answer-actions diagnosis-answer-actions-visible"><button class="diagnosis-answer-copy" aria-label="复制回答" title="复制回答" on:click={copyStreamingAnswer}><Copy size={14} />复制回答</button></div>{/if}
      </div>
    </article>
  {/if}
</div>
