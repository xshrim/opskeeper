<script lang="ts">
  import { MessageSquarePlus } from 'lucide-svelte';
  import type { DiagnosisSnapshot, Resource } from '../../lib/api';

  export let title = '新建诊断会话';
  export let scopeLabel = '当前级别';
  export let diagnosisTargets: Resource[] = [];
  export let diagnosisTargetIds: string[] = [];
  export let generating = false;
  export let snapshot: DiagnosisSnapshot | null = null;
  export let statusLabel: (status: string) => string;
  export let onCreate: () => void;
</script>

<header class="diagnosis-conversation-head">
  <div class="diagnosis-conversation-title">
    <h1>{title}</h1>
    <small>{scopeLabel} · 只读证据链</small>
  </div>
  <div class="diagnosis-loaded-context">
    <span class="diagnosis-loaded-context-label">已加载上下文</span>
    <div class="diagnosis-context-resources">
      {#each diagnosisTargets.filter((resource) => diagnosisTargetIds.includes(resource.id)).slice(0, 3) as resource}
        <span class="diagnosis-context-chip">{resource.name}</span>
      {/each}
      {#if diagnosisTargetIds.length > 3}
        <span class="diagnosis-context-chip accent">+{diagnosisTargetIds.length - 3}</span>
      {/if}
      {#if diagnosisTargetIds.length === 0}
        <span class="diagnosis-context-chip muted">未选择</span>
      {/if}
    </div>
  </div>
  <div class="diagnosis-head-actions">
    <span class="diagnosis-head-status"
      ><i class:running={generating}></i>{generating
        ? '正在生成回答'
        : snapshot
          ? statusLabel(snapshot.session.status)
          : '等待提问'}</span
    >
    <button
      class="icon-button"
      type="button"
      aria-label="新建诊断会话"
      title="新建诊断会话"
      on:click={onCreate}
      ><MessageSquarePlus size={16} /></button
    >
  </div>
</header>
