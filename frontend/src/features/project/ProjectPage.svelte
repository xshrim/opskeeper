<script lang="ts">
  import {
    AlertTriangle,
    ArrowLeft,
    ArrowRight,
    CheckCircle2,
    ChevronDown,
    Clock3,
    Map,
    Network,
    Pencil,
    Plus,
    Search,
    Server,
    Sparkles
  } from 'lucide-svelte';
  import {
    api,
    ApiError,
    type Application,
    type Project,
    type ProjectWorkspace,
    type Resource,
    type Team
  } from '../../lib/api';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import FormField from '../../components/FormField.svelte';
  import IconPicker from '../../components/IconPicker.svelte';
  import SearchInput from '../../components/SearchInput.svelte';
  import ScopeChip from '../../components/ScopeChip.svelte';
  import TextInput from '../../components/TextInput.svelte';
  import ApplicationWizard from './ApplicationWizard.svelte';
  import ProjectCreateForm from './ProjectCreateForm.svelte';
  import { runtimeLabel, type ApplicationDraft } from './projectTypes';

  export let teams: Team[] = [];
  export let allProjects: Project[] = [];
  export let visibleProjects: Project[] = [];
  export let availableResources: Resource[] = [];
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
  export let onOpenApplicationDiagnosis: (
    project: Project,
    application: Application
  ) => void = () => {};

  let workspace: ProjectWorkspace | null = null;
  let workspaceProjectId = '';
  let loadingWorkspace = false;
  let expanded = new Set<string>();
  let collapsedTeams = new Set<string>();
  let initializedTeamKey = '';
  let projectSearch = '';
  let projectCreateOpen = false;
  let applicationWizardOpen = false;
  let editingAppId = '';
  let editName = '';
  let editDescription = '';
  let editIcon = 'lucide:AppWindow';

  $: selectedProject =
    visibleProjects.find((project) => project.id === selectedProjectId) ??
    allProjects.find((project) => project.id === selectedProjectId) ??
    null;
  $: normalizedProjectSearch = projectSearch.trim().toLocaleLowerCase();
  $: filteredProjects = normalizedProjectSearch
    ? visibleProjects.filter((project) =>
        [project.name, project.code].some((value) =>
          value.toLocaleLowerCase().includes(normalizedProjectSearch)
        )
      )
    : visibleProjects;
  $: groupedProjects = teams.map((team) => ({
    team,
    projects: filteredProjects.filter((project) => project.team_id === team.id)
  }));
  $: teamKey = teams.map((team) => team.id).join(',');
  $: if (teamKey !== initializedTeamKey) {
    initializedTeamKey = teamKey;
    collapsedTeams = new Set(teams.slice(3).map((team) => team.id));
  }
  $: if (selectedProjectId && selectedProjectId !== workspaceProjectId)
    void loadWorkspace(selectedProjectId);
  $: if (!selectedProjectId) {
    workspace = null;
    workspaceProjectId = '';
    expanded = new Set<string>();
  }

  function describeError(error: unknown, fallback: string) {
    return error instanceof ApiError
      ? error.message || fallback
      : error instanceof Error
        ? error.message || fallback
        : fallback;
  }
  function teamName(id: string) {
    return teams.find((team) => team.id === id)?.name ?? '未分配团队';
  }
  function statusLabel(status: string) {
    return ['active', 'healthy', 'up', 'normal'].includes(status)
      ? '正常'
      : status === 'disabled'
        ? '已停用'
        : status === 'warning'
          ? '需关注'
          : '未知';
  }
  function projectStatus(project: Project) {
    return project.status === 'active' && project.summary.alerts === 0
      ? '正常'
      : project.status === 'disabled'
        ? '已停用'
        : '异常';
  }
  function projectHealthTone(project: Project) {
    return projectStatus(project) === '正常'
      ? 'success'
      : project.status === 'disabled'
        ? 'disabled'
        : 'warning';
  }
  function projectHealthSummary(project: Project) {
    if (project.summary.alerts) return `${project.summary.alerts} 条告警待处理`;
    if (project.status === 'disabled') return '暂无可用运行数据';
    if (project.status === 'warning') return '请检查项目健康信号';
    if (project.status === 'unknown') return '等待健康检查结果';
    return '暂无告警';
  }
  function projectUpdatedLabel(value: string) {
    const timestamp = Date.parse(value);
    if (!Number.isFinite(timestamp)) return '更新时间未知';
    const minutes = Math.floor(Math.max(Date.now() - timestamp, 0) / 60000);
    return minutes < 1
      ? '刚刚更新'
      : minutes < 60
        ? `${minutes} 分钟前更新`
        : `${Math.floor(minutes / 60)} 小时前更新`;
  }
  function projectTags(project: Project) {
    return Object.entries(project.labels ?? {})
      .filter(
        ([key, value]) =>
          key !== 'description' && !key.startsWith('opskeeper.') && value.trim()
      )
      .map(([, value]) => value.trim())
      .slice(0, 3);
  }
  function toggleTeam(id: string) {
    const next = new Set(collapsedTeams);
    next.has(id) ? next.delete(id) : next.add(id);
    collapsedTeams = next;
  }
  function toggle(app: Application) {
    const next = new Set(expanded);
    next.has(app.id) ? next.delete(app.id) : next.add(app.id);
  }
  async function loadWorkspace(id: string) {
    loadingWorkspace = true;
    try {
      workspace = await api.projectWorkspace(id);
      workspaceProjectId = id;
    } catch (error) {
      onError(describeError(error, '项目驾驶舱加载失败'));
    } finally {
      loadingWorkspace = false;
    }
  }
  function applicationPayload(draft: ApplicationDraft) {
    return {
      name: draft.name,
      code: draft.code,
      description: draft.description,
      icon: draft.icon,
      runtime_kind: draft.runtimeKind,
      external_uid: draft.externalUid,
      instances: draft.instances.map((instance) => ({
        name: instance.name,
        runtime_kind: draft.runtimeKind,
        target_resource_id: instance.targetResourceId,
        selector: instance.selector
      }))
    };
  }
  async function saveProject(input: {
    teamId: string;
    name: string;
    code: string;
    icon: string;
    description: string;
    labels: Record<string, string>;
    applications: ApplicationDraft[];
  }) {
    const project = await api.createProject(input.teamId, {
      name: input.name,
      code: input.code,
      description: input.description,
      icon: input.icon,
      labels: input.labels,
      applications: input.applications.map(applicationPayload)
    });
    projectCreateOpen = false;
    onProjectCreated(project);
    onSelectProject(project);
    onNotice(`项目“${project.name}”已创建`);
  }
  async function saveApplications(drafts: ApplicationDraft[]) {
    if (!selectedProjectId) return;
    for (const draft of drafts)
      await api.createApplication(selectedProjectId, applicationPayload(draft));
    applicationWizardOpen = false;
    await loadWorkspace(selectedProjectId);
    onNotice(`${drafts.length} 个应用已添加`);
  }
  function beginEdit(app: Application) {
    editingAppId = app.id;
    editName = app.name;
    editDescription = app.description;
    editIcon = app.icon;
  }
  async function saveEdit(app: Application) {
    try {
      await api.updateApplication(selectedProjectId, app.id, {
        name: editName,
        description: editDescription,
        icon: editIcon
      });
      editingAppId = '';
      await loadWorkspace(selectedProjectId);
      onNotice('应用已更新');
    } catch (error) {
      onError(describeError(error, '更新应用失败'));
    }
  }
