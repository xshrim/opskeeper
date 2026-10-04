<script lang="ts">
  import {
    AlertTriangle,
    ArrowLeft,
    ArrowRight,
    CheckCircle2,
    ChevronDown,
    Map,
    Network,
    Pencil,
    Plus,
    Server,
    Sparkles,
    Upload,
    X
  } from 'lucide-svelte';
  import {
    api,
    ApiError,
    type Application,
    type Project,
    type ProjectWorkspace,
    type Team
  } from '../../lib/api';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import IconPicker from '../../components/IconPicker.svelte';
  import ScopeChip from '../../components/ScopeChip.svelte';

  export let teams: Team[] = [];
  export let allProjects: Project[] = [];
  export let visibleProjects: Project[] = [];
  export let selectedScopeId = '';
  export let selectedProjectId = '';
  export let allTeamsSelected = false;
  export let busy = false;
  export let onSelectTeam: (team: Team) => void;
  export let onSelectAllTeams: () => void = () => {};
  export let onSelectProject: (project: Project) => void;
  export let onOpenTeamDialog: () => void;
  export let onProjectCreated: (project: Project) => void;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;
  export let onOpenDiagnosis: (project: Project) => void;
  export let onOpenApplicationDiagnosis: (project: Project, application: Application) => void = () => {};

  type WizardMode = 'project' | 'application' | 'dependency';
  type ProjectSource = 'manual' | 'kubernetes';
  type RuntimeKind = 'virtual_machine' | 'containerized' | 'cloud_native';

  let projectTeamId = '';
  let projectName = '';
  let projectCode = '';
  let projectIcon = 'lucide:FolderKanban';
  let projectDescription = '';
  let projectSource: ProjectSource = 'manual';
  let kubernetesCluster = '';
  let kubernetesNamespace = 'default';
  let kubernetesSelector = 'app.kubernetes.io/part-of=payments';
  let kubernetesCandidates = [
    { name: 'payments-api', selected: true },
    { name: 'billing-worker', selected: true },
    { name: 'settlement-job', selected: false }
  ];

  let workspace: ProjectWorkspace | null = null;
  let workspaceProjectId = '';
  let loadingWorkspace = false;
  let expanded = new Set<string>();
  let editingAppId = '';
  let editName = '';
  let editDescription = '';
  let editIcon = 'lucide:AppWindow';
  let importInput: HTMLInputElement;

  let wizardOpen = false;
  let wizardMode: WizardMode = 'project';
  let wizardStep = 0;
  let wizardProjectId = '';
  let wizardApplicationId = '';
  let wizardInstanceSaved = false;
  let wizardDependencySaved = false;
  let wizardSaving = false;
  let applicationName = '';
  let applicationCode = '';
  let applicationDescription = '';
  let applicationIcon = 'lucide:AppWindow';
  let applicationSource: 'manual' | 'runtime' = 'manual';
  let runtimeKind: RuntimeKind = 'virtual_machine';
  let runtimeResourceId = '';
  let relationName = '';
  let relationBinding = '';
  let relationKind = 'database';
  let relationResourceId = '';
  let relationRequired = true;
  let relationAppId = '';
  let collapsedTeams = new Set<string>();
  let initializedTeamKey = '';

  $: if (!projectTeamId && teams[0]) projectTeamId = teams[0].id;
  $: selectedProject = visibleProjects.find((item) => item.id === selectedProjectId) ?? null;
  $: groupedProjects = teams.map((team) => ({ team, projects: visibleProjects.filter((project) => project.team_id === team.id) }));
  $: teamKey = teams.map((team) => team.id).join(',');
  $: if (teamKey !== initializedTeamKey) {
    initializedTeamKey = teamKey;
    collapsedTeams = new Set(teams.slice(3).map((team) => team.id));
  }
  $: if (selectedProjectId && selectedProjectId !== workspaceProjectId) void loadWorkspace(selectedProjectId);
  $: if (selectedProjectId === '') {
    workspace = null;
    workspaceProjectId = '';
    expanded = new Set<string>();
  }

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) return error.message || fallback;
    return error instanceof Error ? error.message || fallback : fallback;
  }

  function teamName(id: string) {
    return teams.find((team) => team.id === id)?.name ?? '未分配团队';
  }

  function sourceLabel(source: string) {
    if (source === 'kubernetes') return 'Kubernetes 导入';
    if (source === 'file') return '文件导入';
    return '手动创建';
  }

  function statusLabel(status: string) {
    if (status === 'active' || status === 'healthy' || status === 'up') return '正常';
    if (status === 'warning') return '需关注';
    if (status === 'disabled') return '已停用';
    return status || '未知';
  }

  function toggleTeam(teamID: string) {
    const next = new Set(collapsedTeams);
    if (next.has(teamID)) next.delete(teamID);
    else next.add(teamID);
    collapsedTeams = next;
  }

  function selectedRuntimeResource() {
    return (workspace?.resources ?? []).find((resource) => resource.id === runtimeResourceId) ?? null;
  }

  async function loadWorkspace(id: string) {
    loadingWorkspace = true;
    try {
      workspace = await api.projectWorkspace(id);
      workspaceProjectId = id;
      if (!runtimeResourceId) runtimeResourceId = workspace.resources[0]?.id ?? '';
      if (!relationResourceId) relationResourceId = workspace.resources[0]?.id ?? '';
    } catch (error) {
      onError(describeError(error, '项目驾驶舱加载失败'));
    } finally {
      loadingWorkspace = false;
    }
  }

  function resetWizard() {
    wizardOpen = false;
    wizardMode = 'project';
    wizardStep = 0;
    wizardProjectId = '';
    wizardApplicationId = '';
    wizardInstanceSaved = false;
    wizardDependencySaved = false;
    wizardSaving = false;
    projectName = '';
    projectCode = '';
    projectDescription = '';
    projectIcon = 'lucide:FolderKanban';
    projectSource = 'manual';
    kubernetesCluster = '';
    kubernetesNamespace = 'default';
    kubernetesSelector = 'app.kubernetes.io/part-of=payments';
    applicationName = '';
    applicationCode = '';
    applicationDescription = '';
    applicationIcon = 'lucide:AppWindow';
    applicationSource = 'manual';
    runtimeKind = 'virtual_machine';
    runtimeResourceId = workspace?.resources[0]?.id ?? '';
    relationName = '';
    relationBinding = '';
    relationKind = 'database';
    relationResourceId = workspace?.resources[0]?.id ?? '';
    relationRequired = true;
    relationAppId = '';
  }

  function openWizard(mode: WizardMode, app?: Application) {
    resetWizard();
    wizardOpen = true;
    wizardMode = mode;
    wizardProjectId = selectedProjectId;
    if (mode === 'application') wizardStep = 1;
    if (mode === 'application' && app) {
      wizardApplicationId = app.id;
      wizardStep = 2;
      relationAppId = app.id;
    }
    if (mode === 'dependency') {
      wizardStep = 3;
      relationAppId = app?.id ?? '';
      relationResourceId = workspace?.resources[0]?.id ?? '';
    }
  }

  function closeWizard() {
    if (!wizardSaving) resetWizard();
  }

  async function ensureProjectCreated() {
    if (wizardProjectId) return true;
    if (!projectTeamId || !projectName.trim()) return false;
    const created = await api.createProject(projectTeamId, {
      name: projectName.trim(),
      code: projectCode.trim(),
      icon: projectIcon,
      labels: { description: projectDescription.trim() },
      source: projectSource
    });
    wizardProjectId = created.id;
    onProjectCreated(created);
    onSelectProject(created);
    await loadWorkspace(created.id);
    onNotice(`项目“${created.name}”已创建`);
    return true;
  }

  async function ensureApplicationCreated() {
    if (wizardApplicationId) return true;
    const projectId = wizardProjectId || selectedProjectId;
    if (!projectId) return false;
    if (wizardMode === 'project' && projectSource === 'kubernetes') {
      const candidates = kubernetesCandidates.filter((candidate) => candidate.selected);
      if (!candidates.length) return true;
      for (const candidate of candidates) {
        const created = await api.createApplication(projectId, {
          name: candidate.name,
          code: candidate.name.toLowerCase().replace(/[^a-z0-9-]+/g, '-'),
          description: `从 ${kubernetesCluster || 'Kubernetes'} / ${kubernetesNamespace} 导入`,
          icon: 'lucide:AppWindow',
          source: 'kubernetes',
          labels: { source: 'kubernetes', selector: kubernetesSelector }
        });
        if (!wizardApplicationId) wizardApplicationId = created.id;
      }
      await loadWorkspace(projectId);
      onNotice(`已导入 ${candidates.length} 个 Kubernetes 应用候选`);
      return true;
    }
    if (!applicationName.trim() || !applicationCode.trim()) return false;
    const created = await api.createApplication(projectId, {
      name: applicationName.trim(),
      code: applicationCode.trim(),
      description: applicationDescription.trim(),
      icon: applicationIcon,
      source: applicationSource,
      labels: { source: applicationSource }
    });
    wizardApplicationId = created.id;
    await loadWorkspace(projectId);
    onNotice('应用已添加，继续配置实例和依赖');
    return true;
  }

  async function saveWizardInstance() {
    if (wizardInstanceSaved || !wizardApplicationId) return true;
    if (applicationSource === 'runtime' && !runtimeResourceId) return false;
    if (applicationSource !== 'runtime') return true;
    const selector = relationBinding.trim() ? JSON.parse(relationBinding) : {};
    await api.createApplicationInstance(wizardProjectId || selectedProjectId, wizardApplicationId, {
      name: relationName.trim() || 'default',
      runtime_kind: runtimeKind,
      target_resource_id: runtimeResourceId,
      selector
    });
    wizardInstanceSaved = true;
    await loadWorkspace(wizardProjectId || selectedProjectId);
    return true;
  }

  async function saveWizardDependency() {
    if (wizardDependencySaved) return true;
    if (!relationAppId || !relationResourceId) return false;
    const binding = relationBinding.trim() ? JSON.parse(relationBinding) : {};
    await api.createApplicationDependency(wizardProjectId || selectedProjectId, relationAppId, {
      target_resource_id: relationResourceId,
      dependency_kind: relationKind,
      binding,
      required: relationRequired
    });
    wizardDependencySaved = true;
    await loadWorkspace(wizardProjectId || selectedProjectId);
    return true;
  }

  async function nextWizard() {
    wizardSaving = true;
    onError('');
    try {
      if (wizardStep === 0) {
        if (!(await ensureProjectCreated())) return;
        wizardStep = 1;
      } else if (wizardStep === 1) {
        if (!(await ensureApplicationCreated())) return;
        wizardStep = 2;
      } else if (wizardStep === 2) {
        if (!(await saveWizardInstance())) return;
        wizardStep = 3;
      } else {
        if (wizardMode === 'dependency') {
          if (!(await saveWizardDependency())) {
            onError('请选择应用和关联资源后再完成依赖配置');
            return;
          }
        } else if (relationAppId) {
          if (!(await saveWizardDependency())) {
            onError('请选择依赖资源后再完成配置');
            return;
          }
        }
        const projectId = wizardProjectId || selectedProjectId;
        if (projectId) await loadWorkspace(projectId);
        onNotice(wizardMode === 'dependency' ? '应用依赖已添加' : '配置已完成');
        resetWizard();
      }
    } catch (error) {
      onError(describeError(error, '保存向导配置失败，请检查资源定位配置'));
    } finally {
      wizardSaving = false;
    }
  }

  function skipWizardStep() {
    if (wizardStep === 3) {
      resetWizard();
      return;
    }
    wizardStep += 1;
  }

  function toggleCandidate(index: number) {
    kubernetesCandidates = kubernetesCandidates.map((candidate, candidateIndex) => candidateIndex === index ? { ...candidate, selected: !candidate.selected } : candidate);
  }

  function toggle(app: Application) {
    const next = new Set(expanded);
    next.has(app.id) ? next.delete(app.id) : next.add(app.id);
    expanded = next;
  }

  function beginEdit(app: Application) {
    editingAppId = app.id;
    editName = app.name;
    editDescription = app.description;
    editIcon = app.icon;
  }

  async function saveEdit(app: Application) {
    try {
      await api.updateApplication(selectedProjectId, app.id, { name: editName, description: editDescription, icon: editIcon });
      editingAppId = '';
      await loadWorkspace(selectedProjectId);
      onNotice('应用已更新');
    } catch (error) {
      onError(describeError(error, '更新应用失败'));
    }
  }

  function importApplication(event: Event) {
    const file = (event.target as HTMLInputElement).files?.[0];
    if (!file || !selectedProject) return;
    const reader = new FileReader();
    reader.onload = async () => {
      try {
        await api.importApplication(selectedProject.id, JSON.parse(String(reader.result)));
        await loadWorkspace(selectedProject.id);
        onNotice('应用已导入');
      } catch (error) {
        onError(describeError(error, '导入应用失败'));
      }
    };
    reader.readAsText(file);
    (event.target as HTMLInputElement).value = '';
  }

