<script lang="ts">
  import {
    AlertTriangle,
    ArrowLeft,
    ArrowRight,
    CheckCircle2,
    ChevronDown,
    ChevronUp,
    Clock3,
    Map,
    Network,
    Pencil,
    Plus,
    RefreshCw,
    Search,
    Server,
    Sparkles,
    Trash2
  } from 'lucide-svelte';
  import {
    api,
    type Application,
    type Project,
    type ProjectWorkspace,
    type Resource,
    type Team,
    type User
  } from '../../lib/api';
  import { errorMessage as formatErrorMessage } from '../../lib/errorMessage';
import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
import StatusBadge from '../../components/StatusBadge.svelte';
import ExpandableTable from '../../components/ExpandableTable.svelte';
import ReadonlyTable from '../../components/ReadonlyTable.svelte';
import type { ReadonlyTableColumn } from '../../components/ReadonlyTable.svelte';
import SearchInput from '../../components/SearchInput.svelte';
import ScopeChip from '../../components/ScopeChip.svelte';
  import DeleteConfirmationDialog from '../../components/DeleteConfirmationDialog.svelte';
import { labelTags } from '../../lib/labels';
  import TagList from '../../components/TagList.svelte';
  import ApplicationWizard from './ApplicationWizard.svelte';
  import ProjectCreateForm from './ProjectCreateForm.svelte';
