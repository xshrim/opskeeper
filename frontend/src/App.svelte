<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Boxes,
    Bot,
    Building2,
    ChevronDown,
    ClipboardCheck,
    CloudDownload,
    Copy,
    Eye,
    EyeOff,
    FolderKanban,
    LayoutDashboard,
    LogOut,
    PanelLeftClose,
    PanelLeftOpen,
    Pencil,
    PlugZap,
    Plus,
    RefreshCw,
    ScanSearch,
    Search,
    ShieldCheck,
    Sparkles,
    Stethoscope,
    Trash2,
    UserRound,
    UsersRound
  } from 'lucide-svelte';
  import { fetchHealth, toStatusRows, type HealthReport } from './lib/health';
  import { resourceHasConnector, resourceIcon as getResourceIcon, resourceSchemaName as getResourceSchemaName } from './lib/resources';
  import { loadResourceConnectionChecks as loadResourceConnectionChecksData } from './features/resource/resourceData';
  import { buildScopeChoices, scopeName as getScopeName, scopeType as getScopeType, type ScopeChoice } from './lib/scope';
  import { viewBreadcrumb as getViewBreadcrumb, viewTitle as getViewTitle } from './lib/navigation';
  import {
    projectSelection,
    resourceInWorkspace,
    teamSelection
  } from './lib/workspace';
  import {
    actorPermissionsAtScope as getActorPermissionsAtScope,
    canManageResource,
    userRoleBindings as getUserRoleBindings
  } from './features/access/accessUtils';
  import { formatDate } from './lib/format';
  import MessageBanner from './components/MessageBanner.svelte';
  import DiagnosisPage from './features/diagnosis/DiagnosisPage.svelte';
  import AgentProfilesPage from './features/agent/AgentProfilesPage.svelte';
  import SkillRegistryPage from './features/skill/SkillRegistryPage.svelte';
  import ProfilePage from './features/profile/ProfilePage.svelte';
  import OperationPage from './features/operation/OperationPage.svelte';
  import AuthGate from './features/auth/AuthGate.svelte';
  import DiscoveryPage from './features/discovery/DiscoveryPage.svelte';
  import InspectionPage from './features/inspection/InspectionPage.svelte';
  import OverviewPage from './features/overview/OverviewPage.svelte';
  import ProjectPage from './features/project/ProjectPage.svelte';
  import ResourcePage from './features/resource/ResourcePage.svelte';
  import AccessPage from './features/access/AccessPage.svelte';
  import Topbar from './layouts/Topbar.svelte';
  import AppShell from './layouts/AppShell.svelte';
  import {
    api,
    ApiError,
    type ConnectionCheck,
    type ConnectorCapability,
    type DiscoveryItem,
    type DiscoveryProjectMapping,
    type DiscoveryRun,
    type Group,
    type Platform,
    type Project,
    type Resource,
    type ResourceSchema,
    type RoleBinding,
    type RoleDefinition,
    type Team,
    type MCPSnapshot,
    type SkillVersion,
    type AgentProfileVersion,
    type User,
    type UserPreferences
  } from './lib/api';

  type View =
    | 'overview'
    | 'project'
    | 'discovery'
    | 'resource'
    | 'skill'
    | 'agent'
    | 'operation'
    | 'diagnosis'
    | 'inspection'
    | 'access'
    | 'profile';
  type Theme = UserPreferences['theme'];
  type SidebarMode = UserPreferences['sidebar_mode'];
  type AccessTab = 'teams' | 'users' | 'roles';
  type ProjectMappingDraft = DiscoveryProjectMapping & {
    mode: 'existing' | 'create' | 'ignore';
  };
  type AIProviderBindingSummary = {
    scope_id: string;
    provider_resource_id: string;
    tag: string;
  };
  let authState: 'loading' | 'login' | 'ready' = 'loading';
  let currentUser: User | null = null;
  let notice = '';
  let noticeTimer: number | null = null;
  let errorMessage = '';
  let errorTimer: number | null = null;
  let activeMessage = '';
  let activeMessageTone: 'success' | 'error' = 'success';
  let messageInChildSurface = false;
  let resourceChildSurfaceActive = false;
  let busy = false;
  let view: View = 'overview';
  let preferences: UserPreferences = {
    theme: 'auto',
    sidebar_mode: 'fixed',
    sidebar_collapsed: false
  };
  let sidebarHovered = false;
  let previousSidebarCompact = false;
  let userMenuOpen = false;
  let teamMenuOpen = false;
  let accessMenuOpen = false;
  let isPlatformAdmin = false;
  let hasPlatformRole = false;
  let selectedTeamId = '';
  let selectedProjectId = '';
  let copiedControl:
    'created-password' | 'reset-username' | 'reset-credentials' | null = null;
  let copiedControlTimer: number | null = null;
  let platform: Platform | null = null;
  let teams: Team[] = [];
  let projects: Project[] = [];
  let resources: Resource[] = [];
  let contextResources: Resource[] = [];
  let aiProviderBindings: AIProviderBindingSummary[] = [];
  let schemas: ResourceSchema[] = [];
  let health: HealthReport | null = null;
  let healthController: AbortController | null = null;
  let healthInterval: number | null = null;
  let selectedScopeId = '';
  let selectedResourceId = '';
  let connectionBusy = false;
  let resourceConnectionChecks: Record<string, ConnectionCheck | null> = {};
  let groups: Group[] = [];
  let groupMembers: Record<string, string[]> = {};
  let roles: RoleDefinition[] = [];
  let bindings: RoleBinding[] = [];
  let agentProfileResources: Resource[] = [];
  let operationSnapshots: Record<string, MCPSnapshot[]> = {};
  let accessTab: AccessTab = 'teams';
  let openCreateTeamRequest = 0;

  $: scopeChoices = buildScopeChoices(platform, teams, projects);
  $: activeScope =
    scopeChoices.find((scope) => scope.id === selectedScopeId) ??
    scopeChoices[0];
  $: selectedTeam = teams.find((team) => team.id === selectedTeamId) ?? null;
  $: selectedTeamProjects = projects.filter(
    (project) => project.team_id === selectedTeamId
  );
  $: selectedProject =
    selectedTeamProjects.find((project) => project.id === selectedProjectId) ??
    null;
  $: workspaceProjects = selectedTeamId
    ? selectedTeamProjects
    : hasPlatformRole
      ? projects
      : [];
  $: visibleProjects = selectedScopeId
    ? activeScope?.type === 'platform'
      ? projects
      : activeScope?.type === 'team'
        ? selectedTeamProjects
        : projects.filter((project) => project.scope.id === selectedScopeId)
    : projects;
  $: visibleResources = selectedScopeId
    ? resources.filter((resource) => resourceInActiveWorkspace(resource))
    : resources;
  $: selectedResource =
    resources.find((resource) => resource.id === selectedResourceId) ?? null;
  $: selectedResourceCanUpdate = selectedResource
    ? resourceCanManage(selectedResource, 'resource:update')
    : false;
  $: selectedResourceCanDelete = selectedResource
    ? resourceCanManage(selectedResource, 'resource:delete')
    : false;
  $: rows = toStatusRows(health);
  $: selectedResourceHasConnector = Boolean(
    selectedResource && resourceHasConnector(selectedResource)
  );
  $: kubernetesClusters = resources.filter(
    (resource) => resource.kind === 'Kubernetes'
  );
  $: skillResources = resources.filter((item) => item.kind === 'Skill');
  $: agentProfileResources = resources.filter(
    (item) => item.kind === 'AgentProfile'
  );
  $: executableTargets = visibleResources.filter(
    (item) => item.kind !== 'AIProvider' && item.kind !== 'Skill'
  );
  $: sidebarCompact =
    preferences.sidebar_mode === 'hover'
      ? !sidebarHovered
      : preferences.sidebar_collapsed;
  $: if (sidebarCompact && !previousSidebarCompact) {
    accessMenuOpen = false;
  }
  $: previousSidebarCompact = sidebarCompact;
  $: avatarURL = preferences.avatar_updated_at
    ? api.avatarURL(preferences.avatar_updated_at)
    : '';
  $: if (!currentUser) applySystemTheme();
  onMount(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    applySystemTheme();
    const refreshTheme = () => {
      if (currentUser) {
        applyTheme();
      } else {
        applySystemTheme();
      }
    };
    media.addEventListener('change', refreshTheme);
    document.addEventListener('pointerdown', handleDocumentPointerDown);
    return () => {
      if (noticeTimer !== null) window.clearTimeout(noticeTimer);
      if (errorTimer !== null) window.clearTimeout(errorTimer);
      stopHealthPolling();
      media.removeEventListener('change', refreshTheme);
      document.removeEventListener('pointerdown', handleDocumentPointerDown);
    };
  });

  $: if (notice) {
    if (noticeTimer !== null) window.clearTimeout(noticeTimer);
    noticeTimer = window.setTimeout(() => {
      notice = '';
      noticeTimer = null;
    }, 5_000);
  }

  $: if (errorMessage) {
    if (errorTimer !== null) window.clearTimeout(errorTimer);
    errorTimer = window.setTimeout(() => {
      errorMessage = '';
      errorTimer = null;
    }, 5_000);
  }

  $: activeMessage = errorMessage || notice;
  $: activeMessageTone = errorMessage ? 'error' : 'success';
  $: messageInChildSurface = resourceChildSurfaceActive;

  function startHealthPolling() {
    if (healthInterval !== null) return;
    const controller = new AbortController();
    healthController = controller;
    const checkHealth = async () => {
      try {
        health = await fetchHealth(controller.signal);
      } catch {
        if (healthController === controller) health = null;
      }
    };
    void checkHealth();
    healthInterval = window.setInterval(checkHealth, 15_000);
  }

  function stopHealthPolling() {
    healthController?.abort();
    healthController = null;
    if (healthInterval !== null) {
      window.clearInterval(healthInterval);
      healthInterval = null;
    }
    health = null;
  }

  async function completeAuthenticatedSession(user: User, sessionNotice = '') {
    currentUser = user;
    await loadPreferences();
    if (user.must_change_password) return;

    const sessionContext = await api.sessionContext();
    isPlatformAdmin = sessionContext.platform_admin;
    hasPlatformRole = sessionContext.platform_role;
    startHealthPolling();
    await loadWorkspace();
    if (sessionNotice) notice = sessionNotice;
  }

  async function loadWorkspace() {
    errorMessage = '';
    try {
      const [loadedPlatform, teamPage, loadedSchemas, resourcePage] =
        await Promise.all([
          api.platform(),
          api.teams(),
          api.schemas(),
          api.resources()
        ]);
      platform = loadedPlatform;
      teams = teamPage.items;
      schemas = loadedSchemas;
      resources = resourcePage.items;
      const contextResourceResult = await Promise.allSettled([
        api.contextResources()
      ]);
      const contextPage = contextResourceResult[0];
      contextResources =
        contextPage.status === 'fulfilled' ? contextPage.value.items : [];
      await loadResourceConnectionChecks(resources);
      const projectPages = await Promise.all(
        teams.map((team) => api.projects(team.id))
      );
      projects = projectPages.flatMap((page) => page.items);
      const scopeIDs = [
        loadedPlatform.scope.id,
        ...teams.map((team) => team.scope.id),
        ...projects.map((project) => project.scope.id)
      ];
      const bindingPages = await Promise.allSettled(
        [...new Set(scopeIDs)].map((scopeID) => api.aiProviderBindings(scopeID))
      );
      aiProviderBindings = bindingPages.flatMap((result) =>
        result.status === 'fulfilled' ? result.value : []
      );
      const defaultTeam = hasPlatformRole ? undefined : teams[0];
      selectedTeamId = defaultTeam?.id ?? '';
      selectedProjectId = '';
      selectedScopeId = defaultTeam?.scope.id ?? platform.scope.id;
    } catch (error) {
      errorMessage = describeError(error, '工作区数据加载失败');
    }
  }

  async function logout() {
    busy = true;
    try {
      await api.logout();
    } finally {
      currentUser = null;
      isPlatformAdmin = false;
      hasPlatformRole = false;
      selectedTeamId = '';
      selectedProjectId = '';
      authState = 'login';
      applySystemTheme();
      stopHealthPolling();
      view = 'overview';
      userMenuOpen = false;
      busy = false;
    }
  }

  function chooseView(nextView: View) {
    view = nextView;
    notice = '';
    errorMessage = '';
    userMenuOpen = false;
    teamMenuOpen = false;
  }

  function chooseAccessTab(tab: AccessTab) {
    accessTab = tab;
    accessMenuOpen = true;
    chooseView('access');
  }

  async function loadPreferences() {
    try {
      preferences = await api.preferences();
      applyTheme();
    } catch (error) {
      errorMessage = describeError(error, '个人偏好加载失败');
    }
  }

  function applyTheme() {
    if (typeof window === 'undefined') return;
    const isDark =
      preferences.theme === 'dark' ||
      (preferences.theme === 'auto' &&
        window.matchMedia('(prefers-color-scheme: dark)').matches);
    document.documentElement.dataset.theme = isDark ? 'dark' : 'light';
  }

  function applySystemTheme() {
    if (typeof window === 'undefined') return;
    document.documentElement.dataset.theme = window.matchMedia(
      '(prefers-color-scheme: dark)'
    ).matches
      ? 'dark'
      : 'light';
  }

  function openProfile() {
    chooseView('profile');
  }

  function openTeamDialog() {
    openCreateTeamRequest += 1;
    chooseAccessTab('teams');
  }

  function markCopySuccess(control: NonNullable<typeof copiedControl>) {
    copiedControl = control;
    if (copiedControlTimer !== null) window.clearTimeout(copiedControlTimer);
    copiedControlTimer = window.setTimeout(() => {
      copiedControl = null;
      copiedControlTimer = null;
    }, 3000);
  }

  async function toggleSidebar() {
    if (preferences.sidebar_mode === 'hover') {
      preferences = {
        ...preferences,
        sidebar_mode: 'fixed',
        sidebar_collapsed: true
      };
    } else {
      preferences = {
        ...preferences,
        sidebar_collapsed: !preferences.sidebar_collapsed
      };
    }
    try {
      preferences = await api.updatePreferences({
        theme: preferences.theme,
        sidebar_mode: preferences.sidebar_mode,
        sidebar_collapsed: preferences.sidebar_collapsed
      });
    } catch (error) {
      errorMessage = describeError(error, '侧边导航设置保存失败');
    }
  }

  function handleGlobalKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      userMenuOpen = false;
      teamMenuOpen = false;
      accessMenuOpen = false;
    }
  }

  function handleDocumentPointerDown(event: PointerEvent) {
    const target = event.target;
    if (!(target instanceof Element)) return;
    if (userMenuOpen && !target.closest('.sidebar-user-menu')) {
      userMenuOpen = false;
    }
    if (teamMenuOpen && !target.closest('.workspace-team-wrap')) {
      teamMenuOpen = false;
    }
    if (accessMenuOpen && view !== 'access' && !target.closest('.nav-group')) {
      accessMenuOpen = false;
    }
  }

  function chooseTeam(teamID: string) {
    const selection = teamSelection(teamID, hasPlatformRole, platform?.scope.id, teams);
    if (!selection) return;
    selectedTeamId = selection.teamId;
    selectedProjectId = selection.projectId;
    selectedScopeId = selection.scopeId;
    selectedResourceId = '';
    if (teamID || hasPlatformRole) teamMenuOpen = false;
  }

  function chooseProject(projectID: string) {
    const selection = projectSelection(
      projectID,
      selectedTeamId,
      selectedTeam?.scope.id,
      platform?.scope.id,
      projects
    );
    selectedTeamId = selection.teamId;
    selectedProjectId = selection.projectId;
    selectedScopeId = selection.scopeId;
    selectedResourceId = '';
  }

  function resourceInActiveWorkspace(resource: Resource) {
    return resourceInWorkspace(
      resource,
      activeScope?.type,
      platform?.scope.id,
      selectedTeam?.scope.id,
      selectedTeamProjects.map((project) => project.scope.id),
      selectedProject?.scope.id
    );
  }

  async function loadResourceConnectionChecks(items: Resource[]) {
    const checks = await loadResourceConnectionChecksData(items);
    resourceConnectionChecks = {
      ...resourceConnectionChecks,
      ...checks
    };
  }

  async function action(operation: () => Promise<void>) {
    busy = true;
    errorMessage = '';
    try {
      await operation();
    } catch (error) {
      errorMessage = describeError(error, '操作失败');
    } finally {
      busy = false;
    }
  }

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 403) return '当前账号没有执行此操作的权限。';
      if (error.status === 401) return '会话已过期，请重新登录。';
      if (
        error.status === 409 &&
        error.message === 'Resource conflicts with existing data'
      ) {
        return '当前级别下已有同名 AI 接入或凭据冲突，请更换名称后重试。';
      }
      return error.message || fallback;
    }
    if (error instanceof SyntaxError) return '配置必须是有效的 JSON 对象。';
    if (error instanceof Error) return error.message || fallback;
    return fallback;
  }

  const viewTitle = (currentView: View) => getViewTitle(currentView, accessTab);
  const viewBreadcrumb = (currentView: View) => getViewBreadcrumb(currentView, accessTab);

  const scopeName = (id: string) => getScopeName(scopeChoices, id);
  const scopeType = (id: string) => getScopeType(scopeChoices, id);

  function resourceCanManage(resource: Resource, permission: string) {
    return canManageResource(
      resource,
      permission,
      selectedScopeId,
      isPlatformAdmin,
      actorPermissionsAtScope(resource.scope_id)
    );
  }

  function userRoleBindings(userID: string) {
    return getUserRoleBindings(userID, groups, groupMembers, bindings);
  }

  function actorPermissionsAtScope(scopeID: string) {
    return getActorPermissionsAtScope(
      scopeID,
      currentUser?.id,
      isPlatformAdmin,
      roles,
      groups,
      groupMembers,
      bindings,
      scopeChoices
    );
  }

  function resourceSchemaName(kind: string) {
    return getResourceSchemaName(kind, schemas);
  }

  function resourceIcon(kind: string) {
    return getResourceIcon(kind, schemas);
  }
