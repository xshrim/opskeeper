<script lang="ts">
  import { Pencil, Plus, Trash2 } from 'lucide-svelte';
  import type { DiagnosisSession } from '../../lib/api';

  export let sessions: DiagnosisSession[] = [];
  export let selectedSessionId = '';
  export let search = '';
  export let statusLabel: (status: DiagnosisSession['status']) => string;
  export let formatDate: (value: string) => string;
  export let onClear: () => void;
  export let onCreate: () => void;
  export let onOpen: (sessionID: string) => void;
  export let onRename: (session: DiagnosisSession) => void;
  export let onDelete: (session: DiagnosisSession) => void;
</script>

<aside class="diagnosis-history-panel">
  <div class="diagnosis-panel-top">
    <div>
      <h2>会话历史</h2>
      <small>{sessions.length} 个会话</small>
    </div>
    <div class="diagnosis-heading-actions">
      <button
        class="icon-button"
        aria-label="清空会话历史"
        title="清空会话历史"
        on:click={onClear}
        ><Trash2 size={15} /></button
      >
      <button
        class="icon-button"
        aria-label="新建诊断会话"
        title="新建诊断会话"
        on:click={onCreate}><Plus size={16} /></button
      >
    </div>
  </div>
  <input
    class="diagnosis-session-search"
    bind:value={search}
    placeholder="搜索会话"
    aria-label="搜索会话"
  />
  <div class="diagnosis-session-list-f">
    {#each sessions.filter((session) => !search.trim() || (session.title || '').toLowerCase().includes(search.toLowerCase())) as session}
      <div class="diagnosis-session-item">
        <button
          class:active={selectedSessionId === session.id}
          class="diagnosis-session-row-f"
          on:click={() => onOpen(session.id)}
        >
          <strong class="diagnosis-session-title-f">{session.title || '未命名诊断'}</strong>
          <span class="diagnosis-session-meta-f"
            ><small>{formatDate(session.created_at)}</small><em
              class={`diagnosis-session-status-f ${session.status}`}
              >{statusLabel(session.status)}</em
            ></span
          >
        </button>
        <div class="diagnosis-session-actions">
          <button
            aria-label="重命名会话"
            title="重命名会话"
            on:click|stopPropagation={() => onRename(session)}
            ><Pencil size={13} /></button
          >
          <button
            aria-label="删除会话"
            title="删除会话"
            on:click|stopPropagation={() => onDelete(session)}
            ><Trash2 size={13} /></button
          >
        </div>
      </div>
    {:else}
      <p class="diagnosis-empty">还没有诊断会话。</p>
    {/each}
  </div>
</aside>