</script>

<section class="resource-map-page">
  {#if !selectedProject}
    <header class="resource-map-header project-overview-header">
      <div class="project-page-heading">
        <span class="project-page-heading-icon" aria-hidden="true"
          ><Map size={19} /></span
        >
        <div>
          <h2><strong>项目全景图</strong><small>PROJECT OVERVIEW</small></h2>
          <p>按团队查看项目分布，进入项目查看运行状态与资源关系。</p>
        </div>
      </div>
      <div class="resource-map-actions">
        <SearchInput
          bind:value={projectSearch}
          className="project-overview-search"
          placeholder="搜索项目名称或编号"
          ariaLabel="搜索项目名称或编号"
        /><button
          class="primary"
          type="button"
          on:click={() => (projectCreateOpen = true)}
          ><Plus size={15} />新增项目</button
        >
      </div>
    </header>
    <nav class="team-filter" aria-label="团队筛选">
      <ScopeChip
        label="全部团队"
        count={allProjects.length}
        active={allTeamsSelected}
        on:click={onSelectAllTeams}
      />{#each teams as team}<ScopeChip
          label={team.name}
          count={allProjects.filter((project) => project.team_id === team.id)
            .length}
          active={selectedScopeId === team.scope.id}
          on:click={() => onSelectTeam(team)}
        />{/each}<button
        class="secondary team-filter-add"
        type="button"
        disabled={busy}
        on:click={onOpenTeamDialog}><Plus size={14} />添加团队</button
      >
    </nav>
    {#if normalizedProjectSearch && filteredProjects.length === 0}<div
        class="map-empty project-search-empty"
      >
        <Search size={28} />
        <h2>没有匹配项目</h2>
        <p>未找到名称或编号包含“{projectSearch.trim()}”的项目。</p>
        <button
          class="secondary"
          type="button"
          on:click={() => (projectSearch = '')}>清除搜索</button
        >
      </div>{:else}<div class="team-project-groups">
        {#each groupedProjects as group}{#if group.projects.length || (!selectedScopeId && !normalizedProjectSearch)}{@const teamSelected =
              selectedScopeId === group.team.scope.id}{@const teamCollapsed =
              !normalizedProjectSearch &&
              collapsedTeams.has(group.team.id) &&
              !teamSelected}
            <section class="team-project-group">
              <button
                class="group-heading group-heading-toggle"
                type="button"
                aria-expanded={!teamCollapsed}
                on:click={() => toggleTeam(group.team.id)}
                ><span class="group-title"
                  ><EntityBrandIcon
                    kind="Team"
                    fallback={group.team.icon || 'lucide:UsersRound'}
                    size={16}
                    className="team-mark"
                  /><span
                    ><h2>{group.team.name}</h2>
                    <small>{group.team.description || '团队项目'}</small></span
                  ></span
                ><span class="group-heading-end"
                  ><span class="group-count"
                    >{group.projects.length} 个项目</span
                  ><span
                    class:rotate={teamCollapsed}
                    class="team-collapse-icon"
                    aria-hidden="true"><ChevronDown size={16} /></span
                  ></span
                ></button
              >{#if !teamCollapsed}<div class="project-card-grid">
                  {#each group.projects as project}<button
                      class="project-map-card"
                      type="button"
                      on:click={() => onSelectProject(project)}
                      ><span class="project-card-top"
                        ><EntityBrandIcon
                          kind="Project"
                          fallback={project.icon || 'lucide:FolderKanban'}
                          size={20}
                          className="project-card-icon"
                        /><span class="project-card-identity"
                          ><strong>{project.name}</strong><small
                            >{project.code}</small
                          ></span
                        ><span class="project-card-arrow"
                          ><ArrowRight size={17} /></span
                        ></span
                      ><span class="project-card-middle"
                        ><span class="project-card-primary"
                          ><span class="project-card-application-health"
                            ><strong
                              >{project.summary.applications_healthy} / {project
                                .summary.applications}</strong
                            ><small>正常应用</small></span
                          ><span class="project-card-tags" aria-label="项目标签"
                            >{#each projectTags(project) as tag}<span
                                class="project-card-tag">{tag}</span
                              >{/each}</span
                          ></span
                        ><span
                          class="project-card-secondary-metrics"
                          aria-label="项目运行统计"
                          ><span
                            ><small>关联资源</small><strong
                              >{project.summary.resources}</strong
                            ></span
                          ><span
                            ><small>服务依赖</small><strong
                              >{project.summary.dependencies}</strong
                            ></span
                          ><span
                            ><small>项目告警</small><strong
                              class:warning={project.summary.alerts > 0}
                              >{project.summary.alerts}</strong
                            ></span
                          ></span
                        ></span
                      ><span class="project-card-footer"
                        ><span
                          class="project-card-health-summary"
                          data-tooltip={projectHealthSummary(project)}
                          ><span
                            class="status-label {projectHealthTone(project)}"
                            >{projectStatus(project)}</span
                          ><span
                            class="project-health-separator"
                            aria-hidden="true">·</span
                          ><span class="project-health-copy"
                            >{projectHealthSummary(project)}</span
                          ></span
                        ><span class="project-card-updated"
                          ><Clock3 size={12} />{projectUpdatedLabel(
                            project.updated_at
                          )}</span
                        ></span
                      ></button
                    >{:else}<div class="map-empty compact">
                      <Network size={20} /><span>该团队还没有项目</span><button
                        type="button"
                        class="text-button"
                        on:click={() => (projectCreateOpen = true)}
                        >新增项目</button
                      >
                    </div>{/each}
                </div>{/if}
            </section>{/if}{/each}
      </div>{/if}
  {:else}
    <header class="workspace-header resource-map-header">
      <div class="workspace-title">
        <div class="project-heading">
          <button
            class="project-heading-back"
            type="button"
            data-tooltip="返回项目全景图"
            aria-label="返回项目全景图"
            on:click={onSelectAllTeams}><ArrowLeft size={18} /></button
          >
          <div>
            <h1>
              <strong>项目驾驶舱</strong><small class="project-heading-english"
                >PROJECT COCKPIT</small
              >
            </h1>
            <p class="project-heading-context">
              {teamName(selectedProject.team_id)} / {selectedProject.name} · {selectedProject.code}
            </p>
          </div>
        </div>
      </div>
      <div class="workspace-actions">
        <button
          class="primary workspace-action"
          type="button"
          on:click={() => (projectCreateOpen = true)}
          ><Plus size={15} />新增项目</button
        ><button
          class="primary workspace-action"
          type="button"
          on:click={() => (applicationWizardOpen = true)}
          ><Plus size={15} />添加应用</button
        ><button
          class="diagnose"
          type="button"
          data-tooltip="进入 AI 诊断"
          on:click={() => onOpenDiagnosis(selectedProject)}
          ><Sparkles size={15} />AI 诊断</button
        >
      </div>
    </header>
    <div class="workspace-stats">
      {#each [['应用', workspace?.summary.applications ?? 0, '应用和服务'], ['实例', workspace?.summary.instances ?? 0, '运行中的实例'], ['关联资源', workspace?.summary.resources ?? 0, 'Host / Docker / K8s'], ['服务依赖', workspace?.summary.dependencies ?? 0, '暂未配置'], ['告警', workspace?.summary.alerts ?? 0, '待处理信号']] as stat}<div
          class="workspace-stat"
        >
          <span>{stat[0]}</span><strong>{stat[1]}</strong><small
            >{stat[2]}</small
          >
        </div>{/each}
    </div>
    <div class="resource-map-columns">
      <section class="map-main-column">
        <div class="section-heading">
          <div>
            <h2>项目应用</h2>
            <p>应用状态、实例绑定和运行资源</p>
          </div>
          <span class="muted">{workspace?.applications.length ?? 0} 个应用</span
          >
        </div>
        {#if loadingWorkspace}<div class="panel map-loading">
            正在加载项目驾驶舱...
          </div>{:else if workspace?.applications.length}{#each workspace.applications as app}<article
              class="application-map-card panel"
              class:expanded={expanded.has(app.id)}
            >
              <div class="application-card-head">
                <button
                  class="application-toggle"
                  type="button"
                  on:click={() => toggle(app)}
                  ><EntityBrandIcon
                    kind="Application"
                    fallback={app.icon || 'lucide:AppWindow'}
                    size={19}
                    className="application-icon"
                  /><span class="application-title"
                    ><strong>{app.name}</strong><small
                      >{app.code} · {runtimeLabel(
                        app.runtime_kind as
                          'virtual_machine' | 'containerized' | 'cloud_native'
                      )}</small
                    ></span
                  ></button
                >
                <div class="application-card-meta">
                  <span>{app.instances.length} 实例</span><span
                    class="status-label {app.status}"
                    >{statusLabel(app.status)}</span
                  ><button
                    class="icon-button"
                    type="button"
                    data-tooltip="AI 诊断"
                    aria-label="AI 诊断"
                    on:click|stopPropagation={() =>
                      onOpenApplicationDiagnosis(selectedProject, app)}
                    ><Sparkles size={14} /></button
                  ><button
                    class="icon-button"
                    type="button"
                    data-tooltip="编辑应用"
                    aria-label="编辑应用"
                    on:click|stopPropagation={() => beginEdit(app)}
                    ><Pencil size={14} /></button
                  ><button
                    class="icon-button"
                    type="button"
                    data-tooltip="展开详情"
                    aria-label="展开详情"
                    on:click={() => toggle(app)}
                    ><span class:rotate={expanded.has(app.id)}
                      ><ChevronDown size={18} /></span
                    ></button
                  >
                </div>
              </div>
              {#if editingAppId === app.id}<form
                  class="inline-edit"
                  on:submit|preventDefault={() => saveEdit(app)}
                >
                  <IconPicker
                    value={editIcon}
                    onSelect={(value) => (editIcon = value)}
                    ariaLabel="选择应用图标"
                  /><FormField label="应用名称" required className="inline-edit-field"><TextInput bind:value={editName} required ariaLabel="应用名称" /></FormField><FormField label="应用描述" className="inline-edit-field"><TextInput bind:value={editDescription} placeholder="描述" ariaLabel="应用描述" /></FormField><button class="primary" type="submit">保存</button><button
                    class="secondary"
                    type="button"
                    on:click={() => (editingAppId = '')}>取消</button
                  >
                </form>{/if}{#if expanded.has(app.id)}<div
                  class="application-detail-grid"
                >
                  <section class="detail-block">
                    <div class="detail-heading"><h3>运行实例</h3></div>
                    {#each app.instances as instance}<div class="instance-row">
                        <span class="runtime-badge"
                          >{runtimeLabel(
                            instance.runtime_kind as
                              | 'virtual_machine'
                              | 'containerized'
                              | 'cloud_native'
                          )}</span
                        ><span
                          ><strong>{instance.name}</strong><small
                            >{instance.target_resource_name ||
                              instance.target_resource_id}</small
                          ></span
                        ><span class="status-label {instance.status}"
                          >{statusLabel(instance.status)}</span
                        >
                      </div>{:else}<div class="detail-empty">
                        <AlertTriangle size={14} />未绑定运行资源
                      </div>{/each}
                  </section>
                  <section class="detail-block">
                    <div class="detail-heading"><h3>服务依赖</h3></div>
                    <div class="detail-empty">当前阶段暂不配置服务依赖</div>
                  </section>
                </div>{/if}
            </article>{/each}{:else}<div class="map-empty panel">
            <Network size={28} />
            <h3>还没有应用</h3>
            <p>使用应用向导添加 Host、Docker 或 Kubernetes 应用。</p>
            <button
              class="primary"
              type="button"
              on:click={() => (applicationWizardOpen = true)}
              ><Plus size={15} />添加应用</button
            >
          </div>{/if}
      </section>
      <aside class="map-side-column">
        <section class="panel map-side-panel">
          <div class="panel-heading">
            <div>
              <h2>运行资源</h2>
              <p>项目关联的 Host、Docker 和 Kubernetes 资源</p>
            </div>
            <Server size={17} />
          </div>
          {#each workspace?.resources ?? [] as resource}<div
              class="resource-map-row"
            >
              <span class="resource-kind">{resource.kind}</span><span
                ><strong>{resource.name}</strong><small
                  >{resource.role || '运行资源'}</small
                ></span
              ><span class="status-label {resource.status}"
                >{statusLabel(resource.status)}</span
              >
            </div>{:else}<div class="detail-empty">暂无关联运行资源</div>{/each}
        </section>
        <section class="panel topology-panel">
          <div class="panel-heading">
            <div>
              <h2>运行拓扑</h2>
              <p>项目与运行资源之间的连接概览</p>
            </div>
            <Network size={17} />
          </div>
          <div class="project-topology">
            <div class="topology-node project-node">
              <EntityBrandIcon
                kind="Project"
                fallback={selectedProject.icon || 'lucide:FolderKanban'}
                size={15}
                className="topology-icon"
              />{selectedProject.name}
            </div>
            {#each (workspace?.resources ?? []).slice(0, 5) as resource}<span
                class="topology-branch"
              ></span>
              <div class="topology-node">{resource.name}</div>{/each}
          </div>
        </section>
        <section class="panel map-side-panel alerts-panel">
          <div class="panel-heading">
            <div>
              <h2>项目告警</h2>
              <p>需要关注的项目健康信号</p>
            </div>
            <AlertTriangle size={17} />
          </div>
          {#each workspace?.alerts ?? [] as alert}<div class="alert-map-row">
              <span class="severity {alert.severity}"></span><span
                ><strong>{alert.title}</strong><small
                  >{statusLabel(alert.status)}</small
                ></span
              >
            </div>{:else}<div class="detail-empty success-empty">
              <CheckCircle2 size={15} />当前没有项目告警
            </div>{/each}
        </section>
      </aside>
    </div>
  {/if}
</section>

{#if projectCreateOpen}<ProjectCreateForm
    {teams}
    resources={availableResources}
    defaultTeamId={teams.find((team) => team.scope.id === selectedScopeId)
      ?.id ??
      teams[0]?.id ??
      ''}
    onSave={saveProject}
    onCancel={() => (projectCreateOpen = false)}
  />{/if}
{#if applicationWizardOpen}<ApplicationWizard
    resources={availableResources}
    onSave={saveApplications}
    onCancel={() => (applicationWizardOpen = false)}
  />{/if}
