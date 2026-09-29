<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ClipboardCheck,
    Copy,
    Pencil,
    Plus,
    Search,
    ShieldCheck,
    Trash2,
    ChevronDown,
    RefreshCw
  } from 'lucide-svelte';
  import MessageBanner from '../../components/MessageBanner.svelte';
  import IconPicker from '../../components/IconPicker.svelte';
  import IconValue from '../../components/IconValue.svelte';
  import PasswordInput from '../../components/PasswordInput.svelte';
  import AccessManagementWorkbench from './AccessManagementWorkbench.svelte';
  import {
    api,
    ApiError,
    type AuditEvent,
    type Group,
    type Project,
    type Resource,
    type ResourceRoleBinding,
    type ResourceRoleDefinition,
    type RoleBinding,
    type RoleDefinition,
    type Team,
    type User
  } from '../../lib/api';
  import {
    actorPermissionsAtScope as getActorPermissionsAtScope,
    organizationScopeVisible,
    resourceVisibleToScope as isResourceVisibleToScope,
    userRoleBindings as getUserRoleBindings,
    usersAtScopes,
    viewerResourceRoleAllowed as isViewerResourceRoleAllowed
  } from './accessUtils';

  type AccessTab = 'teams' | 'users' | 'roles';
  type DisableTarget = { kind: 'team' | 'user'; ids: string[] };
  type NewUserResourceGrant = { resourceID: string; roleID: string };
  type NewUserGrant = {
    scopeType: 'platform' | 'team' | 'project';
    scopeID: string;
    roleID: string;
    resourceGrants: NewUserResourceGrant[];
  };
  type MemberScopeRole = {
    scopeID: string;
    scopeType: 'platform' | 'team' | 'project';
    scopeName: string;
    roleName: string;
  };

  export let accessTab: AccessTab = 'teams';
  export let openCreateTeamRequest = 0;
  export let accessSearch = '';
  export let visibleAccessTeams: Team[] = [];
  export let visibleAccessUsers: User[] = [];
  export let accessLoading = false;
  export let accessLoadError = '';
  export let accessCanCreateTeam = false;
  export let accessCanCreateUser = false;
  export let accessCanManageUsers = false;
  export let teams: Team[] = [];
  export let projects: Project[] = [];
  export let selectedTeamId = '';
  export let selectedProjectId = '';
  export let platformName = '平台';
  export let users: User[] = [];
  export let roles: RoleDefinition[] = [];
  export let bindings: RoleBinding[] = [];
  export let groups: Group[] = [];
  export let groupMembers: Record<string, string[]> = {};
  export let resources: Resource[] = [];
  export let resourceRoles: ResourceRoleDefinition[] = [];
  export let resourceBindings: ResourceRoleBinding[] = [];
  export let accessTeamUsers: Record<string, User[]> = {};
  export let accessProjectUsers: Record<string, User[]> = {};
  export let teamAccessExpanded: Record<string, boolean> = {};
  export let selectedAccessTeamIds: string[] = [];
  export let selectedAccessUserIds: string[] = [];
  export let teamDialogOpen = false;
  export let teamName = '';
  export let teamDescription = '';
  export let teamIcon = 'lucide:UsersRound';
  export let userDialogOpen = false;
  export let editingTeam: Team | null = null;
  export let editTeamName = '';
  export let editTeamDescription = '';
  export let editTeamIcon = '';
  export let editTeamStatus = 'active';
  export let editingUser: User | null = null;
  export let disableTarget: DisableTarget | null = null;
  export let newUserUsername = '';
  export let newUserEmail = '';
  export let newUserPhone = '';
  export let newUserDisplayName = '';
  export let newUserPassword = '';
  export let newUserPasswordMode: 'manual' | 'generated' = 'generated';
  export let createdUserCredentials: {
    username: string;
    password: string;
  } | null = null;
  export let passwordResetCredentials: {
    username: string;
    password: string;
  } | null = null;
  export let newUserGrants: NewUserGrant[] = [];
  export let manageableScopeChoices: Array<{
    id: string;
    type: string;
    name: string;
  }> = [];
  export let scopeChoices: Array<{
    id: string;
    type: string;
    name: string;
    parentId?: string;
  }> = [];
  export let preferredScopeId = '';
  export let currentUser: User | null = null;
  export let isPlatformAdmin = false;
  export let busy = false;
  export let copiedControl:
    'created-password' | 'reset-username' | 'reset-credentials' | null = null;
  export let activeMessage = '';
  export let activeMessageTone: 'success' | 'error' = 'success';

  let accessSearchQuery = '';
  let expandedAccessSection: 'organization' | 'audit' | 'roles' | '' =
    'organization';
  let auditEvents: AuditEvent[] = [];
  let auditTotal = 0;
  let auditLoading = false;
  let auditLoadError = '';
  let auditLoaded = false;
  let teamNameError = '';
  let editTeamNameError = '';
  let newUserUsernameError = '';
  let newUserPasswordError = '';
  let editUserDisplayNameError = '';

  $: organizationReadScopeIDs = scopeChoices
    .filter((scope) =>
      getActorPermissionsAtScope(
        scope.id,
        currentUser?.id,
        isPlatformAdmin,
        roles,
        groups,
        groupMembers,
        bindings,
        scopeChoices
      ).includes('organization:read')
    )
    .map((scope) => scope.id);
  $: platformScope = scopeChoices.find((scope) => scope.type === 'platform');
  $: accessCanViewPlatform =
    isPlatformAdmin ||
    Boolean(
      platformScope &&
      organizationScopeVisible(
        scopeChoices,
        platformScope.id,
        organizationReadScopeIDs
      )
    );
  $: manageableScopeChoices = isPlatformAdmin
    ? scopeChoices
    : scopeChoices.filter((scope) =>
        actorPermissionsAtScope(scope.id).includes('member:grant')
      );
  $: accessSearchQuery = accessSearch.trim().toLowerCase();
  $: visibleAccessUsers = users.filter((user) => {
    if (!accessSearchQuery) return true;
    return [user.display_name, user.username, user.email, user.phone]
      .filter(Boolean)
      .some((value) => value.toLowerCase().includes(accessSearchQuery));
  });
  $: scopedTeamID =
    selectedTeamId ||
    projects.find((project) => project.id === selectedProjectId)?.team_id ||
    '';
  $: visibleAccessTeams = teams.filter((team) => {
    if (scopedTeamID && team.id !== scopedTeamID) return false;
    if (
      !isPlatformAdmin &&
      !organizationScopeVisible(
        scopeChoices,
        team.scope.id,
        organizationReadScopeIDs
      )
    )
      return false;
    if (!accessSearchQuery) return true;
    return [team.name, team.description, team.status].some((value) =>
      value.toLowerCase().includes(accessSearchQuery)
    );
  });
  $: accessVisibleProjects = projects.filter(
    (project) =>
      visibleAccessTeams.some((team) => team.id === project.team_id) &&
      (!selectedProjectId || project.id === selectedProjectId) &&
      (isPlatformAdmin ||
        organizationScopeVisible(
          scopeChoices,
          project.scope.id,
          organizationReadScopeIDs
        ))
  );
  $: accessCanManageUsers = manageableScopeChoices.length > 0;
  $: accessCanCreateUser =
    isPlatformAdmin ||
    accessCanManageUsers ||
    userRoleBindings(currentUser?.id ?? '').some((binding) =>
      ['PlatformAdmin', 'TeamAdmin', 'ProjectAdmin'].includes(binding.role_name)
    );
  $: accessCanCreateTeam = isPlatformAdmin;
  $: accessProjectUsers = Object.fromEntries(
    projects.map((project) => [
      project.id,
      usersAtScopes(
        [
          teams.find((team) => team.id === project.team_id)?.scope.id,
          project.scope.id
        ],
        users,
        bindings,
        groups,
        groupMembers
      )
    ])
  );
  $: accessTeamUsers = Object.fromEntries(
    teams.map((team) => [
      team.id,
      usersAtScopes(
        [
          team.scope.id,
          ...projects
            .filter((project) => project.team_id === team.id)
            .map((project) => project.scope.id)
        ],
        users,
        bindings,
        groups,
        groupMembers
      )
    ])
  );

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 403) return '当前账号没有执行此操作的权限。';
      if (error.status === 401) return '会话已过期，请重新登录。';
      return error.message || fallback;
    }
    return error instanceof Error ? error.message || fallback : fallback;
  }
  async function action(operation: () => Promise<void>) {
    busy = true;
    onError('');
    try {
      await operation();
    } catch (error) {
      onError(describeError(error, '操作失败'));
    } finally {
      busy = false;
    }
  }
  async function loadAccess() {
    accessLoading = true;
    accessLoadError = '';
    try {
      const result = await Promise.allSettled([
        api.users(),
        api.groups(),
        api.roles(),
        api.bindings(),
        api.resourceRoles(),
        api.resourceBindings()
      ]);
      users = result[0].status === 'fulfilled' ? result[0].value : [];
      groups = result[1].status === 'fulfilled' ? result[1].value : [];
      roles = result[2].status === 'fulfilled' ? result[2].value : [];
      bindings = result[3].status === 'fulfilled' ? result[3].value : [];
      resourceRoles = result[4].status === 'fulfilled' ? result[4].value : [];
      resourceBindings =
        result[5].status === 'fulfilled' ? result[5].value : [];
      const memberResults = await Promise.allSettled(
        groups.map((group) => api.groupMembers(group.id))
      );
      groupMembers = Object.fromEntries(
        groups.map((group, index) => [
          group.id,
          memberResults[index]?.status === 'fulfilled'
            ? memberResults[index].value.map((member) => member.user_id)
            : []
        ])
      );
      const rejectedItems = result
        .map((item, index) => (item.status === 'rejected' ? index : -1))
        .filter((index) => index >= 0);
      if (rejectedItems.length > 0) {
        const names = [
          '用户',
          '成员组',
          '角色',
          '角色授权',
          '资源角色',
          '资源授权'
        ];
        accessLoadError = `管理数据加载不完整：${rejectedItems.map((index) => names[index]).join('、')}。`;
      }
      if (newUserGrants.length === 0) resetUserDialog();
    } catch {
      accessLoadError = '成员和角色数据加载失败，请重试。';
    } finally {
      accessLoading = false;
    }
  }
  onMount(() => {
    void loadAccess();
  });

  function toggleAccessSection(section: 'organization' | 'audit' | 'roles') {
    expandedAccessSection = expandedAccessSection === section ? '' : section;
    if (
      section === 'audit' &&
      expandedAccessSection === 'audit' &&
      !auditLoaded
    )
      void loadAuditLogs();
  }

  async function loadAuditLogs() {
    auditLoading = true;
    auditLoadError = '';
    try {
      const result = await api.auditLogs();
      auditEvents = result.items;
      auditTotal = result.total;
      auditLoaded = true;
    } catch (error) {
      auditLoadError = describeError(error, '审计记录加载失败');
    } finally {
      auditLoading = false;
    }
  }

  function auditActorName(userID: string) {
    const user = users.find((item) => item.id === userID);
    return user?.display_name || user?.username || userID || '系统';
  }

  function auditTime(value: string) {
    return value ? new Date(value).toLocaleString() : '—';
  }
  let handledCreateTeamRequest = 0;
  $: if (openCreateTeamRequest > handledCreateTeamRequest) {
    handledCreateTeamRequest = openCreateTeamRequest;
    accessTab = 'teams';
    openTeamDialog();
  }
  function scopeType(id: string) {
    return scopeChoices.find((scope) => scope.id === id)?.type ?? 'scope';
  }
  function scopeName(id: string) {
    return (
      scopeChoices.find((scope) => scope.id === id)?.name ?? id.slice(0, 8)
    );
  }
  function userRoleBindings(userID: string) {
    return getUserRoleBindings(userID, groups, groupMembers, bindings);
  }
  function userRoles(userID: string) {
    return [
      ...new Set(userRoleBindings(userID).map((binding) => binding.role_name))
    ];
  }
  function roleLabel(name: string) {
    const labels: Record<string, string> = {
      PlatformAdmin: '平台管理员',
      PlatformOperator: '平台操作员',
      PlatformViewer: '平台观察员',
      TeamAdmin: '团队管理员',
      TeamOperator: '团队操作员',
      TeamViewer: '团队观察员',
      ProjectAdmin: '项目管理员',
      ProjectOperator: '项目操作员',
      ProjectViewer: '项目观察员',
      ResourceAdmin: '资源管理员',
      ResourceOperator: '资源操作员',
      ResourceViewer: '资源观察员'
    };
    return labels[name] ?? name;
  }
  function grantRoleLabel(name: string) {
    const labels: Record<string, string> = {
      PlatformAdmin: '管理员',
      PlatformOperator: '操作员',
      PlatformViewer: '观察员',
      TeamAdmin: '管理员',
      TeamOperator: '操作员',
      TeamViewer: '观察员',
      ProjectAdmin: '管理员',
      ProjectOperator: '操作员',
      ProjectViewer: '观察员'
    };
    return labels[name] ?? roleLabel(name);
  }
  function grantScopeLabel(type: NewUserGrant['scopeType']) {
    return { platform: '平台', team: '团队', project: '项目' }[type];
  }
  function roleDominatesClient(
    stronger: RoleDefinition,
    weaker: RoleDefinition
  ) {
    return (
      stronger.scope_type === weaker.scope_type &&
      weaker.permissions.every((permission) =>
        stronger.permissions.includes(permission)
      )
    );
  }
  function strongerRole(
    first: RoleDefinition,
    second: RoleDefinition | undefined
  ) {
    if (!second || roleDominatesClient(first, second)) return first;
    if (roleDominatesClient(second, first)) return second;
    return first.permissions.length >= second.permissions.length
      ? first
      : second;
  }
  function resourceRoleDominatesClient(
    stronger: ResourceRoleDefinition,
    weaker: ResourceRoleDefinition
  ) {
    return weaker.permissions.every((permission) =>
      stronger.permissions.includes(permission)
    );
  }
  function strongerResourceRole(
    first: ResourceRoleDefinition,
    second: ResourceRoleDefinition | undefined
  ) {
    if (!second || resourceRoleDominatesClient(first, second)) return first;
    if (resourceRoleDominatesClient(second, first)) return second;
    return first.permissions.length >= second.permissions.length
      ? first
      : second;
  }
  function scopeAlreadyGranted(scopeID: string, currentIndex = -1) {
    return newUserGrants.some(
      (grant, index) => index !== currentIndex && grant.scopeID === scopeID
    );
  }
  function resourceGrantViewerRole(type: string) {
    return (
      (
        {
          platform: 'PlatformViewer',
          team: 'TeamViewer',
          project: 'ProjectViewer'
        } as Record<string, string>
      )[type] ?? ''
    );
  }
  function roleScopeLabel(type: string) {
    return (
      (
        {
          platform: '平台级',
          team: '团队级',
          project: '项目级',
          resource: '资源级'
        } as Record<string, string>
      )[type] ?? type
    );
  }
  function userPermissions(userID: string) {
    const roleIDs = new Set(
      userRoleBindings(userID).map((binding) => binding.role_id)
    );
    return [
      ...new Set(
        roles
          .filter((role) => roleIDs.has(role.id))
          .flatMap((role) => role.permissions.map(String))
      )
    ];
  }
  function permissionDescription(permission: string) {
    const descriptions: Record<string, string> = {
      'organization:read': '查看组织、平台和级别信息',
      'team:manage': '创建、编辑和停用团队',
      'project:manage': '创建、编辑和停用项目',
      'member:grant': '管理用户、用户组和角色授权',
      'resource:read': '查看资源列表、配置和详情',
      'resource:create': '创建资源',
      'resource:update': '编辑资源配置',
      'resource:delete': '删除或停用资源',
      'resource:use': '使用资源执行连接测试或业务调用',
      'engine:manage': '管理 AI 引擎及其级别内的默认 Provider',
      'relation:manage': '管理资源之间的关联关系',
      'diagnosis:start': '启动 AI 诊断',
      'diagnosis:read': '查看诊断记录和结果',
      'inspection:manage': '管理自动巡检策略',
      'inspection:execute': '执行自动巡检',
      'audit:read': '查看审计日志'
    };
    return descriptions[permission] ?? '暂无权限说明';
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
  function grantableRolesForScope(scopeID: string) {
    if (!scopeID) return [];
    const permissions = new Set(actorPermissionsAtScope(scopeID));
    return roles.filter(
      (role) =>
        role.scope_type === scopeType(scopeID) &&
        role.permissions.every((permission) => permissions.has(permission))
    );
  }
  function canManageTeam(_team: Team) {
    return isPlatformAdmin;
  }
  function canManageUser(user: User) {
    return (
      user.can_manage ?? (accessCanManageUsers && user.id !== currentUser?.id)
    );
  }
  function userScopeNames(userID: string) {
    return [
      ...new Set(
        userRoleBindings(userID).map((binding) => scopeName(binding.scope_id))
      )
    ];
  }
  function memberScopeRoles(
    userID: string,
    selection: { kind: 'platform' | 'team' | 'project'; id: string }
  ): MemberScopeRole[] {
    const allowedScopeIDs = new Set<string>();
    if (selection.kind === 'project') {
      allowedScopeIDs.add(selection.id);
    } else if (selection.kind === 'team') {
      allowedScopeIDs.add(selection.id);
      projects
        .filter((project) => project.team_id === selection.id)
        .forEach((project) => allowedScopeIDs.add(project.scope.id));
    } else {
      scopeChoices.forEach((scope) => allowedScopeIDs.add(scope.id));
    }
    const seen = new Set<string>();
    const typeOrder: Record<string, number> = {
      platform: 0,
      team: 1,
      project: 2
    };
    return getUserRoleBindings(userID, groups, groupMembers, bindings)
      .filter((binding) => allowedScopeIDs.has(binding.scope_id))
      .filter((binding) => {
        const key = `${binding.scope_id}:${binding.role_id}`;
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      })
      .map((binding) => ({
        scopeID: binding.scope_id,
        scopeType: scopeType(binding.scope_id) as MemberScopeRole['scopeType'],
        scopeName: memberScopeName(binding.scope_id),
        roleName: roleLabel(binding.role_name)
      }))
      .sort((left, right) => {
        const leftType =
          scopeChoices.find((scope) => scope.id === left.scopeID)?.type ?? '';
        const rightType =
          scopeChoices.find((scope) => scope.id === right.scopeID)?.type ?? '';
        return (
          (typeOrder[leftType] ?? 9) - (typeOrder[rightType] ?? 9) ||
          left.scopeName.localeCompare(right.scopeName) ||
          left.roleName.localeCompare(right.roleName)
        );
      });
  }
  function memberScopeName(scopeID: string) {
    const scope = scopeChoices.find((item) => item.id === scopeID);
    if (!scope) return scopeID;
    if (scope.type !== 'project')
      return scope.type === 'platform' ? platformName : scope.name;
    const project = projects.find((item) => item.scope.id === scopeID);
    const team = project && teams.find((item) => item.id === project.team_id);
    return team ? `${team.name} / ${project.name}` : scope.name;
  }
  function resourceVisibleToScope(
    viewerScopeID: string,
    resourceScopeID: string
  ) {
    return isResourceVisibleToScope(
      scopeChoices,
      viewerScopeID,
      resourceScopeID
    );
  }
  function viewerResourceRoleAllowed(resourceRole: ResourceRoleDefinition) {
    return isViewerResourceRoleAllowed(resourceRole);
  }

  export let onNotice: (message: string) => void = () => {};
  export let onError: (message: string) => void = () => {};

  function toggleTeamAccess(id: string) {
    teamAccessExpanded = {
      ...teamAccessExpanded,
      [id]: !teamAccessExpanded[id]
    };
  }
  function requestDisable(kind: DisableTarget['kind'], ids: string[]) {
    if (ids.length > 0) disableTarget = { kind, ids: [...ids] };
  }
  async function confirmDisable() {
    if (!disableTarget) return;
    const target = disableTarget;
    await action(async () => {
      if (target.kind === 'team') {
        const updated = await Promise.all(
          target.ids.map((id) => api.updateTeam(id, { status: 'disabled' }))
        );
        const byID = new Map(updated.map((team) => [team.id, team]));
        teams = teams.map((team) => byID.get(team.id) ?? team);
        selectedAccessTeamIds = [];
      } else {
        const updated = await Promise.all(
          target.ids.map((id) => api.updateUser(id, { status: 'disabled' }))
        );
        const byID = new Map(updated.map((user) => [user.id, user]));
        users = users.map((user) => byID.get(user.id) ?? user);
        selectedAccessUserIds = [];
      }
      disableTarget = null;
      onNotice(
        `${target.ids.length} 个${target.kind === 'team' ? '团队' : '用户'}已禁用`
      );
    });
  }
  function resetUserDialog() {
    newUserUsername = '';
    newUserEmail = '';
    newUserPhone = '';
    newUserDisplayName = '';
    newUserPassword = '';
    newUserPasswordMode = 'generated';
    createdUserCredentials = null;
    newUserUsernameError = '';
    newUserPasswordError = '';
    const preferred =
      scopeChoices.find((scope) => scope.id === preferredScopeId) ??
      manageableScopeChoices[0];
    newUserGrants = preferred
      ? [
          {
            scopeType: preferred.type as NewUserGrant['scopeType'],
            scopeID: preferred.id,
            roleID: '',
            resourceGrants: []
          }
        ]
      : [];
  }
  function addNewUserGrant() {
    const scope = manageableScopeChoices.find(
      (item) => !scopeAlreadyGranted(item.id)
    );
    if (scope)
      newUserGrants = [
        ...newUserGrants,
        {
          scopeType: scope.type as NewUserGrant['scopeType'],
          scopeID: scope.id,
          roleID: '',
          resourceGrants: []
        }
      ];
  }
  function updateNewUserGrant(index: number, updates: Partial<NewUserGrant>) {
    newUserGrants = newUserGrants.map((grant, i) =>
      i === index ? { ...grant, ...updates } : grant
    );
  }
  function chooseNewUserGrantType(
    index: number,
    type: NewUserGrant['scopeType']
  ) {
    const scope = newUserGrantScopes(type, index)[0];
    updateNewUserGrant(index, {
      scopeType: type,
      scopeID: scope?.id ?? '',
      roleID: '',
      resourceGrants: []
    });
  }
  function removeNewUserGrant(index: number) {
    newUserGrants = newUserGrants.filter((_, i) => i !== index);
  }
  function newUserGrantScopes(
    type: NewUserGrant['scopeType'],
    currentIndex = -1
  ) {
    return manageableScopeChoices.filter(
      (scope) =>
        scope.type === type && !scopeAlreadyGranted(scope.id, currentIndex)
    );
  }
  function newUserGrantRoles(grant: NewUserGrant) {
    return grantableRolesForScope(grant.scopeID);
  }
  function newUserGrantIsScopeViewer(grant: NewUserGrant) {
    return (
      roles.find((role) => role.id === grant.roleID)?.name ===
      resourceGrantViewerRole(grant.scopeType)
    );
  }
  function resourceAlreadyGranted(
    grant: NewUserGrant,
    resourceID: string,
    currentIndex = -1
  ) {
    return grant.resourceGrants.some(
      (item, index) => index !== currentIndex && item.resourceID === resourceID
    );
  }
  function newUserGrantResources(grant: NewUserGrant, currentIndex = -1) {
    return resources.filter(
      (resource) =>
        resourceVisibleToScope(grant.scopeID, resource.scope_id) &&
        resource.status === 'active' &&
        !resourceAlreadyGranted(grant, resource.id, currentIndex)
    );
  }
  function newUserGrantResourceRoles(grant: NewUserGrant) {
    return resourceRoles.filter(
      (role) =>
        viewerResourceRoleAllowed(role) &&
        role.permissions.every((permission) =>
          actorPermissionsAtScope(grant.scopeID).includes(String(permission))
        )
    );
  }
  function userResourceGrantsForScope(scopeID: string) {
    const strongestByResource = new Map<string, ResourceRoleBinding>();
    for (const binding of resourceBindings) {
      if (
        binding.subject_type !== 'user' ||
        binding.subject_id !== editingUser?.id
      )
        continue;
      const resource = resources.find(
        (item) => item.id === binding.resource_id
      );
      if (!resource || !resourceVisibleToScope(scopeID, resource.scope_id))
        continue;
      const current = strongestByResource.get(binding.resource_id);
      const nextRole = resourceRoles.find(
        (role) => role.id === binding.role_id
      );
      const currentRole =
        current && resourceRoles.find((role) => role.id === current.role_id);
      if (
        !current ||
        (nextRole && strongerResourceRole(nextRole, currentRole) === nextRole)
      ) {
        strongestByResource.set(binding.resource_id, binding);
      }
    }
    return [...strongestByResource.values()].map((binding) => ({
      resourceID: binding.resource_id,
      roleID: binding.role_id
    }));
  }
  function addNewUserResourceGrant(index: number) {
    const grant = newUserGrants[index];
    if (grant)
      updateNewUserGrant(index, {
        resourceGrants: [
          ...grant.resourceGrants,
          { resourceID: '', roleID: '' }
        ]
      });
  }
  function updateNewUserResourceGrant(
    gi: number,
    ri: number,
    updates: Partial<NewUserResourceGrant>
  ) {
    const grant = newUserGrants[gi];
    if (grant)
      updateNewUserGrant(gi, {
        resourceGrants: grant.resourceGrants.map((item, i) =>
          i === ri ? { ...item, ...updates } : item
        )
      });
  }
  function removeNewUserResourceGrant(gi: number, ri: number) {
    const grant = newUserGrants[gi];
    if (grant)
      updateNewUserGrant(gi, {
        resourceGrants: grant.resourceGrants.filter((_, i) => i !== ri)
      });
  }
  function updateNewUserUsername(value: string) {
    if (!newUserDisplayName || newUserDisplayName === newUserUsername)
      newUserDisplayName = value;
    newUserUsername = value;
    if (value.trim()) newUserUsernameError = '';
  }
  function openTeamDialog() {
    teamName = '';
    teamDescription = '';
    teamIcon = 'lucide:UsersRound';
    teamNameError = '';
    teamDialogOpen = true;
  }
  function openEditTeam(team: Team) {
    editingTeam = team;
    editTeamName = team.name;
    editTeamDescription = team.description;
    editTeamIcon = team.icon;
    editTeamStatus = team.status;
    editTeamNameError = '';
  }
  function openEditUser(user: User) {
    editingUser = user;
    newUserUsername = user.username;
    newUserEmail = user.email;
    newUserPhone = user.phone;
    newUserDisplayName = user.display_name || user.username;
    newUserPassword = '';
    newUserPasswordMode = 'generated';
    createdUserCredentials = null;
    passwordResetCredentials = null;
    newUserUsernameError = '';
    newUserPasswordError = '';
    const direct = bindings.filter(
      (binding) =>
        binding.subject_type === 'user' && binding.subject_id === user.id
    );
    const strongestByScope = new Map<string, RoleBinding>();
    for (const binding of direct) {
      if (
        !manageableScopeChoices.some((scope) => scope.id === binding.scope_id)
      )
        continue;
      const current = strongestByScope.get(binding.scope_id);
      const nextRole = roles.find((role) => role.id === binding.role_id);
      const currentRole =
        current && roles.find((role) => role.id === current.role_id);
      if (
        !current ||
        (nextRole && strongerRole(nextRole, currentRole) === nextRole)
      ) {
        strongestByScope.set(binding.scope_id, binding);
      }
    }
    newUserGrants = [...strongestByScope.values()].map((binding) => {
      const scope = manageableScopeChoices.find(
        (item) => item.id === binding.scope_id
      )!;
      const isViewer =
        roles.find((role) => role.id === binding.role_id)?.name ===
        resourceGrantViewerRole(scope.type);
      return {
        scopeType: scope.type as NewUserGrant['scopeType'],
        scopeID: scope.id,
        roleID: binding.role_id,
        resourceGrants: isViewer ? userResourceGrantsForScope(scope.id) : []
      };
    });
    if (newUserGrants.length === 0) {
      const preferred =
        manageableScopeChoices.find((scope) =>
          direct.some((binding) => binding.scope_id === scope.id)
        ) ?? manageableScopeChoices[0];
      newUserGrants = preferred
        ? [
            {
              scopeType: preferred.type as NewUserGrant['scopeType'],
              scopeID: preferred.id,
              roleID: '',
              resourceGrants: []
            }
          ]
        : [];
    }
  }
  async function createTeam() {
    teamNameError = teamName.trim() ? '' : '请填写团队名称。';
    if (teamNameError) return;
    await action(async () => {
      const created = await api.createTeam({
        name: teamName,
        description: teamDescription,
        icon: teamIcon,
        labels: {}
      });
      teams = [...teams, created];
      teamDialogOpen = false;
      onNotice(`团队“${created.name}”已创建`);
    });
  }
  async function createUser() {
    newUserUsernameError = newUserUsername.trim() ? '' : '请填写用户名。';
    newUserPasswordError =
      newUserPasswordMode === 'manual'
        ? newUserPassword.length >= 8
          ? ''
          : newUserPassword
            ? '密码至少需要 8 位。'
            : '请填写一次性密码。'
        : '';
    if (newUserUsernameError || newUserPasswordError) return;
    await action(async () => {
      const result = await api.createUser({
        username: newUserUsername,
        email: newUserEmail,
        phone: newUserPhone,
        display_name: newUserDisplayName,
        password: newUserPassword,
        password_mode: newUserPasswordMode,
        grants: newUserGrants.map((grant) => ({
          scope_id: grant.scopeID,
          role_id: grant.roleID,
          resource_grants: grant.resourceGrants.map((item) => ({
            resource_id: item.resourceID,
            role_id: item.roleID
          }))
        }))
      });
      users = [...users, result.user];
      bindings = [...bindings, ...result.bindings];
      createdUserCredentials = {
        username: result.user.username,
        password: result.one_time_password
      };
      onNotice(
        `用户“${result.user.display_name || result.user.username}”已创建并完成授权`
      );
    });
  }
  async function saveTeam() {
    if (!editingTeam) return;
    editTeamNameError = editTeamName.trim() ? '' : '请填写团队名称。';
    if (editTeamNameError) return;
    await action(async () => {
      const updated = await api.updateTeam(editingTeam!.id, {
        name: editTeamName,
        description: editTeamDescription,
        icon: editTeamIcon,
        status: editTeamStatus
      });
      teams = teams.map((team) => (team.id === updated.id ? updated : team));
      editingTeam = null;
      onNotice(`团队“${updated.name}”已更新`);
    });
  }
  async function saveUser() {
    editUserDisplayNameError = newUserDisplayName.trim()
      ? ''
      : '请填写显示名。';
    const grantsValid =
      newUserGrants.length > 0 &&
      newUserGrants.every(
        (grant) =>
          grant.scopeID &&
          grant.roleID &&
          grant.resourceGrants.every((item) => item.resourceID && item.roleID)
      );
    if (!editingUser || editUserDisplayNameError || !grantsValid) return;
    await action(async () => {
      const userID = editingUser!.id;
      const updatedUser = await api.updateUser(userID, {
        display_name: newUserDisplayName,
        email: newUserEmail,
        phone: newUserPhone
      });
      const manageableIDs = new Set(
        manageableScopeChoices.map((scope) => scope.id)
      );
      const existingBindings = bindings.filter(
        (binding) =>
          binding.subject_type === 'user' && binding.subject_id === userID
      );
      const desiredBindings = newUserGrants.flatMap((grant) =>
        grant.scopeID && grant.roleID
          ? [{ scopeID: grant.scopeID, roleID: grant.roleID }]
          : []
      );
      const desiredBindingKeys = new Set(
        desiredBindings.map((binding) => `${binding.scopeID}:${binding.roleID}`)
      );
      for (const binding of existingBindings) {
        if (
          manageableIDs.has(binding.scope_id) &&
          !desiredBindingKeys.has(`${binding.scope_id}:${binding.role_id}`)
        )
          await api.deleteBinding(binding.id);
      }
      const createdBindings: RoleBinding[] = [];
      for (const desired of desiredBindings) {
        const existing = existingBindings.find(
          (binding) =>
            binding.scope_id === desired.scopeID &&
            binding.role_id === desired.roleID
        );
        if (!existing)
          createdBindings.push(
            await api.createBinding({
              subject_type: 'user',
              subject_id: userID,
              role_id: desired.roleID,
              scope_id: desired.scopeID
            })
          );
      }
      const desiredResources = newUserGrants.flatMap((grant) =>
        grant.resourceGrants
          .filter((item) => item.resourceID && item.roleID)
          .map((item) => ({ resourceID: item.resourceID, roleID: item.roleID }))
      );
      const existingResources = resourceBindings.filter(
        (binding) =>
          binding.subject_type === 'user' && binding.subject_id === userID
      );
      const desiredResourceKeys = new Set(
        desiredResources.map((item) => `${item.resourceID}:${item.roleID}`)
      );
      for (const binding of existingResources) {
        const resource = resources.find(
          (item) => item.id === binding.resource_id
        );
        const manageableResource = resource
          ? [...manageableIDs].some((scopeID) =>
              resourceVisibleToScope(scopeID, resource.scope_id)
            )
          : false;
        if (
          manageableResource &&
          !desiredResourceKeys.has(`${binding.resource_id}:${binding.role_id}`)
        )
          await api.deleteResourceBinding(binding.id);
      }
      const createdResources: ResourceRoleBinding[] = [];
      for (const desired of desiredResources) {
        const existing = existingResources.find(
          (binding) =>
            binding.resource_id === desired.resourceID &&
            binding.role_id === desired.roleID
        );
        if (!existing)
          createdResources.push(
            await api.createResourceBinding({
              subject_type: 'user',
              subject_id: userID,
              role_id: desired.roleID,
              resource_id: desired.resourceID
            })
          );
      }
      users = users.map((user) =>
        user.id === updatedUser.id ? updatedUser : user
      );
      bindings = [
        ...bindings.filter(
          (binding) =>
            !existingBindings.some(
              (old) =>
                old.id === binding.id &&
                manageableIDs.has(old.scope_id) &&
                !desiredBindingKeys.has(`${old.scope_id}:${old.role_id}`)
            )
        ),
        ...createdBindings
      ];
      resourceBindings = [
        ...resourceBindings.filter(
          (binding) =>
            !existingResources.some(
              (old) =>
                old.id === binding.id &&
                !desiredResourceKeys.has(`${old.resource_id}:${old.role_id}`)
            )
        ),
        ...createdResources
      ];
      editingUser = null;
      onNotice('用户信息和授权已更新');
    });
  }
  async function resetManagedUserPassword() {
    if (!editingUser) return;
    await action(async () => {
      const result = await api.resetUserPassword(editingUser!.id);
      passwordResetCredentials = {
        username: editingUser!.username,
        password: result.one_time_password
      };
      onNotice('已生成一次性密码');
    });
  }
  async function copyOneTimePassword() {
    if (!createdUserCredentials) return;
    try {
      await navigator.clipboard.writeText(createdUserCredentials.password);
      copiedControl = 'created-password';
      onNotice('一次性密码已复制');
    } catch {
      onError('无法访问剪贴板，请手动复制。');
    }
  }
  async function copyPasswordResetCredentials(includePassword: boolean) {
    if (!passwordResetCredentials) return;
    try {
      await navigator.clipboard.writeText(
        includePassword
          ? `用户名：${passwordResetCredentials.username}\n一次性密码：${passwordResetCredentials.password}`
          : passwordResetCredentials.username
      );
      copiedControl = includePassword ? 'reset-credentials' : 'reset-username';
    } catch {
      onError('无法访问剪贴板，请手动复制。');
    }
  }
</script>

<section class="access-page">
  <section class="access-accordion-panel panel">
    <button
      class="access-accordion-heading"
      type="button"
      aria-expanded={expandedAccessSection === 'organization'}
      on:click={() => toggleAccessSection('organization')}
    >
      <span
        ><strong
          >组织 <small class="access-heading-code">ORGANIZATION</small></strong
        ><small>团队、项目与成员管理</small></span
      >
      <ChevronDown
        size={17}
        class={expandedAccessSection === 'organization' ? 'expanded' : ''}
      />
    </button>
    {#if expandedAccessSection === 'organization'}
      <div class="access-accordion-content access-organization-content">
        <AccessManagementWorkbench
          bind:selectedAccessUserIds
          {visibleAccessTeams}
          {visibleAccessUsers}
          projects={accessVisibleProjects}
          {selectedTeamId}
          {selectedProjectId}
          canViewPlatform={accessCanViewPlatform}
          {platformName}
          {accessTeamUsers}
          {accessProjectUsers}
          {teamAccessExpanded}
          {accessLoading}
          {accessLoadError}
          {accessCanCreateTeam}
          {accessCanCreateUser}
          {busy}
          {currentUser}
          onAddTeam={openTeamDialog}
          onAddUser={() => {
            resetUserDialog();
            userDialogOpen = true;
          }}
          onEditTeam={openEditTeam}
          onEditUser={openEditUser}
          onDisable={requestDisable}
          onToggleTeam={toggleTeamAccess}
          onReload={loadAccess}
          {canManageTeam}
          {canManageUser}
          userScopeRoles={memberScopeRoles}
        />
      </div>
    {/if}
  </section>

  <section class="access-accordion-panel panel">
    <button
      class="access-accordion-heading"
      type="button"
      aria-expanded={expandedAccessSection === 'audit'}
      on:click={() => toggleAccessSection('audit')}
    >
      <span
        ><strong>审计 <small class="access-heading-code">AUDIT</small></strong
        ><small>查看最近的安全与管理操作记录</small></span
      >
      <ChevronDown
        size={17}
        class={expandedAccessSection === 'audit' ? 'expanded' : ''}
      />
    </button>
    {#if expandedAccessSection === 'audit'}
      <div class="access-accordion-content access-audit-content">
        <div class="access-audit-toolbar">
          <span
            >{auditLoaded
              ? `共 ${auditTotal} 条记录 · 显示最近 ${auditEvents.length} 条`
              : '最近操作记录'}</span
          ><button
            class="secondary"
            type="button"
            on:click={loadAuditLogs}
            disabled={auditLoading}><RefreshCw size={14} />刷新</button
          >
        </div>
        {#if auditLoading}<div class="access-state" aria-live="polite">
            正在加载审计记录...
          </div>
        {:else if auditLoadError}<div class="access-state access-error">
            {auditLoadError}<button
              class="secondary"
              type="button"
              on:click={loadAuditLogs}>重试</button
            >
          </div>
        {:else}<div class="access-audit-list">
            <div class="access-audit-row access-audit-head">
              <span>时间</span><span>操作者</span><span>操作</span><span
                >对象</span
              ><span>结果</span><span>详情</span>
            </div>
            {#each auditEvents as event}
              <article class="access-audit-row">
                <time>{auditTime(event.created_at)}</time><span
                  >{auditActorName(event.actor_user_id)}</span
                ><span
                  ><strong>{event.action}</strong><small
                    >{event.scope_id
                      ? scopeName(event.scope_id)
                      : '平台'}</small
                  ></span
                ><span
                  ><strong>{event.target_type || '—'}</strong><small
                    >{event.target_id || '—'}</small
                  ></span
                ><span
                  class="status-label {event.result === 'success'
                    ? 'active'
                    : 'disabled'}">{event.result || '—'}</span
                ><span
                  class="access-audit-details"
                  title={JSON.stringify(event.details ?? {})}
                  >{JSON.stringify(event.details ?? {}) || '—'}</span
                >
              </article>
            {:else}<div class="access-state">暂无可查看的审计记录。</div>{/each}
          </div>{/if}
      </div>
    {/if}
  </section>

  <section class="access-accordion-panel panel">
    <button
      class="access-accordion-heading"
      type="button"
      aria-expanded={expandedAccessSection === 'roles'}
      on:click={() => toggleAccessSection('roles')}
    >
      <span
        ><strong>角色 <small class="access-heading-code">ROLE</small></strong
        ><small>角色模板及其权限范围</small></span
      >
      <ChevronDown
        size={17}
        class={expandedAccessSection === 'roles' ? 'expanded' : ''}
      />
    </button>
    {#if expandedAccessSection === 'roles'}
      <div class="access-accordion-content">
        <div class="access-role-cards">
          {#each roles as role}
            <article class="role-catalog-item">
              <div class="role-card-heading">
                <div>
                  <strong>{roleLabel(role.name)}</strong><small
                    >{roleScopeLabel(role.scope_type)}{role.builtin
                      ? ' · 内置'
                      : ''}</small
                  >
                </div>
                <span class="role-permission-count"
                  >{role.permissions.length} 项权限</span
                >
              </div>
              <div class="permission-list">
                {#each role.permissions as permission}<span
                    data-tooltip={permissionDescription(String(permission))}
                    title={permissionDescription(String(permission))}
                    >{permission}</span
                  >{/each}
              </div>
            </article>
          {:else}<div class="access-state">
              当前账号没有角色目录查看权限。
            </div>{/each}
        </div>
      </div>
    {/if}
  </section>
  <!-- legacy tabbed management surface retained below for reference; the unified workbench above owns this page. {#if false}
          <nav class="access-view-switcher" aria-label="权限管理视图">
            <button type="button" class:active={accessTab === 'teams'} on:click={() => (accessTab = 'teams')}>团队</button>
            <button type="button" class:active={accessTab === 'users'} on:click={() => (accessTab = 'users')}>用户</button>
            <button type="button" class:active={accessTab === 'roles'} on:click={() => (accessTab = 'roles')}>角色</button>
          </nav>
          <section class="panel access-workbench">
            {#if accessTab !== 'roles'}
              <div class="access-filterbar">
                <div class="access-filter-copy">
                  <div>
                    <h2>{accessTab === 'teams' ? '团队列表' : '用户列表'}</h2>
                    <p>
                      {accessTab === 'teams'
                        ? '展开团队可查看其成员与项目。仅平台管理员可添加、编辑或删除团队。'
                        : '角色包含直接授权和成员组继承授权；管理员仅可授权、删除或重置其他用户的密码。'}
                    </p>
                  </div>
                  <span class="access-count"
                    >{accessTab === 'teams'
                      ? visibleAccessTeams.length + ' 个团队'
                      : visibleAccessUsers.length + ' 个用户'}</span
                  >
                </div>
                <label class="access-search">
                  <Search size={15} aria-hidden="true" />
                  <span class="sr-only"
                    >搜索{accessTab === 'teams' ? '团队' : '用户'}</span
                  ><input
                    bind:value={accessSearch}
                    placeholder={accessTab === 'teams'
                      ? '搜索团队名称、编码或状态'
                      : '搜索姓名、用户名、邮箱或手机号'}
                  />
                </label>
                <div class="access-heading-actions">
                  {#if accessTab === 'teams'}
                    <button
                      class="secondary danger-action"
                      type="button"
                      disabled={selectedAccessTeamIds.length === 0 || busy}
                      data-tooltip={selectedAccessTeamIds.length === 0
                        ? '请先选择可管理的团队'
                        : '批量禁用所选团队'}
                      on:click={() =>
                        requestDisable('team', selectedAccessTeamIds)}
                      ><Trash2 size={15} aria-hidden="true" />批量删除</button
                    >
                    {#if accessCanCreateTeam}<button
                        class="primary"
                        type="button"
                        on:click={openTeamDialog}
                        ><Plus size={15} aria-hidden="true" />添加团队</button
                      >{/if}
                  {:else if accessTab === 'users'}
                    <button
                      class="secondary danger-action"
                      type="button"
                      disabled={selectedAccessUserIds.length === 0 || busy}
                      data-tooltip={selectedAccessUserIds.length === 0
                        ? '请先选择可管理的用户'
                        : '批量禁用所选用户'}
                      on:click={() =>
                        requestDisable('user', selectedAccessUserIds)}
                      ><Trash2 size={15} aria-hidden="true" />批量删除</button
                    >
                    {#if accessCanCreateUser}<button
                        class="primary"
                        type="button"
                        on:click={() => {
                          resetUserDialog();
                          userDialogOpen = true;
                        }}><Plus size={15} aria-hidden="true" />添加用户</button
                      >{/if}
                  {/if}
                </div>
              </div>
            {/if}
            {#if accessLoading}
              <div class="access-state" aria-live="polite">
                正在加载管理数据...
              </div>
            {:else if accessLoadError}
              <div class="access-state access-error">
                <span>{accessLoadError}</span><button
                  class="secondary"
                  type="button"
                  on:click={loadAccess}>重试</button
                >
              </div>
            {:else if accessTab === 'teams'}
              <div class="access-table access-team-table">
                <div class="access-table-header">
                  <input
                    type="checkbox"
                    aria-label="选择全部可管理团队"
                    checked={visibleAccessTeams.some(canManageTeam) &&
                      visibleAccessTeams
                        .filter(canManageTeam)
                        .every((team) =>
                          selectedAccessTeamIds.includes(team.id)
                        )}
                    on:change={(event) => {
                      selectedAccessTeamIds = event.currentTarget.checked
                        ? visibleAccessTeams
                            .filter(canManageTeam)
                            .map((team) => team.id)
                        : [];
                    }}
                  /><span>团队</span><span>成员</span><span>项目</span><span
                    >状态</span
                  ><span>操作</span>
                </div>
                {#each visibleAccessTeams as team}
                  {@const teamMembers = accessTeamUsers[team.id] ?? []}
                  {@const teamProjects = projects.filter(
                    (project) => project.team_id === team.id
                  )}
                  <article class="access-record">
                    <div class="access-table-row">
                      <input
                        type="checkbox"
                        aria-label={`选择团队 ${team.name}`}
                        disabled={!canManageTeam(team) ||
                          team.status !== 'active'}
                        bind:group={selectedAccessTeamIds}
                        value={team.id}
                      />
                      <button
                        class="access-team-trigger"
                        type="button"
                        aria-expanded={teamAccessExpanded[team.id]}
                        on:click={() => toggleTeamAccess(team.id)}
                      >
                        <span class="entity-icon team-icon"
                          ><IconValue value={team.icon} size={17} /></span
                        ><span
                          ><strong>{team.name}</strong><small>{team.description || '团队'}</small
                          ></span
                        ><ChevronDown
                          size={16}
                          class={teamAccessExpanded[team.id]
                            ? 'expanded'
                            : undefined}
                          aria-hidden="true"
                        />
                      </button>
                      <span class="access-metric"
                        ><strong>{teamMembers.length}</strong><small
                          >位可见成员</small
                        ></span
                      ><span class="access-metric"
                        ><strong>{teamProjects.length}</strong><small
                          >个关联项目</small
                        ></span
                      ><span class="status-label {team.status}"
                        >{team.status === 'active' ? '启用' : '已禁用'}</span
                      >
                      <div class="access-row-actions">
                        {#if canManageTeam(team)}
                          <button
                            class="icon-button"
                            type="button"
                            aria-label={`编辑团队 ${team.name}`}
                            data-tooltip="编辑团队"
                            on:click={() => openEditTeam(team)}
                            ><Pencil size={15} aria-hidden="true" /></button
                          ><button
                            class="icon-button danger-action"
                            type="button"
                            aria-label={`删除团队 ${team.name}`}
                            data-tooltip="禁用团队"
                            disabled={team.status !== 'active'}
                            on:click={() => requestDisable('team', [team.id])}
                            ><Trash2 size={15} aria-hidden="true" /></button
                          >
                        {:else}<span class="read-only-label">只读</span>{/if}
                      </div>
                    </div>
                    {#if teamAccessExpanded[team.id]}
                      <div class="team-directory-detail">
                        <div class="directory-subsection">
                          <span class="directory-label">成员</span>
                          {#each teamMembers as member}<button
                              class="directory-user"
                              type="button"
                              on:click={() => {
                                accessTab = 'users';
                                accessSearch = member.username;
                              }}
                              ><span class="avatar tiny-avatar"
                                >{(member.display_name || member.username)
                                  .slice(0, 1)
                                  .toUpperCase()}</span
                              ><span
                                ><strong
                                  >{member.display_name ||
                                    member.username}</strong
                                ><small
                                  >{userRoles(member.id)
                                    .map(roleLabel)
                                    .join(' · ') || '未分配角色'}</small
                                ></span
                              ></button
                            >{:else}<span class="directory-empty"
                              >当前账号看不到该团队的成员</span
                            >{/each}
                        </div>
                        <div class="directory-subsection">
                          <span class="directory-label">项目</span>
                          {#each teamProjects as project}<div
                              class="directory-project"
                            >
                              <span class="project-dot"></span><span
                                ><strong>{project.name}</strong><small
                                  >{project.code} · {project.status}</small
                                ></span
                              >
                            </div>{:else}<span class="directory-empty"
                              >暂无项目</span
                            >{/each}
                        </div>
                      </div>
                    {/if}
                  </article>
                {:else}<div class="access-state">
                    没有匹配的团队。请清除搜索条件后重试。
                  </div>{/each}
              </div>
            {:else if accessTab === 'users'}
              <div class="access-table access-user-table">
                <div class="access-table-header">
                  <input
                    type="checkbox"
                    aria-label="选择全部可管理用户"
                    checked={visibleAccessUsers.some(canManageUser) &&
                      visibleAccessUsers
                        .filter(canManageUser)
                        .every((user) =>
                          selectedAccessUserIds.includes(user.id)
                        )}
                    on:change={(event) => {
                      selectedAccessUserIds = event.currentTarget.checked
                        ? visibleAccessUsers
                            .filter(canManageUser)
                            .map((user) => user.id)
                        : [];
                    }}
                  /><span>用户</span><span>授权范围</span><span>角色与权限</span
                  ><span>状态</span><span>操作</span>
                </div>
                {#each visibleAccessUsers as user}
                  <article class="access-table-row access-user-row">
                    <input
                      type="checkbox"
                      aria-label={`选择用户 ${user.display_name || user.username}`}
                      disabled={!canManageUser(user) ||
                        user.status !== 'active'}
                      bind:group={selectedAccessUserIds}
                      value={user.id}
                    />
                    <div class="access-user-main">
                      <span class="avatar access-avatar"
                        >{(user.display_name || user.username)
                          .slice(0, 1)
                          .toUpperCase()}</span
                      ><span
                        ><strong>{user.display_name || user.username}</strong
                        ><small
                          >@{user.username}{user.email
                            ? ` · ${user.email}`
                            : ''}</small
                        ></span
                      >
                    </div>
                    <div class="access-user-scopes">
                      {#each userScopeNames(user.id).slice(0, 2) as scope}<span
                          >{scope}</span
                        >{:else}<span class="permission-empty"
                          >无可见 Scope</span
                        >{/each}
                    </div>
                    <div class="access-user-auth">
                      <div class="access-user-roles">
                        {#each userRoles(user.id) as role}<span
                            class="role-chip">{roleLabel(role)}</span
                          >{:else}<span class="role-chip muted-chip"
                            >未分配角色</span
                          >{/each}
                      </div>
                      <div class="access-user-permissions">
                        {#each userPermissions(user.id).slice(0, 3) as permission}<span
                            data-tooltip={permissionDescription(permission)}
                            title={permissionDescription(permission)}
                            >{permission}</span
                          >{:else}<span class="permission-empty">暂无权限</span
                          >{/each}{#if userPermissions(user.id).length > 3}<span
                            >+{userPermissions(user.id).length - 3}</span
                          >{/if}
                      </div>
                    </div>
                    <span class="status-label {user.status}"
                      >{user.status === 'active'
                        ? '启用'
                        : user.status === 'locked'
                          ? '已锁定'
                          : '已禁用'}</span
                    >
                    <div class="access-row-actions">
                      {#if canManageUser(user)}
                        <button
                          class="icon-button"
                          type="button"
                          aria-label={`编辑用户 ${user.display_name || user.username}`}
                          data-tooltip="编辑用户与授权"
                          on:click={() => openEditUser(user)}
                          ><Pencil size={15} aria-hidden="true" /></button
                        ><button
                          class="icon-button danger-action"
                          type="button"
                          aria-label={`删除用户 ${user.display_name || user.username}`}
                          data-tooltip="禁用用户"
                          disabled={user.status !== 'active'}
                          on:click={() => requestDisable('user', [user.id])}
                          ><Trash2 size={15} aria-hidden="true" /></button
                        >
                      {:else}<span class="read-only-label"
                          >{user.id === currentUser?.id
                            ? '当前账号'
                            : '只读'}</span
                        >{/if}
                    </div>
                  </article>
                {:else}<div class="access-state">
                    没有匹配的用户，或当前账号没有成员查看权限。
                  </div>{/each}
              </div>
            {:else}
              <div class="role-catalog-grid">
                <div class="role-catalog-toolbar">
                  <div>
                    <h2>角色权限</h2>
                    <p>
                      角色权限决定用户在对应 Scope
                      内可以执行的操作；仅管理员可为其他用户授权。
                    </p>
                  </div>
                  <span class="access-role-boundary"
                    ><ShieldCheck
                      size={16}
                      aria-hidden="true"
                    />授权时只显示当前账号可完整授予的角色</span
                  >
                </div>
                {#each roles as role}<article class="role-catalog-item">
                    <div>
                      <strong>{roleLabel(role.name)}</strong><small
                        >{roleScopeLabel(role.scope_type)}{role.builtin
                          ? ' · 内置'
                          : ''}</small
                      >
                    </div>
                    <div class="permission-list">
                      {#each role.permissions as permission}<span
                          data-tooltip={permissionDescription(
                            String(permission)
                          )}
                          title={permissionDescription(String(permission))}
                          >{permission}</span
                        >{/each}
                    </div>
                  </article>{:else}<div class="access-state">
                    当前账号没有角色目录查看权限。
                  </div>{/each}
              </div>
            {/if}
          <!-- </section>
        </section>
          {/if} -->
</section>
{#if teamDialogOpen}
  <div
    class="dialog-backdrop"
    role="presentation"
    on:click={(event) => {
      if (event.currentTarget === event.target) teamDialogOpen = false;
    }}
  >
    <dialog open class="dialog" aria-labelledby="team-dialog-title">
      <div class="dialog-heading team-dialog-heading">
        <div>
          <h2 id="team-dialog-title">新增团队</h2>
          <p class="team-dialog-description">设置团队的名称、图标和描述。</p>
        </div>
        <div class="team-dialog-actions">
          <button
            class="secondary"
            type="button"
            on:click={() => (teamDialogOpen = false)}>取消</button
          >
          <button
            class="primary"
            type="submit"
            form="create-team-form"
            disabled={busy}>创建团队</button
          >
        </div>
      </div>
      <form
        id="create-team-form"
        class="stack-form"
        novalidate
        on:submit|preventDefault={createTeam}
      >
        {#if activeMessage}<MessageBanner
            message={activeMessage}
            tone={activeMessageTone}
          />{/if}
        <div class="team-identity-field">
          <div class="team-icon-selection">
            <label
              >图标<IconPicker
                value={teamIcon}
                onSelect={(icon) => (teamIcon = icon)}
                ariaLabel="选择团队图标"
              /></label
            >
          </div>
          <label class:invalid={Boolean(teamNameError)}
            ><span>名称<i class="required-mark" aria-hidden="true">*</i></span
            ><input
              bind:value={teamName}
              required
              aria-invalid={Boolean(teamNameError)}
              aria-describedby={teamNameError
                ? 'create-team-name-error'
                : undefined}
              on:input={() => {
                if (teamName.trim()) teamNameError = '';
              }}
              maxlength="120"
              placeholder="例如：支付平台"
            />{#if teamNameError}<small
                id="create-team-name-error"
                class="access-field-error"
                role="alert">{teamNameError}</small
              >{/if}</label
          >
        </div>
        <label
          >描述<textarea
            bind:value={teamDescription}
            rows="3"
            maxlength="1000"
            placeholder="描述团队的职责或用途"
          ></textarea></label
        >
      </form>
    </dialog>
  </div>
{/if}
{#if userDialogOpen || editingUser}
  <div
    class="dialog-backdrop"
    role="presentation"
    on:click={(event) => {
      if (event.currentTarget === event.target) {
        userDialogOpen = false;
        editingUser = null;
      }
    }}
  >
    <dialog open class="dialog wide-dialog" aria-labelledby="user-dialog-title">
      <div class="dialog-heading">
        <div>
          <p class="eyebrow">USER ACCESS</p>
          <h2 id="user-dialog-title">
            {editingUser ? '编辑用户' : '新增用户'}
          </h2>
        </div>
        {#if activeMessage}<MessageBanner
            message={activeMessage}
            tone={activeMessageTone}
          />{/if}
        <button
          class="icon-button"
          type="button"
          aria-label="关闭"
          on:click={() => {
            userDialogOpen = false;
            editingUser = null;
          }}>×</button
        >
      </div>
      <form
        class="stack-form"
        novalidate
        on:submit|preventDefault={editingUser ? saveUser : createUser}
      >
        <div class="form-row">
          <label class:invalid={Boolean(newUserUsernameError)}
            ><span
              >用户名<span class="required-mark" aria-hidden="true">*</span
              ></span
            ><input
              value={editingUser ? editingUser.username : newUserUsername}
              disabled={Boolean(editingUser)}
              on:input={(event) =>
                updateNewUserUsername(event.currentTarget.value)}
              required
              aria-invalid={Boolean(newUserUsernameError)}
              aria-describedby={newUserUsernameError
                ? 'new-user-username-error'
                : undefined}
              placeholder="登录用户名"
            />{#if newUserUsernameError}<small
                id="new-user-username-error"
                class="access-field-error"
                role="alert">{newUserUsernameError}</small
              >{/if}</label
          >
          <label
            class:invalid={Boolean(editingUser && editUserDisplayNameError)}
            >显示名<input
              bind:value={newUserDisplayName}
              aria-invalid={Boolean(editingUser && editUserDisplayNameError)}
              aria-describedby={editingUser && editUserDisplayNameError
                ? 'edit-user-display-name-error'
                : undefined}
              on:input={() => {
                if (newUserDisplayName.trim()) editUserDisplayNameError = '';
              }}
              maxlength="120"
              placeholder="默认使用用户名"
            />{#if editingUser && editUserDisplayNameError}<small
                id="edit-user-display-name-error"
                class="access-field-error"
                role="alert">{editUserDisplayNameError}</small
              >{/if}</label
          >
        </div>
        <div class="form-row">
          <label
            >邮箱<input
              type="email"
              bind:value={newUserEmail}
              placeholder="name@example.com"
            /></label
          >
          <label
            >手机号<input bind:value={newUserPhone} placeholder="+86" /></label
          >
        </div>
        <fieldset class="preference-group">
          <legend>{editingUser ? '重置密码' : '一次性密码'}</legend>
          {#if editingUser}
            <p class="form-help">
              点击重置密码后生成新的登录一次性密码，原密码立即失效。
            </p>
            <button
              class="secondary"
              type="button"
              disabled={busy}
              on:click={resetManagedUserPassword}>重置密码</button
            >
            {#if passwordResetCredentials}
              <div class="created-credentials-inline" aria-live="polite">
                <span
                  >一次性密码：<strong
                    >{passwordResetCredentials.password}</strong
                  ></span
                >
                <button
                  class="icon-button"
                  type="button"
                  aria-label="复制一次性密码"
                  data-tooltip="复制一次性密码"
                  on:click={() => copyPasswordResetCredentials(true)}
                >
                  {#if copiedControl === 'reset-credentials'}<ClipboardCheck
                      size={15}
                      aria-hidden="true"
                    />{:else}<Copy size={15} aria-hidden="true" />{/if}
                </button>
              </div>
            {/if}
          {:else}
            <div
              class="segmented-control"
              role="radiogroup"
              aria-label="一次性密码方式"
            >
              <button
                type="button"
                class:active={newUserPasswordMode === 'generated'}
                on:click={() => (newUserPasswordMode = 'generated')}
                >自动生成</button
              >
              <button
                type="button"
                class:active={newUserPasswordMode === 'manual'}
                on:click={() => (newUserPasswordMode = 'manual')}
                >手动设置</button
              >
            </div>
            {#if newUserPasswordMode === 'manual'}
              <label class:invalid={Boolean(newUserPasswordError)}
                ><span
                  >一次性密码<i class="required-mark" aria-hidden="true">*</i
                  ></span
                ><PasswordInput
                  bind:value={newUserPassword}
                  required
                  minlength={8}
                  ariaInvalid={Boolean(newUserPasswordError)}
                  ariaDescribedby={newUserPasswordError
                    ? 'new-user-password-error'
                    : ''}
                  autocomplete="new-password"
                  placeholder="至少 8 位"
                  ariaLabel="一次性密码"
                  on:input={() => {
                    if (newUserPassword.length >= 8) newUserPasswordError = '';
                  }}
                />{#if newUserPasswordError}<small
                    id="new-user-password-error"
                    class="access-field-error"
                    role="alert">{newUserPasswordError}</small
                  >{/if}</label
              >
            {:else}
              <p class="form-help">
                创建后显示一次性密码，仅可查看和复制一次。
              </p>
            {/if}
          {/if}
        </fieldset>
        <section class="new-user-grants" aria-label="用户授权">
          <div class="new-user-grants-heading">
            <div>
              <strong>授权配置</strong>
            </div>
            <button
              class="secondary"
              type="button"
              on:click={addNewUserGrant}
              disabled={busy ||
                manageableScopeChoices.length === 0 ||
                manageableScopeChoices.every((scope) =>
                  scopeAlreadyGranted(scope.id)
                )}><Plus size={15} aria-hidden="true" />添加授权</button
            >
          </div>
          <div class="new-user-grant-header" aria-hidden="true">
            <span>级别</span><span>对象</span><span>角色</span><span>操作</span>
          </div>
          {#each newUserGrants as grant, grantIndex}
            <section class="new-user-grant-row">
              <div class="new-user-grant-fields">
                <label
                  ><span class="sr-only">授权级别</span><select
                    value={grant.scopeType}
                    on:change={(event) =>
                      chooseNewUserGrantType(
                        grantIndex,
                        event.currentTarget.value as NewUserGrant['scopeType']
                      )}
                    >{#each ['platform', 'team', 'project'] as type}
                      {#if newUserGrantScopes(type as NewUserGrant['scopeType'], grantIndex).length > 0}
                        <option value={type}
                          >{grantScopeLabel(
                            type as NewUserGrant['scopeType']
                          )}</option
                        >
                      {/if}
                    {/each}</select
                  ></label
                >
                <label
                  ><span class="sr-only">授权对象</span>
                  {#if grant.scopeType === 'platform'}
                    <span class="new-user-no-object">无需选择</span>
                  {:else}
                    <select
                      value={grant.scopeID}
                      on:change={(event) =>
                        updateNewUserGrant(grantIndex, {
                          scopeID: event.currentTarget.value,
                          roleID: '',
                          resourceGrants: []
                        })}
                      ><option value=""
                        >选择{grant.scopeType === 'team'
                          ? '团队'
                          : '项目'}</option
                      >{#each newUserGrantScopes(grant.scopeType, grantIndex) as scope}
                        <option value={scope.id}>{scope.name}</option>
                      {/each}</select
                    >
                  {/if}
                </label>
                <label
                  ><span class="sr-only">角色</span><select
                    value={grant.roleID}
                    disabled={!grant.scopeID}
                    on:change={(event) =>
                      updateNewUserGrant(grantIndex, {
                        roleID: event.currentTarget.value,
                        resourceGrants: []
                      })}
                    ><option value="">选择角色</option
                    >{#each newUserGrantRoles(grant) as role}
                      <option value={role.id}
                        >{grantRoleLabel(role.name)}</option
                      >
                    {/each}</select
                  ></label
                >
                <button
                  class="icon-button danger-action"
                  type="button"
                  data-tooltip="移除此授权"
                  aria-label="移除此授权"
                  disabled={busy || newUserGrants.length === 1}
                  on:click={() => removeNewUserGrant(grantIndex)}
                  ><Trash2 size={15} aria-hidden="true" /></button
                >
              </div>
              {#if newUserGrantIsScopeViewer(grant)}
                <div class="new-user-resource-grants">
                  <div>
                    <strong>范围资源权限</strong>
                    <small
                      >{grantScopeLabel(
                        grant.scopeType
                      )}观察员默认可读取该范围资源；可为指定资源追加操作或管理权限。</small
                    >
                  </div>
                  {#each grant.resourceGrants as resourceGrant, resourceIndex}
                    <div class="new-user-resource-grant-row">
                      <label
                        >资源<select
                          value={resourceGrant.resourceID}
                          on:change={(event) =>
                            updateNewUserResourceGrant(
                              grantIndex,
                              resourceIndex,
                              { resourceID: event.currentTarget.value }
                            )}
                          ><option value="">选择范围内资源</option
                          >{#each newUserGrantResources(grant, resourceIndex) as resource}
                            <option value={resource.id}
                              >{resource.name} · {resource.kind}</option
                            >
                          {/each}</select
                        ></label
                      >
                      <label
                        >资源权限<select
                          value={resourceGrant.roleID}
                          on:change={(event) =>
                            updateNewUserResourceGrant(
                              grantIndex,
                              resourceIndex,
                              { roleID: event.currentTarget.value }
                            )}
                          ><option value="">选择资源权限</option
                          >{#each newUserGrantResourceRoles(grant) as resourceRole}
                            <option value={resourceRole.id}
                              >{roleLabel(resourceRole.name)}</option
                            >
                          {/each}</select
                        ></label
                      >
                      <button
                        class="icon-button danger-action"
                        type="button"
                        data-tooltip="移除资源权限"
                        aria-label="移除资源权限"
                        on:click={() =>
                          removeNewUserResourceGrant(grantIndex, resourceIndex)}
                        ><Trash2 size={14} aria-hidden="true" /></button
                      >
                    </div>
                  {/each}
                  <button
                    class="secondary"
                    type="button"
                    on:click={() => addNewUserResourceGrant(grantIndex)}
                    disabled={busy || newUserGrantResources(grant).length === 0}
                    ><Plus size={14} aria-hidden="true" />添加资源权限</button
                  >
                </div>
              {/if}
            </section>
          {:else}
            <p class="form-help">当前账号没有可授权的范围。</p>
          {/each}
        </section>
        <div class="form-actions">
          {#if createdUserCredentials}
            <div class="created-credentials-inline" aria-live="polite">
              <span
                >一次性密码：<strong>{createdUserCredentials.password}</strong
                ></span
              >
              <button
                class="icon-button"
                type="button"
                aria-label="复制一次性密码"
                data-tooltip="复制一次性密码"
                on:click={copyOneTimePassword}
                >{#if copiedControl === 'created-password'}<ClipboardCheck
                    size={15}
                    aria-hidden="true"
                  />{:else}<Copy size={15} aria-hidden="true" />{/if}</button
              >
            </div>
          {/if}
          <button
            class="secondary"
            type="button"
            on:click={() => {
              userDialogOpen = false;
              editingUser = null;
            }}>取消</button
          ><button
            class="primary"
            disabled={busy ||
              newUserGrants.length === 0 ||
              newUserGrants.some(
                (grant) =>
                  !grant.scopeID ||
                  !grant.roleID ||
                  grant.resourceGrants.some(
                    (resourceGrant) =>
                      !resourceGrant.resourceID || !resourceGrant.roleID
                  )
              )}>{editingUser ? '保存用户与授权' : '创建用户并授权'}</button
          >
        </div>
      </form>
    </dialog>
  </div>
{/if}
{#if editingTeam}
  <div
    class="dialog-backdrop"
    role="presentation"
    on:click={(event) => {
      if (event.currentTarget === event.target) editingTeam = null;
    }}
  >
    <dialog open class="dialog" aria-labelledby="edit-team-dialog-title">
      <div class="dialog-heading team-dialog-heading">
        <div>
          <h2 id="edit-team-dialog-title">编辑团队</h2>
          <p class="team-dialog-description">
            更新团队的名称、图标、描述和状态。
          </p>
        </div>
        <div class="team-dialog-actions">
          <button
            class="secondary"
            type="button"
            on:click={() => (editingTeam = null)}>取消</button
          >
          <button
            class="primary"
            type="submit"
            form="edit-team-form"
            disabled={busy}>保存团队</button
          >
        </div>
      </div>
      <form
        id="edit-team-form"
        class="stack-form"
        novalidate
        on:submit|preventDefault={saveTeam}
      >
        {#if activeMessage}<MessageBanner
            message={activeMessage}
            tone={activeMessageTone}
          />{/if}
        <div class="team-identity-field">
          <div class="team-icon-selection">
            <label
              >图标<IconPicker
                value={editTeamIcon}
                onSelect={(icon) => (editTeamIcon = icon)}
                ariaLabel="选择团队图标"
              /></label
            >
          </div>
          <label class:invalid={Boolean(editTeamNameError)}
            ><span>名称<i class="required-mark" aria-hidden="true">*</i></span
            ><input
              bind:value={editTeamName}
              required
              aria-invalid={Boolean(editTeamNameError)}
              aria-describedby={editTeamNameError
                ? 'edit-team-name-error'
                : undefined}
              on:input={() => {
                if (editTeamName.trim()) editTeamNameError = '';
              }}
              maxlength="120"
              placeholder="例如：支付平台"
            />{#if editTeamNameError}<small
                id="edit-team-name-error"
                class="access-field-error"
                role="alert">{editTeamNameError}</small
              >{/if}</label
          >
        </div>
        <label
          >描述<textarea
            bind:value={editTeamDescription}
            rows="3"
            maxlength="1000"
            placeholder="描述团队的职责或用途"
          ></textarea></label
        >
        <label
          >状态<select bind:value={editTeamStatus}
            ><option value="active">启用</option><option value="disabled"
              >禁用</option
            ></select
          ></label
        >
      </form>
    </dialog>
  </div>
{/if}
{#if disableTarget}
  <div class="dialog-backdrop" role="presentation">
    <dialog
      open
      class="dialog confirm-dialog"
      aria-labelledby="disable-dialog-title"
    >
      <div class="dialog-heading">
        <div>
          <p class="eyebrow">CONFIRM ACTION</p>
          <h2 id="disable-dialog-title">
            删除{disableTarget.ids.length} 个{disableTarget.kind === 'team'
              ? '团队'
              : '用户'}？
          </h2>
        </div>
        {#if activeMessage}<MessageBanner
            message={activeMessage}
            tone={activeMessageTone}
          />{/if}
      </div>
      <p class="confirm-copy">
        {disableTarget.kind === 'team'
          ? '团队将被禁用，其项目与历史数据会保留。禁用后团队不可继续用于新操作。'
          : '用户将被禁用并无法继续登录，现有角色绑定与审计记录会保留。'}
      </p>
      <div class="form-actions">
        <button
          class="secondary"
          type="button"
          disabled={busy}
          on:click={() => (disableTarget = null)}>取消</button
        ><button
          class="danger-button"
          type="button"
          disabled={busy}
          on:click={confirmDisable}>{busy ? '正在处理' : '确认删除'}</button
        >
      </div>
    </dialog>
  </div>
{/if}
