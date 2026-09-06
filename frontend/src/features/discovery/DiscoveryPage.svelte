<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, ApiError, type DiscoveryItem, type DiscoveryProjectMapping, type DiscoveryRun, type Project, type Resource, type Team } from '../../lib/api';
  import { iconGlyph } from '../../lib/icons';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';

  type ProjectMappingDraft = DiscoveryProjectMapping & { mode: 'existing' | 'create' | 'ignore' };

  export let kubernetesClusters: Resource[] = [];
  export let teams: Team[] = [];
  export let projects: Project[] = [];
  export let scopeTypes: Record<string, string> = {};
  export let scopeName: (id: string) => string;
  export let formatDate: (value: string) => string;
  export let onWorkspaceReload: () => void | Promise<void>;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;

  let selectedClusterId = '';
  let activeDiscovery: DiscoveryRun | null = null;
  let discoveryRuns: DiscoveryRun[] = [];
  let discoveryItems: DiscoveryItem[] = [];
  let projectMappingDrafts: Record<string, ProjectMappingDraft> = {};
  let selectedDiscoveryItems: Record<string, boolean> = {};
  let busy = false;
  let destroyed = false;

  $: namespaceCandidates = discoveryItems.filter((item) => item.kind === 'Project');
  $: applicationCandidates = discoveryItems.filter((item) => item.kind === 'Application');
  $: if (!selectedClusterId && kubernetesClusters[0]) {
    selectedClusterId = kubernetesClusters[0].id;
    void loadRuns();
  }

  onDestroy(() => { destroyed = true; });

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 403) return '当前账号没有执行此操作的权限。';
      if (error.status === 401) return '会话已过期，请重新登录。';
      return error.message || fallback;
    }
    if (error instanceof Error) return error.message || fallback;
    return fallback;
  }

  function payloadCount(item: DiscoveryItem, key: string) {
    const value = item.payload[key];
    return Array.isArray(value) ? value.length : 0;
  }

  async function selectCluster() {
    activeDiscovery = null; discoveryItems = []; projectMappingDrafts = {}; selectedDiscoveryItems = {};
    await loadRuns();
  }
  async function loadRuns() {
    const clusterId = selectedClusterId;
    if (!clusterId) return;
    try {
      const runs = await api.discoveryRuns(clusterId);
      if (destroyed || clusterId !== selectedClusterId) return;
      discoveryRuns = runs;
      if (runs[0]) await openDiscovery(runs[0]);
    } catch (error) { onError(describeError(error, '集群同步历史加载失败')); }
  }
  async function startDiscovery() {
    if (!selectedClusterId) return;
    busy = true; onError('');
    try {
      activeDiscovery = await api.startDiscovery(selectedClusterId);
      discoveryRuns = [activeDiscovery, ...discoveryRuns]; discoveryItems = [];
      onNotice('集群扫描已开始'); void pollDiscovery(activeDiscovery.id);
    } catch (error) { onError(describeError(error, '启动集群扫描失败')); } finally { busy = false; }
  }
  async function pollDiscovery(id: string) {
    for (let attempt = 0; attempt < 120; attempt += 1) {
      await new Promise((resolve) => window.setTimeout(resolve, 1000));
      if (destroyed || activeDiscovery?.id !== id) return;
      try {
        const run = await api.discovery(id);
        if (destroyed || activeDiscovery?.id !== id) return;
        activeDiscovery = run; discoveryRuns = discoveryRuns.map((item) => item.id === run.id ? run : item);
        if (run.status === 'succeeded') { await loadItems(id); onNotice(`扫描完成，共发现 ${run.item_count} 个项目和应用候选`); return; }
        if (run.status === 'failed' || run.status === 'cancelled') { onError(run.error_message || '集群扫描失败'); return; }
      } catch (error) { onError(describeError(error, '扫描状态刷新失败')); return; }
    }
    onError('扫描仍在运行，可稍后从同步历史重新打开。');
  }
  async function openDiscovery(run: DiscoveryRun) {
    activeDiscovery = run;
    if (run.status === 'succeeded') await loadItems(run.id);
    else if (run.status === 'queued' || run.status === 'running') void pollDiscovery(run.id);
  }
  async function loadItems(id: string) {
    const items = await api.discoveryItems(id);
    if (destroyed || activeDiscovery?.id !== id) return;
    discoveryItems = items;
    selectedDiscoveryItems = Object.fromEntries(items.filter((item) => item.kind === 'Application').map((item) => [item.id, item.status !== 'ignored']));
    projectMappingDrafts = Object.fromEntries(items.filter((item) => item.kind === 'Project').map((item) => [item.namespace || item.name, defaultProjectMapping(item)]));
  }
  function allowedTeamsForCluster() {
    const cluster = kubernetesClusters.find((resource) => resource.id === selectedClusterId);
    if (!cluster) return [];
    if (scopeTypes[cluster.scope_id] === 'platform') return teams;
    if (scopeTypes[cluster.scope_id] === 'team') return teams.filter((team) => team.scope.id === cluster.scope_id);
    return teams.filter((team) => team.id === projects.find((item) => item.scope.id === cluster.scope_id)?.team_id);
  }
  function allowedProjectsForCluster() {
    const cluster = kubernetesClusters.find((resource) => resource.id === selectedClusterId);
    if (!cluster) return [];
    if (scopeTypes[cluster.scope_id] === 'platform') return projects;
    if (scopeTypes[cluster.scope_id] === 'team') return projects.filter((project) => project.team_id === teams.find((team) => team.scope.id === cluster.scope_id)?.id);
    return projects.filter((project) => project.scope.id === cluster.scope_id);
  }
  function defaultProjectMapping(item: DiscoveryItem): ProjectMappingDraft {
    const namespace = item.namespace || item.name;
    if (['kube-system', 'kube-public', 'kube-node-lease'].includes(namespace)) return { mode: 'ignore', ignore: true };
    const mapped = projects.find((project) => project.source_resource_id === selectedClusterId && project.external_uid === item.external_uid);
    if (mapped) return { mode: 'existing', project_id: mapped.id };
    const onlyProject = allowedProjectsForCluster()[0];
    const cluster = kubernetesClusters.find((resource) => resource.id === selectedClusterId);
    if (cluster && scopeTypes[cluster.scope_id] === 'project' && onlyProject) return { mode: 'existing', project_id: onlyProject.id };
    const team = allowedTeamsForCluster()[0];
    return { mode: 'create', team_id: team?.id || '', name: item.name, code: namespace.trim().toLowerCase().replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '') || 'kubernetes-project' };
  }
  async function importDiscovery() {
    if (!activeDiscovery) return;
    busy = true; onError('');
    try {
      const projectMappings = Object.fromEntries(Object.entries(projectMappingDrafts).map(([namespace, draft]) => [namespace, draft.mode === 'ignore' ? { ignore: true } : draft.mode === 'existing' ? { project_id: draft.project_id } : { team_id: draft.team_id, name: draft.name, code: draft.code }]));
      const result = await api.importDiscovery(activeDiscovery.id, { item_ids: Object.entries(selectedDiscoveryItems).filter(([, selected]) => selected).map(([id]) => id), project_mappings: projectMappings });
      activeDiscovery = result.run;
      await Promise.all([onWorkspaceReload(), loadItems(result.run.id)]);
      onNotice(`已映射 ${result.imported.filter((item) => item.kind === 'Project').length} 个项目并导入 ${result.imported.filter((item) => item.kind === 'Application').length} 个应用`);
    } catch (error) { onError(describeError(error, '导入集群发现结果失败')); } finally { busy = false; }
  }
