<script lang="ts">
  import {
    AppWindow,
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
    StickyNote,
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
import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
import StatusBadge from '../../components/StatusBadge.svelte';
import ExpandableTable from '../../components/ExpandableTable.svelte';
import ReadonlyTable from '../../components/ReadonlyTable.svelte';
import type { ReadonlyTableColumn } from '../../components/ReadonlyTable.svelte';
import SearchInput from '../../components/SearchInput.svelte';
import ScopeChip from '../../components/ScopeChip.svelte';
import ScopeLevelBadge from '../../components/ScopeLevelBadge.svelte';
  import DeleteConfirmationDialog from '../../components/DeleteConfirmationDialog.svelte';
import { labelTags } from '../../lib/labels';
import { resourceCategoryFor, resourceCategoryIcon } from '../resource/resourceCatalog';
  import TagList from '../../components/TagList.svelte';
import ApplicationWizard from './ApplicationWizard.svelte';
import TopologyApplicationCreate from './TopologyApplicationCreate.svelte';
import ProjectTopologyView from './ProjectTopology.svelte';
  import ProjectCreateForm from './ProjectCreateForm.svelte';
import {
  runtimeLabel,
  type ApplicationDraft,
  type ApplicationRuntimeKind,
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
    request: ApplicationResourceCreationRequest
  ) => void = () => {};
  export let resourceCreationResult: ApplicationResourceCreationResult | null = null;
  export let resourceCreationCancelledId = '';
  export let projectAreaTab: 'applications' | 'topology' = 'applications';

  let workspace: ProjectWorkspace | null = null;
  let projectTopology: import('../../lib/api').ProjectTopology | null = null;
  let loadingTopology = false;
  let topologyMode: 'browse' | 'edit' = 'browse';
  let topologyResourceRequestId = '';
  type TopologyComponentIntent = { kind: 'application' | 'resource' | 'annotation'; id?: string; resourceKind?: string; runtimeKind?: string; position?: { x: number; y: number } };
  let topologyComponentIntent: TopologyComponentIntent | null = null;
  let topologyApplicationId = '';
  let topologyResourceId = '';
  let topologyAnnotationName = '';
  let topologyAnnotationDescription = '';
  let topologyConfigBusy = false;
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
  let topologyApplicationCreateOpen = false;
  let topologyEditApplication: Application | null = null;
  let editingApplication: Application | null = null;
  let deletingProject: Project | null = null;
  let deletingApplications: Application[] = [];
  let selectedApplicationIds: string[] = [];
  let inspectedApplicationID = '';
  let inspectedResourceIDs: string[] | null = null;
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
  $: inspectedResourceIDSet = inspectedResourceIDs ? new Set(inspectedResourceIDs) : null;
  $: visibleWorkspaceResources = (workspace?.resources ?? []).filter((resource) => !inspectedResourceIDSet || inspectedResourceIDSet.has(resource.id));
  $: visibleWorkspaceAlerts = (workspace?.alerts ?? []).filter((alert) => !inspectedResourceIDSet || inspectedResourceIDSet.has(alert.target_resource_id));
  $: topologyResourceOptions = topologyComponentIntent?.kind === 'resource'
    ? availableResources.filter((resource) => resourceCategoryFor(resource) === topologyComponentIntent?.resourceKind)
    : availableResources;
  $: topologyResourceScopeTypes = Object.fromEntries((workspace?.resources ?? []).map((resource) => [resource.id, resource.scope_type]));
  $: applicationAlertCounts = Object.fromEntries((workspace?.applications ?? []).map((application) => {
    const resourceIDs = new Set(applicationResourceIDs(application));
    return [application.id, (workspace?.alerts ?? []).filter((alert) => resourceIDs.has(alert.target_resource_id)).length];
  }));
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
    inspectedApplicationID = '';
    inspectedResourceIDs = null;
    topologyMode = 'browse';
    void loadWorkspace(selectedProjectId);
  }
  $: if (!selectedProjectId) {
    workspace = null;
    projectTopology = null;
    workspaceProjectId = '';
    selectedApplicationIds = [];
  }
  $: if (topologyMode === 'browse' || projectAreaTab !== 'topology') topologyComponentIntent = null;

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

  function applicationResourceIDs(application: Application) {
    return [...new Set([
      ...application.instances.map((instance) => instance.target_resource_id),
      ...(application.dependencies ?? []).map((dependency) => dependency.target_resource_id)
    ].filter(Boolean))];
  }

  function inspectApplication(application: Application) {
    inspectedApplicationID = application.id;
    inspectedResourceIDs = applicationResourceIDs(application);
  }

  function inspectTopologySelection(selection: { node: import('../../lib/api').ProjectTopology['nodes'][number] | null; edge: import('../../lib/api').ProjectTopology['edges'][number] | null }) {
    const appIDs = new Set<string>();
    const resourceIDs = new Set<string>();
    const addNode = (id: string) => {
      const node = projectTopology?.nodes.find((item) => item.id === id);
      if (node?.kind === 'application') appIDs.add(node.id);
      if (node?.kind === 'resource') resourceIDs.add(node.id);
    };
    if (selection.node) {
      addNode(selection.node.id);
      inspectedApplicationID = selection.node.kind === 'application' ? selection.node.id : '';
    } else if (selection.edge) {
      addNode(selection.edge.source_id);
      addNode(selection.edge.target_id);
      inspectedApplicationID = '';
    } else {
      inspectedApplicationID = '';
      inspectedResourceIDs = null;
      return;
    }
    for (const id of appIDs) {
      const application = workspace?.applications.find((item) => item.id === id);
      if (application) applicationResourceIDs(application).forEach((resourceID) => resourceIDs.add(resourceID));
    }
    inspectedResourceIDs = [...resourceIDs];
  }

  function setProjectAreaTab(tab: 'applications' | 'topology') {
    projectAreaTab = tab;
    inspectedApplicationID = '';
    inspectedResourceIDs = null;
    if (tab === 'applications') topologyMode = 'browse';
    if (tab === 'applications') topologyComponentIntent = null;
  }

  function scopeLevelLabel(scopeType: string) {
    return scopeType === 'platform' ? '平台' : scopeType === 'team' ? '团队' : '项目';
  }

  function ownerLabel(ids: string[]) {
    const names = ids.map((id) => ownerOptions.find((option) => option.value === id)?.label || id);
    return names.length ? names.join('、') : '未配置';
  }
  async function loadWorkspace(id: string) {
    loadingWorkspace = true;
    loadingTopology = true;
    try {
      const [nextWorkspace, nextTopology] = await Promise.all([
        api.projectWorkspace(id),
        api.projectTopology(id)
      ]);
      workspace = nextWorkspace;
      projectTopology = nextTopology;
      workspaceProjectId = id;
    } catch (error) {
      onError(describeError(error, '项目驾驶舱加载失败'));
    } finally {
      loadingWorkspace = false;
      loadingTopology = false;
    }
  }
  async function loadTopology() {
    if (!selectedProjectId) return;
    loadingTopology = true;
    try {
      projectTopology = await api.projectTopology(selectedProjectId);
      workspace = await api.projectWorkspace(selectedProjectId);
    } catch (error) {
      onError(describeError(error, '项目拓扑刷新失败'));
    } finally {
      loadingTopology = false;
    }
  }
  function enterTopologyEdit() {
    if (selectedProject && canManageProject(selectedProject, 'update')) topologyMode = 'edit';
  }
  function requestTopologyResource(kind: string) {
    const id = crypto.randomUUID();
    topologyResourceRequestId = id;
    onRequestResourceCreate({ id, kind, scopeId: selectedProject?.scope.id ?? selectedScopeId, coverageTarget: '.map-side-column' });
  }
  $: if (resourceCreationCancelledId && resourceCreationCancelledId === topologyResourceRequestId) {
    topologyResourceRequestId = '';
    topologyComponentIntent = null;
  }
  $: if (resourceCreationResult && resourceCreationResult.requestId === topologyResourceRequestId) {
    topologyResourceRequestId = '';
    void (async () => {
      try {
        await api.addProjectTopologyResource(selectedProjectId, resourceCreationResult.resource.id);
        if (topologyComponentIntent?.position) await api.setProjectTopologyPosition(selectedProjectId, resourceCreationResult.resource.id, topologyComponentIntent.position);
        topologyComponentIntent = null;
        await loadTopology();
      } catch (error) {
        onError(describeError(error, '资源已创建，但加入项目拓扑失败'));
      }
    })();
  }
  async function useExistingTopologyComponent(intent: TopologyComponentIntent) {
    if (!selectedProjectId || !intent.id) return;
    try {
      if (intent.kind === 'application') {
        const node = projectTopology?.nodes.find((item) => item.kind === 'application' && item.id === intent.id);
        if (!node) throw new Error('应用不在当前项目拓扑中');
        if (intent.position) await api.setProjectTopologyPosition(selectedProjectId, node.id, intent.position);
        if (intent.position) await loadTopology();
      } else if (intent.kind === 'resource') {
        await api.addProjectTopologyResource(selectedProjectId, intent.id);
        if (intent.position) await api.setProjectTopologyPosition(selectedProjectId, intent.id, intent.position);
        await loadTopology();
      }
      onNotice('拓扑组件已添加');
    } catch (error) {
      onError(describeError(error, '添加拓扑组件失败'));
    }
  }
  function configureTopologyComponent(intent: TopologyComponentIntent) {
    if (intent.kind === 'application' && !intent.id) {
      topologyComponentIntent = intent;
      topologyApplicationCreateOpen = true;
      return;
    }
    if (intent.kind === 'resource' && !intent.id) {
      topologyComponentIntent = intent;
      requestTopologyResource(intent.resourceKind ?? 'Host');
      return;
    }
    if (intent.id) {
      void useExistingTopologyComponent(intent);
      return;
    }
    topologyComponentIntent = intent;
    topologyApplicationId = intent.kind === 'application' && intent.id ? intent.id : workspace?.applications[0]?.id ?? '';
    topologyResourceId = intent.kind === 'resource' && intent.id
      ? intent.id
      : availableResources.find((resource) => resourceCategoryFor(resource) === intent.resourceKind)?.id ?? '';
    topologyAnnotationName = '';
    topologyAnnotationDescription = '';
  }
  function cancelTopologyComponent() {
    if (topologyConfigBusy) return;
    topologyApplicationCreateOpen = false;
    topologyComponentIntent = null;
  }
  async function saveTopologyComponent() {
    const intent = topologyComponentIntent;
    if (!intent || !selectedProjectId || topologyConfigBusy) return;
    topologyConfigBusy = true;
    try {
      if (intent.kind === 'application') {
        const node = projectTopology?.nodes.find((item) => item.kind === 'application' && item.id === topologyApplicationId);
        if (!node) throw new Error('请选择当前项目中已存在的应用');
        topologyComponentIntent = null;
        if (intent.position) await api.setProjectTopologyPosition(selectedProjectId, node.id, intent.position);
        await loadTopology();
      } else if (intent.kind === 'resource') {
        if (!topologyResourceId) throw new Error('请选择资源');
        await api.addProjectTopologyResource(selectedProjectId, topologyResourceId);
        if (intent.position) await api.setProjectTopologyPosition(selectedProjectId, topologyResourceId, intent.position);
        topologyComponentIntent = null;
        await loadTopology();
      } else {
        const name = topologyAnnotationName.trim();
        if (!name) throw new Error('请填写外部实体标题');
        const node = await api.createProjectTopologyAnnotation(selectedProjectId, {
          name,
          metadata: { description: topologyAnnotationDescription.trim() }
        });
        if (intent.position) await api.setProjectTopologyPosition(selectedProjectId, node.id, intent.position);
        topologyComponentIntent = null;
        await loadTopology();
      }
      onNotice('拓扑组件已保存');
    } catch (error) {
      onError(describeError(error, '保存拓扑组件失败'));
    } finally {
      topologyConfigBusy = false;
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
      dependencies: draft.dependencies ?? [],
      relations: draft.relations ?? [],
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
    const addedFromTopology = topologyComponentIntent?.kind === 'application';
    const dropPosition = topologyComponentIntent?.position;
    const result = await api.createApplications(selectedProjectId, drafts.map(applicationPayload));
    applicationWizardOpen = false;
    await loadWorkspace(selectedProjectId);
    if (addedFromTopology) {
      topologyMode = 'edit';
      if (dropPosition) await Promise.all(result.items.map((application, index) => api.setProjectTopologyPosition(selectedProjectId, application.id, { x: Math.min(1171, dropPosition.x + index * 24), y: Math.min(9960, dropPosition.y + index * 24) })));
      topologyComponentIntent = null;
      await loadTopology();
    }
    onNotice(`${drafts.length} 个应用已新增`);
  }
  async function saveTopologyApplication(draft: ApplicationDraft) {
    if (!selectedProjectId || topologyComponentIntent?.kind !== 'application') return;
    const dropPosition = topologyComponentIntent.position;
    const application = await api.createApplication(selectedProjectId, applicationPayload(draft));
    topologyApplicationCreateOpen = false;
    if (dropPosition) await api.setProjectTopologyPosition(selectedProjectId, application.id, dropPosition);
    topologyComponentIntent = null;
    await loadWorkspace(selectedProjectId);
    await loadTopology();
    onNotice(`应用“${application.name}”已新增`);
  }
  async function saveTopologyApplicationEdit(draft: ApplicationDraft) {
    if (!topologyEditApplication) return;
    await updateApplication(topologyEditApplication, draft);
    topologyEditApplication = null;
    await loadTopology();
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
        dependencies: draft.dependencies ?? [],
        relations: draft.relations ?? [],
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
                            <span><small>关联</small><strong>{project.summary.dependencies}</strong></span>
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
      {#each [['应用', workspace?.summary.applications ?? 0, '应用和服务'], ['实例', workspace?.summary.instances ?? 0, '运行中的实例'], ['资源', workspace?.summary.resources ?? 0, '当前项目可见范围'], ['关联', workspace?.summary.dependencies ?? 0, '暂未配置'], ['告警', workspace?.summary.alerts ?? 0, '待处理信号']] as stat}<div
          class="workspace-stat"
        >
          <span>{stat[0]}</span><strong>{stat[1]}</strong><small
            >{stat[2]}</small
          >
        </div>{/each}
    </div>
    <div class="resource-map-columns">
      <section class="panel project-applications-area map-main-column">
      <div class="project-area-toolbar">
        <div class="project-area-tabs" role="tablist" aria-label="项目视图">
          <button class:active={projectAreaTab === 'applications'} role="tab" aria-selected={projectAreaTab === 'applications'} on:click={() => setProjectAreaTab('applications')}>
            <AppWindow size={16} />应用
          </button>
          <button class:active={projectAreaTab === 'topology'} role="tab" aria-selected={projectAreaTab === 'topology'} on:click={() => setProjectAreaTab('topology')}>
            <Network size={16} />拓扑
          </button>
        </div>
        <div class="section-heading-actions project-area-actions">
          <span class="muted">{workspace?.applications.length ?? 0} 个应用</span>
          {#if projectAreaTab === 'applications'}
            <button
              class="secondary"
              type="button"
              disabled={loadingWorkspace}
              data-tooltip="刷新应用"
              aria-label="刷新应用"
              on:click={() => selectedProject && void loadWorkspace(selectedProject.id)}
              ><RefreshCw size={15} />刷新</button
            ><button
              class="danger-button workspace-action"
              type="button"
              disabled={!selectedApplications.length}
              data-tooltip={selectedApplications.length ? `删除所选 ${selectedApplications.length} 个应用` : '请先选择应用'}
              on:click={() => (deletingApplications = selectedApplications)}
              ><Trash2 size={15} />删除</button
            ><button class="primary workspace-action" type="button" on:click={() => (applicationWizardOpen = true)}><Plus size={15} />新增</button>
          {:else}
            <button
              class="secondary"
              type="button"
              disabled={loadingTopology}
              data-tooltip="刷新拓扑"
              aria-label="刷新拓扑"
              on:click={() => void loadTopology()}
              ><RefreshCw size={15} />刷新</button
            ><button
              class="primary"
              type="button"
              disabled={!selectedProject || !canManageProject(selectedProject, 'update')}
              data-tooltip={topologyMode === 'edit' ? '返回浏览模式' : '编辑拓扑'}
              aria-label={topologyMode === 'edit' ? '返回浏览模式' : '编辑拓扑'}
              on:click={() => topologyMode = topologyMode === 'edit' ? 'browse' : 'edit'}
              >{#if topologyMode === 'edit'}<Map size={15} />浏览{:else}<Pencil size={15} />编辑{/if}</button
            >
          {/if}
        </div>
      </div>
        {#if projectAreaTab === 'applications'}
        <div class="project-applications-panel">
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
            onRowClick={inspectApplication}
            rowClass={(row) => row.id === inspectedApplicationID ? 'inspected-application-row' : ''}
            emptyText="当前项目暂无应用"
          >
            <svelte:fragment slot="cell" let:row let:column>
              {#if column.key === 'application'}
                <span class="application-table-name"><EntityBrandIcon kind="Application" fallback={row.icon || 'lucide:AppWindow'} size={17} /><span><strong>{row.name}</strong><small>{row.code}</small></span>{#if applicationAlertCounts[row.id]}<span class="application-alert-indicator" data-tooltip={`${applicationAlertCounts[row.id]} 条关联告警`} aria-label={`${applicationAlertCounts[row.id]} 条关联告警`}><AlertTriangle size={13} />{applicationAlertCounts[row.id]}</span>{/if}</span>
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
                  <div><small>关键</small><strong>{row.critical ? '是' : '否'}</strong></div>
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
        </div>
        {:else}
          <div class="project-topology-panel">
            <ProjectTopologyView
              project={selectedProject!}
              topology={projectTopology}
              applications={workspace?.applications ?? []}
              resources={availableResources}
              resourceScopeTypes={topologyResourceScopeTypes}
              loading={loadingTopology}
              mode={topologyMode}
              configurationOpen={Boolean(topologyComponentIntent)}
              pendingComponent={topologyComponentIntent}
              applicationAlertCounts={applicationAlertCounts}
              onSelectionChange={inspectTopologySelection}
              onRefresh={loadTopology}
              onConfigureNode={configureTopologyComponent}
              onEditApplication={(application) => (topologyEditApplication = workspace?.applications.find((item) => item.id === application.id) ?? null)}
              {onNotice}
              {onError}
            />
          </div>
        {/if}
      </section>
      <aside class="map-side-column" class:topology-configuring={Boolean(topologyComponentIntent)}>
        {#if projectAreaTab === 'topology' && topologyMode === 'edit' && topologyEditApplication}
          <div class="topology-application-create-region">
            <TopologyApplicationCreate
              resources={availableResources}
              {users}
              runtimeKind={topologyEditApplication.runtime_kind as ApplicationRuntimeKind}
              initialApplication={topologyEditApplication}
              onSave={saveTopologyApplicationEdit}
              onCancel={() => (topologyEditApplication = null)}
            />
          </div>
        {:else if projectAreaTab === 'topology' && topologyMode === 'edit' && topologyApplicationCreateOpen && topologyComponentIntent?.kind === 'application'}
          <div class="topology-application-create-region">
            <TopologyApplicationCreate
              resources={availableResources}
              {users}
              runtimeKind={topologyComponentIntent.runtimeKind as ApplicationRuntimeKind}
              onSave={saveTopologyApplication}
              onCancel={() => { topologyApplicationCreateOpen = false; topologyComponentIntent = null; }}
            />
          </div>
        {:else}
        <section class="panel map-side-panel">
          <div class="panel-heading">
            <div class="side-panel-heading-copy">
              <Server size={17} />
              <h2>资源</h2>
              <p>{inspectedApplicationID ? '当前应用关联的资源' : '本项目可见资源及应用已关联资源'}</p>
            </div>
          </div>
          {#each visibleWorkspaceResources as resource}<div
              class="resource-map-row"
            >
              <span class="resource-kind">{resource.kind}</span><span class="resource-map-copy"
              ><strong>{resource.name}</strong></span><span class="resource-map-meta"><span class="resource-reference-count">被 {resource.application_reference_count} 个应用依赖</span><ScopeLevelBadge type={resource.scope_type as 'platform' | 'team' | 'project'} label={scopeLevelLabel(resource.scope_type)} /></span
              >
            </div>{:else}<div class="detail-empty">暂无可见资源</div>{/each}
        </section>
        <section class="panel map-side-panel alerts-panel">
          <div class="panel-heading">
            <div class="side-panel-heading-copy">
              <AlertTriangle size={17} />
              <h2>告警</h2>
              <p>{inspectedApplicationID ? '当前应用关联的告警' : '本项目待处理告警'}</p>
            </div>
          </div>
          {#each visibleWorkspaceAlerts as alert}<div class="alert-map-row">
              <span class="severity {alert.severity}"></span><span
                ><strong>{alert.title}</strong><small
                  >{statusLabel(alert.status)}</small
                ></span
              >
            </div>{:else}<div class="detail-empty success-empty">
              <CheckCircle2 size={15} />当前没有项目告警
            </div>{/each}
        </section>
        {#if projectAreaTab === 'topology' && topologyMode === 'edit' && topologyComponentIntent && !applicationWizardOpen && !topologyApplicationCreateOpen && !topologyResourceRequestId}
          <div class="topology-config-overlay">
            <section class="topology-config-window" aria-label="配置拓扑组件">
              <header><span class="topology-config-icon">{#if topologyComponentIntent.kind === 'application'}<EntityBrandIcon kind="Application" fallback="lucide:AppWindow" size={17} />{:else if topologyComponentIntent.kind === 'resource'}<ResourceBrandIcon resource={{ kind: topologyComponentIntent.resourceKind ?? 'Host' }} fallback={resourceCategoryIcon(topologyComponentIntent.resourceKind ?? 'Host')} size={17} />{:else}<StickyNote size={17} />{/if}</span><div><strong>{topologyComponentIntent.kind === 'application' ? '配置应用节点' : topologyComponentIntent.kind === 'resource' ? '配置资源节点' : '配置外部实体'}</strong><small>确认组件内容后添加到项目拓扑</small></div></header>
              <div class="topology-config-body">
                {#if topologyComponentIntent.kind === 'application'}
                  <label>应用<select bind:value={topologyApplicationId}><option value="">选择应用</option>{#each workspace?.applications ?? [] as application (application.id)}<option value={application.id}>{application.name} · {application.code}</option>{/each}</select></label>
                  <button class="secondary" type="button" on:click={() => (applicationWizardOpen = true)}><Plus size={14} />新增应用</button>
                {:else if topologyComponentIntent.kind === 'resource'}
                  <label>资源<select bind:value={topologyResourceId}><option value="">选择资源</option>{#each topologyResourceOptions as resource (resource.id)}<option value={resource.id}>{resource.name} · {resource.kind}</option>{/each}</select></label>
                  <button class="secondary" type="button" on:click={() => requestTopologyResource(topologyComponentIntent?.resourceKind || 'Host')}><Plus size={14} />新增资源</button>
                {:else}
                  <label>名称<input bind:value={topologyAnnotationName} maxlength="200" placeholder="例如：支付核心系统" /></label>
                  <label>说明<textarea bind:value={topologyAnnotationDescription} rows="4" placeholder="说明其系统边界、职责或连接方式"></textarea></label>
                {/if}
              </div>
              <footer><button class="secondary" type="button" disabled={topologyConfigBusy} on:click={cancelTopologyComponent}>取消</button><button class="primary" type="button" disabled={topologyConfigBusy} on:click={saveTopologyComponent}><CheckCircle2 size={14} />{topologyConfigBusy ? '保存中' : '确认并添加'}</button></footer>
            </section>
          </div>
        {/if}
        {/if}
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
    applications={workspace?.applications ?? []}
    projects={allProjects.map((project) => ({ id: project.id, name: project.name }))}
    initialApplication={editingApplication}
    initialRuntimeKind={topologyComponentIntent?.kind === 'application' ? topologyComponentIntent.runtimeKind : ''}
    onSave={saveApplications}
    onUpdate={updateApplication}
    {onResourceCreated}
      onRequestResourceCreate={(kind: RuntimeResourceKind, instanceId?: string) =>
      onRequestResourceCreate({
        id: crypto.randomUUID(),
        kind,
        scopeId: selectedProject?.scope.id ?? selectedScopeId,
        ...(instanceId ? { instanceId } : {})
      })}
    {resourceCreationResult}
    onCancel={() => { applicationWizardOpen = false; editingApplication = null; topologyComponentIntent = null; }}
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