</script>

<section class="resource-map-page">
  {#if !selectedProject}
    <header class="resource-map-header">
      <div class="project-page-heading">
        <span class="project-page-heading-icon" aria-hidden="true"><Map size={19} /></span>
        <div>
          <h2><strong>项目全景图</strong><small>PROJECT OVERVIEW</small></h2>
          <p>按团队查看项目分布，进入项目查看运行状态与资源关系。</p>
        </div>
      </div>
      <div class="resource-map-actions">
        <button class="primary" type="button" on:click={() => openWizard('project')}><Plus size={15} />新增项目</button>
      </div>
    </header>

    <nav class="team-filter" aria-label="团队筛选">
      <ScopeChip label="全部团队" count={allProjects.length} active={allTeamsSelected} on:click={onSelectAllTeams} />
      {#each teams as team}
        <ScopeChip label={team.name} count={allProjects.filter((project) => project.team_id === team.id).length} active={selectedScopeId === team.scope.id} on:click={() => onSelectTeam(team)} />
      {/each}
      <button class="secondary team-filter-add" type="button" disabled={busy} on:click={onOpenTeamDialog}><Plus size={14} />添加团队</button>
    </nav>

    <div class="team-project-groups">
      {#each groupedProjects as group}
        {#if group.projects.length || !selectedScopeId}
          {@const teamSelected = selectedScopeId === group.team.scope.id}
          {@const teamCollapsed = collapsedTeams.has(group.team.id) && !teamSelected}
          <section class="team-project-group">
            <button class="group-heading group-heading-toggle" type="button" aria-expanded={!teamCollapsed} on:click={() => toggleTeam(group.team.id)}>
              <span class="group-title"><EntityBrandIcon kind="Team" fallback={group.team.icon || 'lucide:UsersRound'} size={16} className="team-mark" /><span><h2>{group.team.name}</h2><small>{group.team.description || '团队项目'}</small></span></span>
              <span class="group-heading-end"><span class="group-count">{group.projects.length} 个项目</span><span class:rotate={teamCollapsed} class="team-collapse-icon" aria-hidden="true"><ChevronDown size={16} /></span></span>
            </button>
            {#if !teamCollapsed}
              <div class="project-card-grid">
                {#each group.projects as project}
                  <button class="project-map-card" type="button" on:click={() => onSelectProject(project)}>
                    <span class="project-card-top"><EntityBrandIcon kind="Project" fallback={project.icon || 'lucide:FolderKanban'} size={20} className="project-card-icon" /><span class="project-card-identity"><strong>{project.name}</strong><small>{project.code}</small></span><span class="project-card-arrow"><ArrowRight size={17} /></span></span>
                    <span class="project-card-meta"><span>{sourceLabel(project.source)}</span><span class="status-label {project.status}">{statusLabel(project.status)}</span></span>
                    <span class="project-card-footer" aria-label="项目运行摘要"><span class="project-card-fact"><strong>{project.summary.applications}</strong><small>应用</small></span><span class="project-card-fact"><strong>{project.summary.resources}</strong><small>资源</small></span><span class="project-card-fact"><strong>{project.summary.alerts}</strong><small>告警</small></span><span class="project-card-fact"><strong>{project.summary.dependencies}</strong><small>依赖</small></span></span>
                  </button>
                {:else}
                  <div class="map-empty compact"><Network size={20} /><span>该团队还没有项目</span><button type="button" class="text-button" on:click={() => openWizard('project')}>新增项目</button></div>
                {/each}
              </div>
            {/if}
          </section>
        {/if}
      {:else}
        <div class="map-empty"><Network size={28} /><h2>暂无可见项目</h2><p>创建第一个项目，开始构建项目全景图。</p><button class="primary" type="button" on:click={() => openWizard('project')}><Plus size={15} />新增项目</button></div>
      {/each}
    </div>
  {:else}
    <header class="workspace-header resource-map-header">
      <div class="workspace-title"><div class="project-heading"><button class="project-heading-back" type="button" data-tooltip="返回项目全景图" aria-label="返回项目全景图" on:click={() => onSelectTeam(teams.find((team) => team.id === selectedProject?.team_id) ?? teams[0])}><ArrowLeft size={18} /></button><div><h1><strong>项目驾驶舱</strong><small class="project-heading-english">PROJECT COCKPIT</small></h1><p class="project-heading-context">{teamName(selectedProject.team_id)} / {selectedProject.name} · {selectedProject.code}</p></div></div></div>
      <div class="workspace-actions">
        <input bind:this={importInput} type="file" accept="application/json" hidden on:change={importApplication} />
        <button class="primary workspace-action" type="button" on:click={() => importInput?.click()}><Upload size={15} />导入应用</button>
        <button class="primary workspace-action" type="button" on:click={() => openWizard('project')}><Plus size={15} />新增项目</button>
        <button class="primary workspace-action" type="button" on:click={() => openWizard('application')}><Plus size={15} />添加应用</button>
        <button class="primary workspace-action" type="button" on:click={() => openWizard('dependency')}><Plus size={15} />添加依赖</button>
        <button class="diagnose" type="button" data-tooltip="进入 AI 诊断" on:click={() => onOpenDiagnosis(selectedProject)}><Sparkles size={15} />AI 诊断</button>
      </div>
    </header>

    <div class="workspace-stats">
      {#each [['应用', workspace?.summary.applications ?? 0, '应用和服务'], ['实例', workspace?.summary.instances ?? 0, '运行中的实例'], ['关联资源', workspace?.summary.resources ?? 0, 'Host / Docker / K8s'], ['依赖', workspace?.summary.dependencies ?? 0, '应用关系'], ['告警', workspace?.summary.alerts ?? 0, '待处理信号']] as stat}
        <div class="workspace-stat"><span>{stat[0]}</span><strong>{stat[1]}</strong><small>{stat[2]}</small></div>
      {/each}
    </div>

    <div class="resource-map-columns">
      <section class="map-main-column">
        <div class="section-heading"><div><h2>应用与实例</h2><p>应用运行状态、实例绑定和应用级资源关系</p></div><span class="muted">{workspace?.applications.length ?? 0} 个应用</span></div>
        {#if loadingWorkspace}
          <div class="panel map-loading">正在加载项目驾驶舱...</div>
        {:else if workspace?.applications.length}
          {#each workspace.applications as app}
            <article class="application-map-card panel" class:expanded={expanded.has(app.id)}>
              <div class="application-card-head">
                <button class="application-toggle" type="button" on:click={() => toggle(app)}><EntityBrandIcon kind="Application" fallback={app.icon || 'lucide:AppWindow'} size={19} className="application-icon" /><span class="application-title"><strong>{app.name}</strong><small>{app.code} · {app.description || '未填写描述'}</small></span></button>
                <div class="application-card-meta"><span>{app.instances.length} 实例</span><span>{app.dependencies.length} 依赖</span>{#if app.instances.length === 0}<span class="unbound-mark" data-tooltip="未关联运行资源" aria-label="未关联运行资源">!</span>{/if}<span class="status-label {app.status}">{statusLabel(app.status)}</span><button class="icon-button" type="button" data-tooltip="AI 诊断" aria-label="AI 诊断" on:click|stopPropagation={() => onOpenApplicationDiagnosis(selectedProject, app)}><Sparkles size={14} /></button><button class="icon-button" type="button" data-tooltip="编辑应用" aria-label="编辑应用" on:click|stopPropagation={() => beginEdit(app)}><Pencil size={14} /></button><button class="icon-button" type="button" data-tooltip="展开详情" aria-label="展开详情" on:click={() => toggle(app)}><span class:rotate={expanded.has(app.id)}><ChevronDown size={18} /></span></button></div>
              </div>
              {#if editingAppId === app.id}
                <form class="inline-edit" on:submit|preventDefault={() => saveEdit(app)}><IconPicker value={editIcon} onSelect={(icon) => (editIcon = icon)} ariaLabel="选择应用图标" /><input bind:value={editName} required /><input bind:value={editDescription} placeholder="描述" /><button class="primary" type="submit">保存</button><button class="secondary" type="button" on:click={() => editingAppId = ''}>取消</button></form>
              {/if}
              {#if expanded.has(app.id)}
                <div class="application-detail-grid">
                  <section class="detail-block"><div class="detail-heading"><h3>运行实例</h3><button class="text-button" type="button" on:click={() => { relationAppId = app.id; openWizard('application', app); }}><Plus size={13} />添加实例</button></div>{#each app.instances as instance}<div class="instance-row"><span class="runtime-badge">{instance.runtime_kind === 'cloud_native' ? 'K8s' : instance.runtime_kind === 'containerized' ? 'Docker' : 'Host'}</span><span><strong>{instance.name}</strong><small>{instance.target_resource_name || instance.target_resource_id}</small></span><span class="status-label {instance.status}">{statusLabel(instance.status)}</span></div>{:else}<div class="detail-empty"><AlertTriangle size={14} />未绑定运行资源</div>{/each}</section>
                  <section class="detail-block"><div class="detail-heading"><h3>关联依赖</h3><button class="text-button" type="button" on:click={() => openWizard('dependency', app)}><Plus size={13} />添加依赖</button></div>{#each app.dependencies as dependency}<div class="instance-row"><span class="dependency-badge">{dependency.dependency_kind}</span><span><strong>{dependency.target_resource_name || dependency.target_resource_id}</strong><small>{dependency.target_resource_kind || '关联资源'}</small></span><span class="status-label {dependency.status}">{statusLabel(dependency.status)}</span></div>{:else}<div class="detail-empty">暂无应用依赖</div>{/each}</section>
                  <section class="topology-block"><div class="detail-heading"><div><h3><Network size={15} />应用资源拓扑</h3><p>应用与实例、依赖资源之间的连接</p></div><span class="muted">{app.instances.length + app.dependencies.length} 个连接</span></div><div class="topology-flow"><span class="topology-node app-node"><EntityBrandIcon kind="Application" fallback={app.icon || 'lucide:AppWindow'} size={14} className="topology-icon" />{app.name}</span>{#each [...app.instances, ...app.dependencies] as edge}<span class="topology-line"></span><span class="topology-node">{edge.target_resource_name || edge.target_resource_id}</span>{/each}</div></section>
                </div>
              {/if}
            </article>
          {/each}
        {:else}
          <div class="map-empty panel"><Network size={28} /><h3>还没有应用</h3><p>添加应用或从 Kubernetes 导入应用，开始完善项目驾驶舱。</p><button class="primary" type="button" on:click={() => openWizard('application')}><Plus size={15} />添加应用</button></div>
        {/if}
      </section>

      <aside class="map-side-column">
        <section class="panel map-side-panel"><div class="panel-heading"><div><h2>运行资源</h2><p>项目关联的 Host、Docker 和 Kubernetes 资源</p></div><Server size={17} /></div>{#each workspace?.resources ?? [] as resource}<div class="resource-map-row"><span class="resource-kind">{resource.kind}</span><span><strong>{resource.name}</strong><small>{resource.role || '运行资源'}</small></span><span class="status-label {resource.status}">{statusLabel(resource.status)}</span></div>{:else}<div class="detail-empty">暂无关联运行资源</div>{/each}</section>
        <section class="panel topology-panel"><div class="panel-heading"><div><h2>运行拓扑</h2><p>项目与运行资源之间的连接概览</p></div><Network size={17} /></div><div class="project-topology"><div class="topology-node project-node"><EntityBrandIcon kind="Project" fallback={selectedProject.icon || 'lucide:FolderKanban'} size={15} className="topology-icon" />{selectedProject.name}</div>{#each (workspace?.resources ?? []).slice(0, 5) as resource}<span class="topology-branch"></span><div class="topology-node">{resource.name}</div>{/each}</div><p class="topology-caption">展开应用后可查看实例和依赖的详细连接。</p></section>
        <section class="panel map-side-panel alerts-panel"><div class="panel-heading"><div><h2>项目告警</h2><p>需要关注的项目健康信号</p></div><AlertTriangle size={17} /></div>{#each workspace?.alerts ?? [] as alert}<div class="alert-map-row"><span class="severity {alert.severity}"></span><span><strong>{alert.title}</strong><small>{statusLabel(alert.status)}</small></span></div>{:else}<div class="detail-empty success-empty"><CheckCircle2 size={15} />当前没有项目告警</div>{/each}</section>
      </aside>
    </div>
  {/if}
</section>

{#if wizardOpen}
  <div class="wizard-backdrop" role="presentation" on:click={(event) => event.target === event.currentTarget && closeWizard()}>
    <div class="project-wizard" role="dialog" aria-modal="true" aria-labelledby="project-wizard-title" tabindex="-1">
      <header class="wizard-header"><div><p class="eyebrow">PROJECT CONFIGURATION WIZARD</p><h2 id="project-wizard-title">{wizardMode === 'project' ? '新增项目' : wizardMode === 'application' ? '添加应用' : '添加应用依赖'}</h2><p>{wizardMode === 'project' ? '从项目到应用、实例和依赖关系，一次完成项目驾驶舱配置。' : '沿用项目驾驶舱配置流，随时跳过暂不需要的步骤。'}</p></div><button class="icon-button" type="button" data-tooltip="关闭向导" aria-label="关闭向导" on:click={closeWizard}><X size={18} /></button></header>
      <nav class="wizard-steps" aria-label="配置步骤">{#each ['项目来源', '应用清单', '实例绑定', '依赖关系'] as step, index}<button class:active={wizardStep === index} class:done={wizardStep > index} type="button" on:click={() => index <= wizardStep && (wizardStep = index)}><span>{index + 1}</span>{step}</button>{/each}</nav>
      <div class="wizard-content">
        {#if wizardStep === 0}
          <div class="wizard-pane">
            <div class="wizard-pane-heading"><div><p class="eyebrow">STEP 01</p><h3>项目从哪里开始？</h3><p>选择手动创建或关联 Kubernetes，应用和依赖可以稍后配置。</p></div></div>
            <div class="wizard-choice-grid"><button class:selected={projectSource === 'manual'} type="button" on:click={() => (projectSource = 'manual')}><span class="choice-symbol">✎</span><span><strong>手动新增</strong><small>填写项目基本信息，应用和依赖可以稍后配置。</small></span></button><button class:selected={projectSource === 'kubernetes'} type="button" on:click={() => (projectSource = 'kubernetes')}><span class="choice-symbol">◌</span><span><strong>关联 Kubernetes</strong><small>先预览候选应用，删减后再真正导入项目。</small></span></button></div>
            <div class="wizard-form-grid"><label>所属团队<i class="required-mark">*</i><select bind:value={projectTeamId} required><option value="" disabled>选择团队</option>{#each teams as team}<option value={team.id}>{team.name}</option>{/each}</select></label><label>项目名称<i class="required-mark">*</i><input bind:value={projectName} placeholder="支付结算平台" required /></label><label>项目编码<input bind:value={projectCode} placeholder="留空自动生成" /></label><label>项目图标<span class="icon-control"><IconPicker value={projectIcon} onSelect={(icon) => (projectIcon = icon)} ariaLabel="选择项目图标" /></span></label><label class="full-field">项目说明<textarea bind:value={projectDescription} rows="3" placeholder="项目职责与边界"></textarea></label></div>
            {#if projectSource === 'kubernetes'}
              <div class="wizard-form-grid"><label>集群定位<input bind:value={kubernetesCluster} placeholder="cluster-prod-01" required /></label><label>命名空间<input bind:value={kubernetesNamespace} placeholder="payments" required /></label><label class="full-field">标签选择器<input bind:value={kubernetesSelector} placeholder="app.kubernetes.io/part-of=payments" required /></label></div>
              <div class="candidate-preview"><div class="candidate-heading"><span><strong>候选应用预览</strong><small>取消勾选即可在导入前删减</small></span><span>{kubernetesCandidates.filter((candidate) => candidate.selected).length} / {kubernetesCandidates.length}</span></div>{#each kubernetesCandidates as candidate, index}<label class="candidate-row"><input type="checkbox" checked={candidate.selected} on:change={() => toggleCandidate(index)} /><span>{candidate.name}</span><small>Deployment</small></label>{/each}</div>
            {/if}
          </div>
        {:else if wizardStep === 1}
          <div class="wizard-pane"><div class="wizard-pane-heading"><div><p class="eyebrow">STEP 02</p><h3>应用清单</h3><p>{wizardMode === 'project' && projectSource === 'kubernetes' ? '确认 Kubernetes 候选应用后再导入。' : '可以先添加一个应用，也可以跳过后在驾驶舱继续配置。'}</p></div></div>{#if wizardMode === 'project' && projectSource === 'kubernetes'}<div class="import-review"><CheckCircle2 size={18} /><div><strong>{kubernetesCandidates.filter((candidate) => candidate.selected).length} 个候选应用待导入</strong><p>{kubernetesCluster || 'Kubernetes 集群'} / {kubernetesNamespace} · {kubernetesSelector}</p></div></div>{:else}<div class="wizard-form-grid"><label>应用名称<i class="required-mark">*</i><input bind:value={applicationName} placeholder="payments-api" required /></label><label>应用编码<i class="required-mark">*</i><input bind:value={applicationCode} pattern="[a-z0-9][a-z0-9-]*" placeholder="payments-api" required /></label><label>应用图标<span class="icon-control"><IconPicker value={applicationIcon} onSelect={(icon) => (applicationIcon = icon)} ariaLabel="选择应用图标" /></span></label><label>添加方式<div class="segmented-control"><button class:active={applicationSource === 'manual'} type="button" on:click={() => (applicationSource = 'manual')}>手动</button><button class:active={applicationSource === 'runtime'} type="button" on:click={() => (applicationSource = 'runtime')}>关联运行资源</button></div></label><label class="full-field">应用说明<textarea bind:value={applicationDescription} rows="3" placeholder="应用职责与边界"></textarea></label></div>{/if}</div>
        {:else if wizardStep === 2}
          <div class="wizard-pane"><div class="wizard-pane-heading"><div><p class="eyebrow">STEP 03</p><h3>实例绑定</h3><p>已关联资源的实例会自动显示健康状态；手动应用可以保留为待配置。</p></div></div>{#if wizardMode === 'dependency'}<div class="skip-pane"><Network size={22} /><strong>这是依赖配置向导</strong><p>实例绑定对当前动作不是必需步骤，可以直接跳过。</p></div>{:else if wizardMode === 'project' && projectSource === 'kubernetes'}<div class="skip-pane"><CheckCircle2 size={22} /><strong>Kubernetes 实例自动发现</strong><p>导入后由集群标签和命名空间定位实例，此步骤无需手动绑定。</p></div>{:else}<div class="runtime-options"><button class:selected={applicationSource === 'manual'} type="button" on:click={() => (applicationSource = 'manual')}><span>!</span><strong>暂不关联资源</strong><small>驾驶舱会保留待配置警示</small></button><button class:selected={applicationSource === 'runtime'} type="button" on:click={() => (applicationSource = 'runtime')}><span>↗</span><strong>绑定运行资源</strong><small>Host、Docker、Kubernetes 均需唯一定位</small></button></div>{#if applicationSource === 'runtime'}<div class="wizard-form-grid"><label>运行方式<select bind:value={runtimeKind}><option value="virtual_machine">Host</option><option value="containerized">Docker</option><option value="cloud_native">Kubernetes</option></select></label><label>实例名称<input bind:value={relationName} placeholder="default" /></label><label class="full-field">唯一资源<select bind:value={runtimeResourceId} required><option value="" disabled>选择运行资源</option>{#each workspace?.resources ?? [] as resource}<option value={resource.id}>{resource.name} · {resource.kind}</option>{/each}</select>{#if !selectedRuntimeResource()}<small class="field-error">必须选择一个可唯一定位的运行资源</small>{/if}</label><label class="full-field">定位字段 JSON<textarea bind:value={relationBinding} rows="3" placeholder="例如 namespace=payments"></textarea></label></div>{/if}{/if}</div>
        {:else}
          <div class="wizard-pane"><div class="wizard-pane-heading"><div><p class="eyebrow">STEP 04</p><h3>依赖关系</h3><p>把数据库、消息队列、缓存等资源挂到应用上，依赖也可以稍后补充。</p></div></div>{#if wizardMode === 'project' && !wizardApplicationId}<div class="skip-pane"><Network size={22} /><strong>还没有应用可绑定依赖</strong><p>完成项目后可以从驾驶舱的应用详情中继续添加。</p></div>{:else}<div class="wizard-form-grid"><label>应用<select bind:value={relationAppId} required><option value="" disabled>选择应用</option>{#each workspace?.applications ?? [] as app}<option value={app.id}>{app.name} · {app.code}</option>{/each}{#if wizardApplicationId && !(workspace?.applications.some((app) => app.id === wizardApplicationId) ?? false)}<option value={wizardApplicationId}>当前新增应用</option>{/if}</select></label><label>依赖类型<select bind:value={relationKind}><option value="database">数据库</option><option value="messaging">消息队列</option><option value="cache">缓存</option><option value="configuration">配置中心</option><option value="repository">代码仓库</option><option value="storage">存储</option></select></label><label class="full-field">关联资源<select bind:value={relationResourceId} required><option value="" disabled>选择资源</option>{#each workspace?.resources ?? [] as resource}<option value={resource.id}>{resource.name} · {resource.kind}</option>{/each}</select></label><label class="full-field">绑定字段 JSON<textarea bind:value={relationBinding} rows="3" placeholder="例如 database=payments"></textarea></label><label class="checkbox-field"><input type="checkbox" bind:checked={relationRequired} />必须可用</label></div>{/if}</div>
        {/if}
      </div>
      <footer class="wizard-footer"><button class="secondary" type="button" on:click={closeWizard}>取消</button><div class="wizard-footer-right">{#if wizardStep > (wizardMode === 'project' ? 0 : wizardMode === 'application' ? 1 : 3)}<button class="secondary" type="button" on:click={() => (wizardStep -= 1)}><ArrowLeft size={14} />上一步</button>{/if}<button class="secondary" type="button" on:click={skipWizardStep}>{wizardStep === 3 ? '跳过并完成' : '跳过此步'}</button><button class="primary" type="button" disabled={wizardSaving} on:click={nextWizard}>{wizardStep === 3 ? '完成配置' : '下一步'}<ArrowRight size={14} /></button></div></footer>
    </div>
  </div>
{/if}
