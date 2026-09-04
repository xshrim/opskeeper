<script lang="ts">
  import type { DiscoveryItem, DiscoveryProjectMapping, DiscoveryRun, Project, Resource, Team } from '../../lib/api';

  type ProjectMappingDraft = DiscoveryProjectMapping & { mode: 'existing' | 'create' | 'ignore' };

  export let kubernetesClusters: Resource[] = [];
  export let selectedClusterId = '';
  export let activeDiscovery: DiscoveryRun | null = null;
  export let discoveryRuns: DiscoveryRun[] = [];
  export let namespaceCandidates: DiscoveryItem[] = [];
  export let applicationCandidates: DiscoveryItem[] = [];
  export let projectMappingDrafts: Record<string, ProjectMappingDraft> = {};
  export let selectedDiscoveryItems: Record<string, boolean> = {};
  export let teams: Team[] = [];
  export let busy = false;
  export let scopeName: (id: string) => string;
  export let formatDate: (value: string) => string;
  export let payloadCount: (item: DiscoveryItem, key: string) => number;
  export let allowedTeamsForCluster: () => Team[];
  export let allowedProjectsForCluster: () => Project[];
  export let onSelectCluster: () => void | Promise<void>;
  export let onStartDiscovery: () => void | Promise<void>;
  export let onOpenDiscovery: (run: DiscoveryRun) => void | Promise<void>;
  export let onImportDiscovery: () => void | Promise<void>;
</script>

<section class="discovery-layout">
  <section class="panel discovery-control">
    <div class="panel-heading"><div><p class="eyebrow">KUBERNETES SOURCE</p><h2>选择集群并扫描</h2></div><span class="entity-icon resource-icon" aria-hidden="true">☸</span></div>
    {#if kubernetesClusters.length === 0}
      <div class="empty-state">请先在资源目录登记 Kubernetes 集群及其 kubeconfig 凭据。</div>
    {:else}
      <div class="discovery-toolbar"><label>Kubernetes 集群<select bind:value={selectedClusterId} on:change={onSelectCluster}>{#each kubernetesClusters as cluster}<option value={cluster.id}>{cluster.name} · {scopeName(cluster.scope_id)}</option>{/each}</select></label><button class="primary" on:click={onStartDiscovery} disabled={busy || !selectedClusterId || activeDiscovery?.status === 'running' || activeDiscovery?.status === 'queued'}>开始扫描</button></div>
    {/if}
    {#if activeDiscovery}<div class="discovery-status"><span class="status-pill" class:healthy={activeDiscovery.status === 'succeeded'}><span class="status-dot"></span>{activeDiscovery.status}</span><span>{activeDiscovery.item_count} 个候选</span><span>{activeDiscovery.imported_count} 个已处理</span><span>{formatDate(activeDiscovery.created_at)}</span></div>{/if}
    {#if discoveryRuns.length > 0}<div class="run-history" aria-label="同步历史">{#each discoveryRuns.slice(0, 6) as run}<button class:active={activeDiscovery?.id === run.id} on:click={() => void onOpenDiscovery(run)}><span>{formatDate(run.created_at)}</span><strong>{run.status}</strong><small>{run.item_count} 项</small></button>{/each}</div>{/if}
  </section>

  {#if activeDiscovery?.status === 'succeeded'}
    <section class="panel mapping-panel">
      <div class="panel-heading"><div><p class="eyebrow">NAMESPACE MAPPING</p><h2>命名空间映射项目</h2></div><span class="count">{namespaceCandidates.length}</span></div>
      <div class="mapping-list">
        {#each namespaceCandidates as item}
          {@const namespace = item.namespace || item.name}
          {@const draft = projectMappingDrafts[namespace]}
          {#if draft}<div class="mapping-row"><div class="mapping-source"><span class="entity-icon project-icon">▰</span><span><strong>{namespace}</strong><small>Namespace · {item.external_uid.slice(0, 12)}</small></span></div><label>处理方式<select bind:value={draft.mode}><option value="existing">映射已有项目</option><option value="create">创建新项目</option><option value="ignore">忽略</option></select></label>{#if draft.mode === 'existing'}<label>目标项目<select bind:value={draft.project_id} required><option value="" disabled>选择项目</option>{#each allowedProjectsForCluster() as project}<option value={project.id}>{project.name} · {teams.find((team) => team.id === project.team_id)?.name}</option>{/each}</select></label>{:else if draft.mode === 'create'}<div class="mapping-create-fields"><label>所属团队<select bind:value={draft.team_id} required><option value="" disabled>选择团队</option>{#each allowedTeamsForCluster() as team}<option value={team.id}>{team.name}</option>{/each}</select></label><label>项目名称<input bind:value={draft.name} required /></label><label>项目编码<input bind:value={draft.code} required /></label></div>{:else}<p class="mapping-note">该命名空间及其工作负载不会进入项目和应用目录。</p>{/if}</div>{/if}
        {/each}
      </div>
    </section>

    <section class="panel application-preview-panel">
      <div class="panel-heading"><div><p class="eyebrow">APPLICATION PREVIEW</p><h2>工作负载映射应用</h2></div><span class="count">{applicationCandidates.length}</span></div>
      <div class="application-preview-list">{#each applicationCandidates as item}<label class="application-preview-row"><input type="checkbox" bind:checked={selectedDiscoveryItems[item.id]} disabled={projectMappingDrafts[item.namespace || '']?.mode === 'ignore'} /><span class="entity-icon resource-icon">⌘</span><span class="application-identity"><strong>{item.name}</strong><small>{item.namespace} · {String((item.payload.kubernetes as Record<string, unknown> | undefined)?.workload_kind || 'Workload')}</small></span><span class="application-facts"><span>{payloadCount(item, 'services')} Service</span><span>{payloadCount(item, 'ingresses')} Ingress</span><span>{payloadCount(item, 'endpoints')} Endpoint</span><span>{payloadCount(item, 'instances')} Instance</span></span></label>{:else}<div class="empty-state">集群中没有可导入的工作负载。</div>{/each}</div>
      <div class="import-actions"><p class="muted">确认后创建或绑定项目，并将选中的工作负载写入项目应用；Kubernetes 子对象不会登记为独立资源。</p><button class="primary" on:click={onImportDiscovery} disabled={busy || namespaceCandidates.length === 0}>确认导入项目与应用</button></div>
    </section>
  {/if}
</section>