</script>

<section class="discovery-layout">
  <section class="panel discovery-control">
    <div class="panel-heading"><div><p class="eyebrow">KUBERNETES SOURCE</p><h2>选择集群并扫描</h2></div><span class="entity-icon resource-icon"><ResourceBrandIcon resource={{ kind: 'Kubernetes' }} size={18} /></span></div>
    {#if kubernetesClusters.length === 0}
      <div class="empty-state">请先在资源目录登记 Kubernetes 集群及其 kubeconfig 凭据。</div>
    {:else}
      <div class="discovery-toolbar"><label>Kubernetes 集群<select bind:value={selectedClusterId} on:change={selectCluster}>{#each kubernetesClusters as cluster}<option value={cluster.id}>{cluster.name} · {scopeName(cluster.scope_id)}</option>{/each}</select></label><button class="primary" on:click={startDiscovery} disabled={busy || !selectedClusterId || activeDiscovery?.status === 'running' || activeDiscovery?.status === 'queued'}>开始扫描</button></div>
    {/if}
    {#if activeDiscovery}<div class="discovery-status"><span class="status-pill" class:healthy={activeDiscovery.status === 'succeeded'}><span class="status-dot"></span>{activeDiscovery.status}</span><span>{activeDiscovery.item_count} 个候选</span><span>{activeDiscovery.imported_count} 个已处理</span><span>{formatDate(activeDiscovery.created_at)}</span></div>{/if}
    {#if discoveryRuns.length > 0}<div class="run-history" aria-label="同步历史">{#each discoveryRuns.slice(0, 6) as run}<button class:active={activeDiscovery?.id === run.id} on:click={() => void openDiscovery(run)}><span>{formatDate(run.created_at)}</span><strong>{run.status}</strong><small>{run.item_count} 项</small></button>{/each}</div>{/if}
  </section>

  {#if activeDiscovery?.status === 'succeeded'}
    <section class="panel mapping-panel">
      <div class="panel-heading"><div><p class="eyebrow">NAMESPACE MAPPING</p><h2>命名空间映射项目</h2></div><span class="count">{namespaceCandidates.length}</span></div>
      <div class="mapping-list">
        {#each namespaceCandidates as item}
          {@const namespace = item.namespace || item.name}
          {@const draft = projectMappingDrafts[namespace]}
          {#if draft}<div class="mapping-row"><div class="mapping-source"><span class="entity-icon project-icon"><ResourceBrandIcon resource={{ kind: 'Project' }} fallback={iconGlyph('project')} size={17} /></span><span><strong>{namespace}</strong><small>Namespace · {item.external_uid.slice(0, 12)}</small></span></div><label>处理方式<select bind:value={draft.mode}><option value="existing">映射已有项目</option><option value="create">创建新项目</option><option value="ignore">忽略</option></select></label>{#if draft.mode === 'existing'}<label>目标项目<select bind:value={draft.project_id} required><option value="" disabled>选择项目</option>{#each allowedProjectsForCluster() as project}<option value={project.id}>{project.name} · {teams.find((team) => team.id === project.team_id)?.name}</option>{/each}</select></label>{:else if draft.mode === 'create'}<div class="mapping-create-fields"><label>所属团队<select bind:value={draft.team_id} required><option value="" disabled>选择团队</option>{#each allowedTeamsForCluster() as team}<option value={team.id}>{team.name}</option>{/each}</select></label><label>项目名称<input bind:value={draft.name} required /></label><label>项目编码<input bind:value={draft.code} required /></label></div>{:else}<p class="mapping-note">该命名空间及其工作负载不会进入项目和应用目录。</p>{/if}</div>{/if}
        {/each}
      </div>
    </section>

    <section class="panel application-preview-panel">
      <div class="panel-heading"><div><p class="eyebrow">APPLICATION PREVIEW</p><h2>工作负载映射应用</h2></div><span class="count">{applicationCandidates.length}</span></div>
      <div class="application-preview-list">{#each applicationCandidates as item}<label class="application-preview-row"><input type="checkbox" bind:checked={selectedDiscoveryItems[item.id]} disabled={projectMappingDrafts[item.namespace || '']?.mode === 'ignore'} /><span class="entity-icon resource-icon"><ResourceBrandIcon resource={{ kind: 'Application' }} fallback={iconGlyph('application')} size={17} /></span><span class="application-identity"><strong>{item.name}</strong><small>{item.namespace} · {String((item.payload.kubernetes as Record<string, unknown> | undefined)?.workload_kind || 'Workload')}</small></span><span class="application-facts"><span>{payloadCount(item, 'services')} Service</span><span>{payloadCount(item, 'ingresses')} Ingress</span><span>{payloadCount(item, 'endpoints')} Endpoint</span><span>{payloadCount(item, 'instances')} Instance</span></span></label>{:else}<div class="empty-state">集群中没有可导入的工作负载。</div>{/each}</div>
      <div class="import-actions"><p class="muted">确认后创建或绑定项目，并将选中的工作负载写入项目应用；Kubernetes 子对象不会登记为独立资源。</p><button class="primary" on:click={importDiscovery} disabled={busy || namespaceCandidates.length === 0}>确认导入项目与应用</button></div>
    </section>
  {/if}
</section>
