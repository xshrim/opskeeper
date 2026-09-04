<script lang="ts">
  import type { Resource } from '../../lib/api';
  import type { StatusRow } from '../../lib/health';

  export let teamCount = 0;
  export let projectCount = 0;
  export let resourceCount = 0;
  export let healthStatus: 'ready' | 'not_ready' | undefined;
  export let rows: StatusRow[] = [];
  export let visibleResources: Resource[] = [];
  export let resourceSchemaName: (kind: string) => string;
  export let scopeName: (id: string) => string;
  export let resourceIcon: (kind: string) => string;
  export let onOpenResources: () => void;
  export let onOpenResource: (resource: Resource) => void;
</script>

<section class="content-grid">
  <div class="metric-grid">
    <article class="metric"><span class="metric-label">团队</span><strong>{teamCount}</strong><span class="metric-note">可访问的组织单元</span></article>
    <article class="metric"><span class="metric-label">项目</span><strong>{projectCount}</strong><span class="metric-note">当前作用域内</span></article>
    <article class="metric"><span class="metric-label">资源</span><strong>{resourceCount}</strong><span class="metric-note">已登记资源</span></article>
  </div>
  <section class="panel wide-panel" aria-labelledby="health-heading">
    <div class="panel-heading"><div><p class="eyebrow">SYSTEM</p><h2 id="health-heading">控制平面状态</h2></div><span class:healthy={healthStatus === 'ready'} class="status-pill"><span class="status-dot"></span>{healthStatus === 'ready' ? 'Ready' : 'Checking'}</span></div>
    <div class="status-table"><div class="table-header"><span>服务</span><span>状态</span><span>延迟</span></div>{#each rows as row}<div class="table-row"><span class="service-name">{row.name}</span><span class:up={row.status === 'up'} class:down={row.status === 'down'} class="service-status"><span class="status-dot"></span>{row.status === 'up' ? 'Operational' : row.status === 'down' ? 'Unavailable' : 'Checking'}</span><span class="latency">{row.latency ?? '—'}</span></div>{/each}</div>
  </section>
  <section class="panel recent-panel">
    <div class="panel-heading"><div><p class="eyebrow">CATALOG</p><h2>最近资源</h2></div><button class="text-button" on:click={onOpenResources}>查看全部 →</button></div>
    {#if visibleResources.length === 0}<div class="empty-state">当前作用域还没有资源。</div>{:else}<div class="compact-list">{#each visibleResources.slice(0, 5) as resource}<button class="compact-row" on:click={() => onOpenResource(resource)}><span class="entity-summary"><span class="entity-icon resource-icon">{resourceIcon(resource.kind)}</span><span><strong>{resource.name}</strong><small>{resourceSchemaName(resource.kind)} · {scopeName(resource.scope_id)}</small></span></span><span class="status-label {resource.status}">{resource.status}</span></button>{/each}</div>{/if}
  </section>
</section>