</script>

<svelte:head>
  <meta name="description" content="OpsKeeper platform control plane" />
</svelte:head>

<svelte:window on:keydown={handleGlobalKeydown} />

<AuthGate
  bind:authState
  {currentUser}
  onAuthenticated={completeAuthenticatedSession}
/>
{#if authState !== 'loading' && authState !== 'login' && !currentUser?.must_change_password}
  <AppShell
    {sidebarCompact}
    sidebarHoverMode={preferences.sidebar_mode === 'hover'}
  >
    <aside
      class="sidebar"
      class:sidebar-compact={sidebarCompact}
      on:mouseenter={() => (sidebarHovered = true)}
      on:mouseleave={() => (sidebarHovered = false)}
    >
      <div class="brand">
        <span class="brand-mark" aria-hidden="true">O</span><span
          class="brand-copy">OpsKeeper<small>智能值守平台</small></span
        >
      </div>
      <div class="workspace-label">WORKSPACE</div>
      <nav aria-label="主导航">
        <button
          aria-label="总览"
          class:active={view === 'overview'}
          class="nav-item"
          on:click={() => chooseView('overview')}
          data-tooltip={sidebarCompact ? '总览' : undefined}
          ><LayoutDashboard
            size={18}
            strokeWidth={1.8}
            aria-hidden="true"
          /><span class="nav-item-label">总览</span></button
        >
        <button
          aria-label="项目"
          class:active={view === 'project'}
          class="nav-item"
          on:click={() => chooseView('project')}
          data-tooltip={sidebarCompact ? '项目 / Project' : undefined}
          ><FolderKanban size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">项目</span
          ></button
        >
        <button
          aria-label="资源"
          class:active={view === 'resource'}
          class="nav-item"
          on:click={() => chooseView('resource')}
          data-tooltip={sidebarCompact ? '资源' : undefined}
          ><Boxes size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">资源</span
          ></button
        >
        <button
          aria-label="集群导入"
          class:active={view === 'discovery'}
          class="nav-item"
          on:click={() => chooseView('discovery')}
          data-tooltip={sidebarCompact ? '集群导入' : undefined}
          ><CloudDownload size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">集群导入</span
          ></button
        >
        <button
          aria-label="Skill"
          class:active={view === 'skill'}
          class="nav-item"
          on:click={() => chooseView('skill')}
          data-tooltip={sidebarCompact ? 'Skill' : undefined}
          ><ClipboardCheck
            size={18}
            strokeWidth={1.8}
            aria-hidden="true"
          /><span class="nav-item-label">Skill</span></button
        >
        <button
          aria-label="Agent 专家"
          class:active={view === 'agent'}
          class="nav-item"
          on:click={() => chooseView('agent')}
          data-tooltip={sidebarCompact ? 'Agent 专家' : undefined}
          ><Bot size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">Agent 专家</span
          ></button
        >
        <button
          aria-label="AI 诊断"
          class:active={view === 'diagnosis'}
          class="nav-item"
          on:click={() => chooseView('diagnosis')}
          data-tooltip={sidebarCompact ? 'AI 诊断' : undefined}
          ><ScanSearch size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">AI 诊断</span
          ></button
        >
        <button
          aria-label="自动巡检"
          class:active={view === 'inspection'}
          class="nav-item"
          on:click={() => chooseView('inspection')}
          data-tooltip={sidebarCompact ? '自动巡检' : undefined}
          ><Stethoscope size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">自动巡检</span
          ></button
        >
        <button
          aria-label="受控操作"
          class:active={view === 'operation'}
          class="nav-item"
          on:click={() => chooseView('operation')}
          data-tooltip={sidebarCompact ? '受控操作' : undefined}
          ><ClipboardCheck
            size={18}
            strokeWidth={1.8}
            aria-hidden="true"
          /><span class="nav-item-label">受控操作</span></button
        >
        <div class="nav-group" class:open={accessMenuOpen}>
          <button
            aria-label="展开权限菜单"
            aria-expanded={accessMenuOpen}
            class:active={view === 'access'}
            class="nav-item nav-group-trigger"
            on:click={() => (accessMenuOpen = !accessMenuOpen)}
            data-tooltip={sidebarCompact ? '权限 / Access' : undefined}
            ><UsersRound size={18} strokeWidth={1.8} aria-hidden="true" /><span
              class="nav-item-label">权限</span
            ><ChevronDown
              class="nav-group-chevron"
              size={14}
              strokeWidth={1.8}
              aria-hidden="true"
            /></button
          >
          {#if accessMenuOpen}
            <div class="nav-submenu" aria-label="权限子菜单">
              <button
                type="button"
                class:active={view === 'access' && accessTab === 'teams'}
                on:click={() => chooseAccessTab('teams')}
                ><Building2
                  size={15}
                  strokeWidth={1.8}
                  aria-hidden="true"
                /><span>团队管理</span></button
              ><button
                type="button"
                class:active={view === 'access' && accessTab === 'users'}
                on:click={() => chooseAccessTab('users')}
                ><UserRound
                  size={15}
                  strokeWidth={1.8}
                  aria-hidden="true"
                /><span>用户管理</span></button
              ><button
                type="button"
                class:active={view === 'access' && accessTab === 'roles'}
                on:click={() => chooseAccessTab('roles')}
                ><ShieldCheck
                  size={15}
                  strokeWidth={1.8}
                  aria-hidden="true"
                /><span>角色管理</span></button
              >
            </div>
          {/if}
        </div>
      </nav>
      <div class="sidebar-footer">
        <div class="user-menu-wrap sidebar-user-menu">
          <button
            class="user-menu-trigger"
            aria-label="打开用户菜单"
            aria-expanded={userMenuOpen}
            on:click={() => (userMenuOpen = !userMenuOpen)}
          >
            {#if avatarURL}<img
                src={avatarURL}
                alt=""
                class="avatar avatar-image"
              />{:else}<span class="avatar"
                >{(currentUser?.display_name || currentUser?.username || 'U')
                  .slice(0, 1)
                  .toUpperCase()}</span
              >{/if}<span class="user-menu-name"
              >{currentUser?.display_name || currentUser?.username}</span
            >
          </button>
          {#if userMenuOpen}<div class="user-menu" role="menu">
              <button role="menuitem" on:click={openProfile}
                ><UserRound
                  size={16}
                  strokeWidth={1.8}
                  aria-hidden="true"
                />个人中心</button
              ><button role="menuitem" on:click={logout} disabled={busy}
                ><LogOut
                  size={16}
                  strokeWidth={1.8}
                  aria-hidden="true"
                />退出登录</button
              >
            </div>{/if}
        </div>
        <button
          class="sidebar-toggle"
          aria-label={sidebarCompact ? '展开导航栏' : '折叠导航栏'}
          data-tooltip={sidebarCompact ? '展开导航栏' : '折叠导航栏'}
          on:click={toggleSidebar}
        >
          {#if sidebarCompact}<PanelLeftOpen
              size={18}
              strokeWidth={1.8}
              aria-hidden="true"
            />{:else}<PanelLeftClose
              size={18}
              strokeWidth={1.8}
              aria-hidden="true"
            />{/if}
        </button>
      </div>
    </aside>

    <main
      class="main-content"
      class:diagnosis-main-content={view === 'diagnosis'}
    >
      <Topbar
        breadcrumb={view === 'access'
          ? viewBreadcrumb(view)
          : `${activeScope?.name ?? '平台'} / ${viewBreadcrumb(view)}`}
        title={viewTitle(view)}
        {activeMessage}
        {activeMessageTone}
        {messageInChildSurface}
        {hasPlatformRole}
        {selectedTeamId}
        {selectedProjectId}
        {teams}
        {workspaceProjects}
        {chooseTeam}
        {chooseProject}
      />

      {#if view === 'overview'}
        <OverviewPage
          teamCount={teams.length}
          projectCount={visibleProjects.length}
          resourceCount={visibleResources.length}
          healthStatus={health?.status}
          {rows}
          {visibleResources}
          {resourceSchemaName}
          {scopeName}
          {resourceIcon}
          onOpenResources={() => chooseView('resource')}
          onOpenResource={(resource) => {
            selectedResourceId = resource.id;
            chooseView('resource');
          }}
        />
      {:else if view === 'profile'}
        <ProfilePage
          bind:currentUser
          {avatarURL}
          bind:preferences
          onApplyTheme={applyTheme}
          onNotice={(message) => (notice = message)}
          onError={(message) => (errorMessage = message)}
        />
      {:else if view === 'project'}
        <ProjectPage
          {teams}
          {visibleProjects}
          bind:selectedScopeId
          {busy}
          {scopeName}
          onSelectTeam={(team) => (selectedScopeId = team.scope.id)}
          onSelectProject={(project) => (selectedScopeId = project.scope.id)}
          onOpenTeamDialog={openTeamDialog}
          onProjectCreated={(project) => (projects = [...projects, project])}
          onNotice={(message) => (notice = message)}
          onError={(message) => (errorMessage = message)}
        />
      {:else if view === 'discovery'}
        <DiscoveryPage
          {kubernetesClusters}
          {teams}
          {projects}
          scopeTypes={Object.fromEntries(
            scopeChoices.map((scope) => [scope.id, scope.type])
          )}
          {scopeName}
          {formatDate}
          onWorkspaceReload={loadWorkspace}
          onNotice={(message) => (notice = message)}
          onError={(message) => (errorMessage = message)}
        />
      {:else if view === 'resource'}
        <ResourcePage
          {visibleResources}
          bind:selectedResourceId
          bind:resourceConnectionChecks
          bind:operationSnapshots
          bind:childSurfaceActive={resourceChildSurfaceActive}
          bind:busy
          bind:connectionBusy
          onSelectResourceScope={(scopeId) => {
            selectedScopeId = scopeId;
            const project = projects.find((item) => item.scope.id === scopeId);
            const team = teams.find((item) => item.scope.id === scopeId);
            selectedTeamId = project?.team_id ?? team?.id ?? '';
            selectedProjectId = project?.id ?? '';
          }}
          {resourceCanManage}
          {scopeType}
          {formatDate}
          {resourceIcon}
          {selectedResource}
          {selectedResourceCanDelete}
          {selectedResourceHasConnector}
          {selectedScopeId}
          bind:aiProviderBindings
          {selectedResourceCanUpdate}
          {scopeName}
          onNotice={(message) => (notice = message)}
          onError={(message) => (errorMessage = message)}
          bind:resources
          {schemas}
          {activeMessage}
          {activeMessageTone}
        />
      {:else if view === 'inspection'}
        <InspectionPage
          scopeId={selectedScopeId}
          {executableTargets}
          agentProfiles={agentProfileResources}
          {scopeName}
          {resourceInActiveWorkspace}
          onNotice={(message) => (notice = message)}
          onError={(message) => (errorMessage = message)}
        />
      {:else if view === 'operation'}
        <OperationPage
          {resources}
          scopeId={selectedScopeId}
          bind:operationSnapshots
          {resourceSchemaName}
          {formatDate}
          onNotice={(message) => (notice = message)}
          onError={(message) => (errorMessage = message)}
        />
      {:else if view === 'diagnosis'}
        <DiagnosisPage
          scopeId={selectedScopeId}
          runAction={action}
          {describeError}
          onError={(message) => (errorMessage = message)}
          {formatDate}
          scopeLabel={activeScope?.name ?? '当前级别'}
          {resources}
          {contextResources}
          {resourceInActiveWorkspace}
          {busy}
          onNotice={(message) => (notice = message)}
          {resourceIcon}
          {resourceSchemaName}
          {scopeName}
        />
      {:else if view === 'agent'}
        <AgentProfilesPage
          profiles={agentProfileResources}
          scopeId={selectedScopeId}
          {scopeName}
          {formatDate}
          onResourceCreated={(resource) =>
            (resources = [resource, ...resources])}
          onNotice={(message) => (notice = message)}
          onError={(message) => (errorMessage = message)}
        />
      {:else if view === 'skill'}
        <SkillRegistryPage
          resources={skillResources}
          scopeId={selectedScopeId}
          {scopeName}
          onNotice={(message) => (notice = message)}
          onError={(message) => (errorMessage = message)}
        />
      {:else if view === 'access'}
        <AccessPage
          bind:accessTab
          {openCreateTeamRequest}
          bind:teams
          {projects}
          {resources}
          {scopeChoices}
          preferredScopeId={selectedScopeId}
          {currentUser}
          {isPlatformAdmin}
          {activeMessage}
          {activeMessageTone}
          onNotice={(message) => (notice = message)}
          onError={(message) => (errorMessage = message)}
        />
      {/if}
    </main>
  </AppShell>
{/if}