import {
  runtimeLabel,
  type ApplicationDraft,
  type ApplicationResourceCreationRequest,
  type ApplicationResourceCreationResult,
  type RuntimeResourceKind
} from './projectTypes';
  import { onMount, tick } from 'svelte';

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
  export let onProjectUpdated: (project: Project) => void = () => {};
  export let onProjectDeleted: (project: Project) => void = () => {};
  export let canManageProject: (
    project: Project,
    action: 'update' | 'delete'
  ) => boolean = () => false;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;
  export let onOpenDiagnosis: (project: Project) => void;
  export let onOpenApplicationDiagnosis: (
    project: Project,
    application: Application
  ) => void = () => {};
  export let onResourceCreated: (resource: Resource) => void = () => {};
  export let onRequestResourceCreate: (
    request: Omit<ApplicationResourceCreationRequest, 'id'>
  ) => void = () => {};
  export let resourceCreationResult: ApplicationResourceCreationResult | null = null;

  let workspace: ProjectWorkspace | null = null;
  let workspaceProjectId = '';
  let loadingWorkspace = false;
  let collapsedTeams = new Set<string>();
  let initializedTeamKey = '';
  let teamFilterElement: HTMLElement;
  let teamFilterMeasureElement: HTMLElement;
  let teamFilterExpanded = false;
  let visibleTeamCount = 0;
  let teamFilterHasOverflow = false;
  let measuredTeamFilterKey = '';
  let projectSearch = '';
  let projectCreateOpen = false;
  let editingProject: Project | null = null;
  let applicationWizardOpen = false;
  let editingApplication: Application | null = null;
  let deletingProject: Project | null = null;
  let deletingApplications: Application[] = [];
  let selectedApplicationIds: string[] = [];
  let users: User[] = [];

  const applicationColumns: ReadonlyTableColumn[] = [
    { key: 'application', label: '应用', width: '22%' },
    { key: 'runtime', label: '运行时', width: '13%' },
    { key: 'instances', label: '实例', width: '10%' },
    { key: 'resources', label: '资源', width: '10%' },
    { key: 'labels', label: '标签', width: '19%' },
    { key: 'status', label: '状态', width: '10%' },
    { key: 'actions', label: '操作', width: '12%', className: 'application-actions-column' }
  ];
  const applicationInstanceColumns: ReadonlyTableColumn[] = [
    { key: 'name', label: '实例', width: '22%' },
    { key: 'runtime', label: '运行时', width: '14%' },
    { key: 'resource', label: '运行资源', width: '24%' },
    { key: 'selector', label: '定位条件', width: '28%' },
    { key: 'status', label: '状态', width: '12%' }
  ];

  onMount(() => {
    const resizeObserver = new ResizeObserver(() => void measureTeamFilter());
    if (teamFilterElement) resizeObserver.observe(teamFilterElement);
    void api.users().then((result) => (users = result)).catch(() => (users = []));
    return () => resizeObserver.disconnect();
  });
  $: ownerOptions = users
    .filter((user) => user.status === 'active')
    .map((user) => ({
      value: user.id,
      label: user.display_name || user.username,
      description: user.display_name ? user.username : user.email || user.phone
    }));

  $: selectedProject =
    visibleProjects.find((project) => project.id === selectedProjectId) ??
    allProjects.find((project) => project.id === selectedProjectId) ??
    null;
  $: selectedApplications = workspace?.applications.filter((application) => selectedApplicationIds.includes(application.id)) ?? [];
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
  $: teamFilterKey = `${teamKey}:${allProjects.map((project) => `${project.team_id}:${project.id}`).join(',')}:${teams.map((team) => team.name).join('|')}:${teamFilterExpanded}`;
  $: if (teamFilterKey !== measuredTeamFilterKey) {
    measuredTeamFilterKey = teamFilterKey;
    void measureTeamFilter();
  }
  $: if (teamKey !== initializedTeamKey) {
    initializedTeamKey = teamKey;
    collapsedTeams = new Set(teams.slice(3).map((team) => team.id));
  }
  $: if (selectedProjectId && selectedProjectId !== workspaceProjectId) {
    selectedApplicationIds = [];
    void loadWorkspace(selectedProjectId);
  }
  $: if (!selectedProjectId) {
    workspace = null;
    workspaceProjectId = '';
    selectedApplicationIds = [];
  }

  async function measureTeamFilter() {
    await tick();
    if (!teamFilterElement || !teamFilterMeasureElement) return;
    if (teamFilterExpanded) {
      visibleTeamCount = teams.length;
      return;
    }
    const allChip = teamFilterElement.querySelector<HTMLElement>('.scope-chip');
    const addButton = teamFilterElement.querySelector<HTMLElement>('.team-filter-add');
    const teamChips = Array.from(teamFilterMeasureElement.querySelectorAll<HTMLElement>('.scope-chip'));
    const moreProbe = teamFilterMeasureElement.querySelector<HTMLElement>('.team-filter-more-probe');
    if (!allChip || !addButton || !moreProbe) return;
    const gap = Number.parseFloat(getComputedStyle(teamFilterElement).columnGap) || 0;
    const fixedWidth = allChip.getBoundingClientRect().width + addButton.getBoundingClientRect().width;
    const availableWidth = teamFilterElement.clientWidth;
    for (let count = teams.length; count >= 0; count -= 1) {
      const hiddenCount = teams.length - count;
      const moreWidth = hiddenCount ? moreProbe.getBoundingClientRect().width : 0;
      const itemCount = count + 2 + (hiddenCount ? 1 : 0);
      const teamsWidth = teamChips.slice(0, count).reduce((sum, chip) => sum + chip.getBoundingClientRect().width, 0);
      if (fixedWidth + teamsWidth + moreWidth + gap * Math.max(itemCount - 1, 0) <= availableWidth) {
        visibleTeamCount = count;
        teamFilterHasOverflow = hiddenCount > 0;
        return;
      }
    }
    visibleTeamCount = 0;
    teamFilterHasOverflow = teams.length > 0;
  }

  function describeError(error: unknown, fallback: string) {
    return formatErrorMessage(error, fallback);
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
    return labelTags(project.labels);
  }
  function toggleTeam(id: string) {
    const next = new Set(collapsedTeams);
    next.has(id) ? next.delete(id) : next.add(id);
    collapsedTeams = next;
  }
  function applicationInstanceRows(application: Application) {
    return application.instances.map((instance) => ({
      name: instance.name,
      runtime: runtimeLabel(instance.runtime_kind as 'virtual_machine' | 'containerized' | 'cloud_native'),
      resource: instance.target_resource_name || instance.target_resource_id,
      selector: Object.entries(instance.selector)
        .filter(([, value]) => typeof value === 'string' && value)
        .map(([key, value]) => key + '=' + value)
        .join(' · ') || '—',
      status: statusLabel(instance.status),
      tone: instance.status
    }));
  }

  function applicationRuntimeResourceNames(application: Application) {
    if (!application.instances.length) return '未知';
    return [...new Set(application.instances
      .map((instance) => instance.target_resource_name || instance.target_resource_id)
      .filter(Boolean))].join('、') || '—';
  }

  function applicationResourceCount(application: Application) {
    return new Set([
      ...application.instances.map((instance) => instance.target_resource_id),
      ...(application.dependencies ?? []).map((dependency) => dependency.target_resource_id)
    ].filter(Boolean)).size;
  }

  function ownerLabel(ids: string[]) {
    const names = ids.map((id) => ownerOptions.find((option) => option.value === id)?.label || id);
    return names.length ? names.join('、') : '未配置';
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
      labels: draft.labels,
      runtime_kind: draft.runtimeKind,
      external_uid: draft.externalUid,
      critical: draft.critical,
      technical_owner_ids: draft.technicalOwnerIds,
      operations_owner_ids: draft.operationsOwnerIds,
      instances: draft.instances.map((instance) => ({
        name: instance.name,
        runtime_kind: draft.runtimeKind,
        target_resource_id: instance.targetResourceId,
        selector: instance.selector,
        log_binding: instance.logBinding,
        status: instance.status
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
    importanceLevel: 'ITP1' | 'ITP2' | 'ITP3' | 'ITP4';
    technicalOwnerIds: string[];
    operationsOwnerIds: string[];
    applications: ApplicationDraft[];
  }) {
    if (editingProject) {
      const project = await api.updateProject(editingProject.id, {
        name: input.name,
        description: input.description,
        icon: input.icon,
        labels: input.labels,
        importance_level: input.importanceLevel,
        technical_owner_ids: input.technicalOwnerIds,
        operations_owner_ids: input.operationsOwnerIds
      });
      editingProject = null;
      onProjectUpdated(project);
      onNotice(`项目“${project.name}”已更新`);
      return;
    }
    const project = await api.createProject(input.teamId, {
      name: input.name,
      code: input.code,
      description: input.description,
      icon: input.icon,
      labels: input.labels,
      importance_level: input.importanceLevel,
      technical_owner_ids: input.technicalOwnerIds,
      operations_owner_ids: input.operationsOwnerIds,
      applications: input.applications.map(applicationPayload)
    });
    projectCreateOpen = false;
    onProjectCreated(project);
    onSelectProject(project);
    onNotice(`项目“${project.name}”已创建`);
  }
  async function saveApplications(drafts: ApplicationDraft[]) {
    if (!selectedProjectId) return;
    await api.createApplications(selectedProjectId, drafts.map(applicationPayload));
    applicationWizardOpen = false;
    await loadWorkspace(selectedProjectId);
    onNotice(`${drafts.length} 个应用已新增`);
  }
  async function updateApplication(app: Application, draft: ApplicationDraft) {
    try {
      await api.updateApplication(selectedProjectId, app.id, {
        name: draft.name,
        code: draft.code,
        description: draft.description,
        icon: draft.icon,
        runtime_kind: draft.runtimeKind,
        external_uid: draft.externalUid,
        labels: draft.labels,
        critical: draft.critical,
        technical_owner_ids: draft.technicalOwnerIds,
        operations_owner_ids: draft.operationsOwnerIds,
        instances: applicationPayload(draft).instances
      });
      editingApplication = null;
      await loadWorkspace(selectedProjectId);
      onNotice('应用已更新');
    } catch (error) {
      throw new Error(describeError(error, '更新应用失败'));
    }
  }
  async function confirmProjectDelete() {
    if (!deletingProject) return;
    const project = deletingProject;
    try {
      await api.deleteProject(project.id, project.name);
      deletingProject = null;
      onProjectDeleted(project);
      onSelectAllTeams();
      onNotice(`项目“${project.name}”及其应用和范围配置已删除；关联资源已保留至团队范围`);
    } catch (error) {
      onError(describeError(error, '删除项目失败'));
    }
  }
  async function confirmApplicationDelete() {
    if (!deletingApplications.length || !selectedProjectId) return;
    try {
      await Promise.all(deletingApplications.map((application) => api.deleteApplication(selectedProjectId, application.id)));
      const deletedCount = deletingApplications.length;
      deletingApplications = [];
      selectedApplicationIds = [];
      await loadWorkspace(selectedProjectId);
      onNotice(String(deletedCount) + ' 个应用已删除，关联资源未受影响');
    } catch (error) {
      onError(describeError(error, '删除应用失败'));
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
          width="160px"
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
    <nav class:expanded={teamFilterExpanded} class="team-filter" aria-label="团队筛选" bind:this={teamFilterElement}>
      <ScopeChip
        label="全部团队"
        count={allProjects.length}
        active={allTeamsSelected}
        on:click={onSelectAllTeams}
      />{#each (teamFilterExpanded ? teams : teams.slice(0, visibleTeamCount)) as team}<ScopeChip
          label={team.name}
          count={allProjects.filter((project) => project.team_id === team.id)
            .length}
          active={selectedScopeId === team.scope.id}
          on:click={() => onSelectTeam(team)}
        />{/each}{#if teamFilterHasOverflow}<button
        class="team-filter-expand"
        type="button"
        aria-label={teamFilterExpanded ? '收起团队列表' : `展开其余 ${teams.length - visibleTeamCount} 个团队`}
        aria-expanded={teamFilterExpanded}
        data-tooltip={teamFilterExpanded ? '收起团队' : `展开其余 ${teams.length - visibleTeamCount} 个团队`}
        on:click={() => (teamFilterExpanded = !teamFilterExpanded)}
      >{#if teamFilterExpanded}<ChevronUp size={14} />{:else}<span>+{teams.length - visibleTeamCount}</span><ChevronDown size={14} />{/if}</button>{/if}<button
        class="secondary team-filter-add"
        type="button"
        disabled={busy}
        on:click={onOpenTeamDialog}><Plus size={14} />新增团队</button
      >
      <div class="team-filter-measure" aria-hidden="true" inert bind:this={teamFilterMeasureElement}>
        {#each teams as team}<ScopeChip
            label={team.name}
            count={allProjects.filter((project) => project.team_id === team.id).length}
          />{/each}<button class="team-filter-expand team-filter-more-probe" type="button"><span>+{teams.length}</span><ChevronDown size={14} /></button>
      </div>
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
                  {#each group.projects as project}<article class="project-map-card" role="group" aria-label={project.name}>
                        <button class="project-card-open" type="button" aria-label={`打开项目 ${project.name}`} on:click={() => onSelectProject(project)}></button>
                        <div class="project-card-top">
                          <EntityBrandIcon kind="Project" fallback={project.icon || 'lucide:FolderKanban'} size={20} className="project-card-icon" />
                          <div class="project-card-identity">
                            <div class="project-card-title">
                              <strong>{project.name}</strong>
                              <span class={`project-importance-tag ${project.importance_level.toLowerCase()}`}>{project.importance_level}</span>
                            </div>
                            <small>{project.code}</small>
                          </div>
                          <span class="project-card-arrow"><ArrowRight size={17} /></span>
                        </div>
                        <div class="project-card-middle">
                          <div class="project-card-primary">
                            <div class="project-card-application-health">
                              <strong>{project.summary.applications_healthy} / {project.summary.applications}</strong>
                              <small>正常应用</small>
                            </div>
                            <TagList tags={projectTags(project)} className="project-card-tags" ariaLabel="项目标签" distinguishKeyOnly={false} fit wrap={false} />
                          </div>
                          <div class="project-card-secondary-metrics" aria-label="项目运行统计">
                            <span><small>关联资源</small><strong>{project.summary.resources}</strong></span>
                            <span><small>服务依赖</small><strong>{project.summary.dependencies}</strong></span>
                            <span><small>项目告警</small><strong class:warning={project.summary.alerts > 0}>{project.summary.alerts}</strong></span>
                          </div>
                        </div>
                        <div class="project-card-footer">
                          <span class="project-card-health-summary">
                            <StatusBadge className="status-label" tone={projectHealthTone(project)} label={projectStatus(project)} />
                            <span class="project-health-separator" aria-hidden="true">·</span>
                            <span class="project-card-updated"><Clock3 size={12} />{projectUpdatedLabel(project.updated_at)}</span>
                          </span>
                          <span class="project-card-actions">
                            {#if canManageProject(project, 'update')}<button class="icon-button edit-action" type="button" data-tooltip="编辑项目" aria-label={`编辑项目 ${project.name}`} on:click={() => (editingProject = project)}><Pencil size={14} aria-hidden="true" /></button>{/if}
                            {#if canManageProject(project, 'delete')}<button class="icon-button danger-action delete-action" type="button" data-tooltip="删除项目" aria-label={`删除项目 ${project.name}`} on:click={() => (deletingProject = project)}><Trash2 size={14} aria-hidden="true" /></button>{/if}
                          </span>
                        </div>
                      </article>{:else}<div class="map-empty compact">
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
          class="secondary"
          type="button"
          disabled={loadingWorkspace}
          data-tooltip="刷新项目驾驶舱"
          aria-label="刷新项目驾驶舱"
          on:click={() => selectedProject && void loadWorkspace(selectedProject.id)}
          ><RefreshCw size={15} />刷新</button
        >
        <button
          class="danger-button workspace-action"
          type="button"
          on:click={() => (deletingProject = selectedProject)}
          ><Trash2 size={15} />删除项目</button
        ><button
          class="primary workspace-action"
          type="button"
          on:click={() => (projectCreateOpen = true)}
          ><Plus size={15} />新增项目</button
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
        <section class="panel project-applications-panel">
        <div class="section-heading">
          <div>
            <h2>项目应用</h2>
            <p>应用状态、实例绑定和运行资源</p>
          </div>
          <div class="section-heading-actions">
            <span class="muted">{workspace?.applications.length ?? 0} 个应用</span><button
              class="secondary"
              type="button"
              disabled={loadingWorkspace}
              data-tooltip="刷新项目应用"
              aria-label="刷新项目应用"
              on:click={() => selectedProject && void loadWorkspace(selectedProject.id)}
              ><RefreshCw size={15} />刷新</button
            ><button
              class="danger-button workspace-action"
              type="button"
              disabled={!selectedApplications.length}
              data-tooltip={selectedApplications.length ? `删除所选 ${selectedApplications.length} 个应用` : '请先选择应用'}
              on:click={() => (deletingApplications = selectedApplications)}
              ><Trash2 size={15} />删除应用</button
            ><button class="primary workspace-action" type="button" on:click={() => (applicationWizardOpen = true)}><Plus size={15} />新增应用</button>
          </div>
        </div>
        {#if loadingWorkspace}<div class="map-loading">正在加载项目驾驶舱...</div>
        {:else if workspace?.applications.length}
          <ExpandableTable
            columns={applicationColumns}
            rows={workspace.applications}
            rowKey={(application) => application.id}
            expandable
            selectable
            selectedIds={selectedApplicationIds}
            onSelectionChange={(ids) => (selectedApplicationIds = ids)}
            emptyText="当前项目暂无应用"
          >
            <svelte:fragment slot="cell" let:row let:column>
              {#if column.key === 'application'}
                <span class="application-table-name"><EntityBrandIcon kind="Application" fallback={row.icon || 'lucide:AppWindow'} size={17} /><span><strong>{row.name}</strong><small>{row.code}</small></span></span>
              {:else if column.key === 'runtime'}
                <span class="application-table-value" data-tooltip={applicationRuntimeResourceNames(row)}><strong>{applicationRuntimeResourceNames(row)}</strong><small>{row.instances.length ? row.runtime_kind === 'virtual_machine' ? '虚拟机' : row.runtime_kind === 'containerized' ? '容器化' : '云原生' : '未绑定运行时'}</small></span>
              {:else if column.key === 'instances'}<span class="application-table-count">{row.instances.length} 个实例</span>
              {:else if column.key === 'resources'}<span class="application-table-count">{applicationResourceCount(row)} 个资源</span>
              {:else if column.key === 'labels'}<TagList className="application-table-tags" tags={labelTags(row.labels)} fit wrap={false} colorScheme="independent" maxWidth="130px" emptyText="—" />
              {:else if column.key === 'status'}<StatusBadge className="status-label" tone={row.status} label={statusLabel(row.status)} />
              {:else if column.key === 'actions'}
                <span class="application-table-actions">
                  <button class="icon-button edit-action" type="button" data-tooltip="AI 诊断" aria-label="AI 诊断" on:click|stopPropagation={() => onOpenApplicationDiagnosis(selectedProject, row)}><Sparkles size={14} /></button>
                  <button class="icon-button edit-action" type="button" data-tooltip="编辑应用" aria-label="编辑应用" on:click|stopPropagation={() => (editingApplication = row)}><Pencil size={15} /></button>
                  <button class="icon-button danger-action delete-action" type="button" data-tooltip="删除应用" aria-label={`删除应用 ${row.name}`} on:click|stopPropagation={() => (deletingApplications = [row])}><Trash2 size={14} /></button>
                </span>
              {/if}
            </svelte:fragment>
            <svelte:fragment slot="details" let:row>
              <div class="application-detail-grid">
                <div class="application-detail-fields">
                  <div><small>应用名称</small><strong>{row.name}</strong></div>
                  <div><small>应用标识</small><strong>{row.code}</strong></div>
                  <div><small>运行时</small><strong>{row.instances.length ? runtimeLabel(row.runtime_kind) : '未知'}</strong></div>
                  <div><small>关键应用</small><strong>{row.critical ? '是' : '否'}</strong></div>
                  <div><small>技术负责人</small><strong>{ownerLabel(row.technical_owner_ids)}</strong></div>
                  <div><small>运维负责人</small><strong>{ownerLabel(row.operations_owner_ids)}</strong></div>
                  <div><small>标签</small><TagList tags={labelTags(row.labels)} /></div>
                  <div><small>描述</small><strong>{row.description || '—'}</strong></div>
                </div>
                <ReadonlyTable columns={applicationInstanceColumns} rows={applicationInstanceRows(row)} emptyText="未绑定运行实例">
                  <svelte:fragment slot="cell" let:row={instance} let:column>
                    {#if column.key === 'status'}<StatusBadge className="status-label" tone={instance.tone} label={instance.status} />{:else}{instance[column.key]}{/if}
                  </svelte:fragment>
                </ReadonlyTable>
              </div>
            </svelte:fragment>
          </ExpandableTable>
        {:else}<div class="map-empty">
            <Network size={28} />
            <h3>还没有应用</h3>
            <p>使用应用向导新增 Host、Docker 或 Kubernetes 应用。</p>
            <button
              class="primary"
              type="button"
              on:click={() => (applicationWizardOpen = true)}
              ><Plus size={15} />新增应用</button
            >
          </div>{/if}
        </section>
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

{#if projectCreateOpen || editingProject}<ProjectCreateForm
    {teams}
    resources={availableResources}
    {users}
    initialProject={editingProject}
    defaultTeamId={teams.find((team) => team.scope.id === selectedScopeId)
      ?.id ??
      teams[0]?.id ??
      ''}
    onSave={saveProject}
    onCancel={() => { projectCreateOpen = false; editingProject = null; }}
  />{/if}
{#if applicationWizardOpen || editingApplication}<ApplicationWizard
    resources={availableResources}
    {users}
    initialApplication={editingApplication}
    onSave={saveApplications}
    onUpdate={updateApplication}
    {onResourceCreated}
    onRequestResourceCreate={(kind: RuntimeResourceKind, instanceId?: string) =>
      onRequestResourceCreate({
        kind,
        scopeId: selectedProject?.scope.id ?? selectedScopeId,
        ...(instanceId ? { instanceId } : {})
      })}
    {resourceCreationResult}
    onCancel={() => { applicationWizardOpen = false; editingApplication = null; }}
  />{/if}
{#if deletingProject}
  <DeleteConfirmationDialog
    title={`删除项目“${deletingProject.name}”？`}
    name={deletingProject.name}
    description="将删除该项目的应用、实例、授权和范围配置。用户账号、审计记录及关联资源不会被删除；资源将保留至所属团队范围。"
    impact="此操作不可撤销。"
    {busy}
    onCancel={() => (deletingProject = null)}
    onConfirm={confirmProjectDelete}
  />
{/if}
{#if deletingApplications.length}
  <DeleteConfirmationDialog
    title={`删除 ${deletingApplications.length} 个应用？`}
    name={deletingApplications.map((application) => application.name).join('、')}
    description="将删除所选应用及其实例和资源绑定。关联资源、项目、用户和历史审计记录不会被删除。"
    impact="此操作不可撤销。"
    {busy}
    onCancel={() => (deletingApplications = [])}
    onConfirm={confirmApplicationDelete}
  />
{/if}
