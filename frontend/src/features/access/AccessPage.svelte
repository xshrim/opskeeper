<script lang="ts">
  import { onMount } from 'svelte';
  import { ClipboardCheck, Copy, Pencil, Plus, Search, ShieldCheck, Trash2, ChevronDown } from 'lucide-svelte';
  import MessageBanner from '../../components/MessageBanner.svelte';
  import { api, ApiError, type Group, type Project, type Resource, type ResourceRoleBinding, type ResourceRoleDefinition, type RoleBinding, type RoleDefinition, type Team, type User } from '../../lib/api';
  import {
    actorPermissionsAtScope as getActorPermissionsAtScope,
    resourceVisibleToScope as isResourceVisibleToScope,
    userRoleBindings as getUserRoleBindings,
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
  export let users: User[] = [];
  export let roles: RoleDefinition[] = [];
  export let bindings: RoleBinding[] = [];
  export let groups: Group[] = [];
  export let groupMembers: Record<string, string[]> = {};
  export let resources: Resource[] = [];
  export let resourceRoles: ResourceRoleDefinition[] = [];
  export let resourceBindings: ResourceRoleBinding[] = [];
  export let accessTeamUsers: Record<string, User[]> = {};
  export let teamAccessExpanded: Record<string, boolean> = {};
  export let selectedAccessTeamIds: string[] = [];
  export let selectedAccessUserIds: string[] = [];
  export let teamDialogOpen = false;
  export let teamName = '';
  export let teamCode = '';
  export let teamIcon = 'UsersRound';
  export let iconPickerTarget: 'create' | 'edit' | null = null;
  export let teamIconSearch = '';
  export let userDialogOpen = false;
  export let editingTeam: Team | null = null;
  export let editTeamName = '';
  export let editTeamIcon = '';
  export let editTeamStatus = 'active';
  export let editingUser: User | null = null;
  export let editUserDisplayName = '';
  export let editUserScopeId = '';
  export let editUserRoleIds: string[] = [];
  export let editUserResourceRoleId = '';
  export let editUserResourceId = '';
  export let disableTarget: DisableTarget | null = null;
  export let newUserUsername = '';
  export let newUserEmail = '';
  export let newUserPhone = '';
  export let newUserDisplayName = '';
  export let newUserPassword = '';
  export let newUserPasswordMode: 'manual' | 'generated' = 'generated';
  export let createdUserCredentials: { username: string; password: string } | null = null;
  export let passwordResetCredentials: { username: string; password: string } | null = null;
  export let newUserGrants: NewUserGrant[] = [];
  export let manageableScopeChoices: Array<{ id: string; type: string; name: string }> = [];
  export let availableEditUserRoles: RoleDefinition[] = [];
  export let editingScopeViewer = false;
  export let availableScopeViewerResourceRoles: ResourceRoleDefinition[] = [];
  export let scopeViewerResources: Resource[] = [];
  export let scopeViewerResourceBindings: Array<ResourceRoleBinding & { resource_name: string; role_name: string }> = [];
  export let scopeChoices: Array<{ id: string; type: string; name: string; parentId?: string }> = [];
  export let preferredScopeId = '';
  export let teamIconOptions: Array<{ value: string; label: string; keywords: string }> = [];
  export let currentUser: User | null = null;
  export let isPlatformAdmin = false;
  export let busy = false;
  export let copiedControl: 'created-password' | 'reset-username' | 'reset-credentials' | null = null;
  export let activeMessage = '';
  export let activeMessageTone: 'success' | 'error' = 'success';
  export let filteredTeamIconOptions: Array<{ value: string; label: string; keywords: string }> = [];

  let accessSearchQuery = '';

  $: manageableScopeChoices = isPlatformAdmin
    ? scopeChoices
    : scopeChoices.filter((scope) => actorPermissionsAtScope(scope.id).includes('member:grant'));
  $: accessSearchQuery = accessSearch.trim().toLowerCase();
  $: visibleAccessUsers = users.filter((user) => {
    if (!accessSearchQuery) return true;
    return [user.display_name, user.username, user.email, user.phone]
      .filter(Boolean)
      .some((value) => value.toLowerCase().includes(accessSearchQuery));
  });
  $: visibleAccessTeams = teams.filter((team) => {
    if (!accessSearchQuery) return true;
    return [team.name, team.code, team.status].some((value) =>
      value.toLowerCase().includes(accessSearchQuery)
    );
  });
  $: accessCanManageUsers = manageableScopeChoices.length > 0;
  $: accessCanCreateUser = isPlatformAdmin || accessCanManageUsers || userRoleBindings(currentUser?.id ?? '').some((binding) => ['PlatformAdmin', 'TeamAdmin', 'ProjectAdmin'].includes(binding.role_name));
  $: accessCanCreateTeam = isPlatformAdmin;
  $: availableEditUserRoles = grantableRolesForScope(editUserScopeId);
  $: editingScopeViewer = Boolean(editingUser && editUserRoleIds.some((roleID) => roles.find((role) => role.id === roleID)?.name === resourceGrantViewerRole(scopeType(editUserScopeId))));
  $: scopeViewerResources = resources.filter((resource) => resourceVisibleToScope(editUserScopeId, resource.scope_id) && resource.status === 'active');
  $: availableScopeViewerResourceRoles = resourceRoles.filter((resourceRole) => viewerResourceRoleAllowed(resourceRole) && resourceRole.permissions.every((permission) => actorPermissionsAtScope(editUserScopeId).includes(String(permission))));
  $: scopeViewerResourceBindings = editingUser ? resourceBindings.filter((binding) => binding.subject_type === 'user' && binding.subject_id === editingUser?.id && binding.scope_id === editUserScopeId) : [];
  $: accessTeamUsers = Object.fromEntries(teams.map((team) => {
    const teamScopeIDs = new Set([team.scope.id, ...projects.filter((project) => project.team_id === team.id).map((project) => project.scope.id)]);
    const memberIDs = groups.filter((group) => teamScopeIDs.has(group.scope_id)).flatMap((group) => groupMembers[group.id] ?? []);
    const roleIDs = bindings.filter((binding) => teamScopeIDs.has(binding.scope_id) && binding.subject_type === 'user').map((binding) => binding.subject_id);
    const visibleIDs = new Set([...memberIDs, ...roleIDs]);
    return [team.id, users.filter((user) => visibleIDs.has(user.id))];
  })) as Record<string, User[]>;

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 403) return '当前账号没有执行此操作的权限。';
      if (error.status === 401) return '会话已过期，请重新登录。';
      return error.message || fallback;
    }
    return error instanceof Error ? error.message || fallback : fallback;
  }
  async function action(operation: () => Promise<void>) {
    busy = true; onError('');
    try { await operation(); } catch (error) { onError(describeError(error, '操作失败')); } finally { busy = false; }
  }
  async function loadAccess() {
    accessLoading = true; accessLoadError = '';
    try {
      const result = await Promise.allSettled([api.users(), api.groups(), api.roles(), api.bindings(), api.resourceRoles(), api.resourceBindings()]);
      users = result[0].status === 'fulfilled' ? result[0].value : [];
      groups = result[1].status === 'fulfilled' ? result[1].value : [];
      roles = result[2].status === 'fulfilled' ? result[2].value : [];
      bindings = result[3].status === 'fulfilled' ? result[3].value : [];
      resourceRoles = result[4].status === 'fulfilled' ? result[4].value : [];
      resourceBindings = result[5].status === 'fulfilled' ? result[5].value : [];
      const memberResults = await Promise.allSettled(groups.map((group) => api.groupMembers(group.id)));
      groupMembers = Object.fromEntries(groups.map((group, index) => [group.id, memberResults[index]?.status === 'fulfilled' ? memberResults[index].value.map((member) => member.user_id) : []]));
      const rejectedItems = result.map((item, index) => item.status === 'rejected' ? index : -1).filter((index) => index >= 0);
      if (rejectedItems.length > 0) { const names = ['用户', '成员组', '角色', '角色授权', '资源角色', '资源授权']; accessLoadError = `管理数据加载不完整：${rejectedItems.map((index) => names[index]).join('、')}。`; }
      if (newUserGrants.length === 0) resetUserDialog();
    } catch { accessLoadError = '成员和角色数据加载失败，请重试。'; }
    finally { accessLoading = false; }
  }
  onMount(() => { void loadAccess(); });
  let handledCreateTeamRequest = 0;
  $: if (openCreateTeamRequest > handledCreateTeamRequest) {
    handledCreateTeamRequest = openCreateTeamRequest;
    accessTab = 'teams';
    openTeamDialog();
  }
  function scopeType(id: string) { return scopeChoices.find((scope) => scope.id === id)?.type ?? 'scope'; }
  function scopeName(id: string) { return scopeChoices.find((scope) => scope.id === id)?.name ?? id.slice(0, 8); }
  function userRoleBindings(userID: string) { return getUserRoleBindings(userID, groups, groupMembers, bindings); }
  function userRoles(userID: string) { return [...new Set(userRoleBindings(userID).map((binding) => binding.role_name))]; }
  function roleLabel(name: string) { const labels: Record<string, string> = { PlatformAdmin: '平台管理员', PlatformOperator: '平台操作员', PlatformViewer: '平台观察员', TeamAdmin: '团队管理员', TeamOperator: '团队操作员', TeamViewer: '团队观察员', ProjectAdmin: '项目管理员', ProjectOperator: '项目操作员', ProjectViewer: '项目观察员', ResourceAdmin: '资源管理员', ResourceOperator: '资源操作员', ResourceViewer: '资源观察员' }; return labels[name] ?? name; }
  function grantRoleLabel(name: string) { const labels: Record<string, string> = { PlatformAdmin: '管理员', PlatformOperator: '操作员', PlatformViewer: '观察员', TeamAdmin: '管理员', TeamOperator: '操作员', TeamViewer: '观察员', ProjectAdmin: '管理员', ProjectOperator: '操作员', ProjectViewer: '观察员' }; return labels[name] ?? roleLabel(name); }
  function grantScopeLabel(type: NewUserGrant['scopeType']) { return ({ platform: '平台', team: '团队', project: '项目' })[type]; }
  function resourceGrantViewerRole(type: string) { return ({ platform: 'PlatformViewer', team: 'TeamViewer', project: 'ProjectViewer' } as Record<string, string>)[type] ?? ''; }
  function roleScopeLabel(type: string) { return ({ platform: '平台级', team: '团队级', project: '项目级', resource: '资源级' } as Record<string, string>)[type] ?? type; }
  function userPermissions(userID: string) { const roleIDs = new Set(userRoleBindings(userID).map((binding) => binding.role_id)); return [...new Set(roles.filter((role) => roleIDs.has(role.id)).flatMap((role) => role.permissions.map(String)))]; }
  function permissionDescription(permission: string) { const descriptions: Record<string, string> = { 'organization:read': '查看组织、平台和级别信息', 'team:manage': '创建、编辑和停用团队', 'project:manage': '创建、编辑和停用项目', 'member:grant': '管理用户、用户组和角色授权', 'resource:read': '查看资源列表、配置和详情', 'resource:create': '创建资源', 'resource:update': '编辑资源配置', 'resource:delete': '删除或停用资源', 'resource:use': '使用资源执行连接测试或业务调用', 'engine:manage': '管理 AI 引擎及其级别内的默认 AIProvider', 'credential:manage': '管理凭据及其关联配置', 'credential:test': '测试凭据连接', 'relation:manage': '管理资源之间的关联关系', 'discovery:run': '启动集群或资源发现', 'discovery:import': '导入发现结果', 'diagnosis:start': '启动 AI 诊断', 'diagnosis:read': '查看诊断记录和结果', 'inspection:manage': '管理自动巡检策略', 'inspection:execute': '执行自动巡检', 'operation:approve': '审批受控操作', 'audit:read': '查看审计日志' }; return descriptions[permission] ?? '暂无权限说明'; }
  function actorPermissionsAtScope(scopeID: string) { return getActorPermissionsAtScope(scopeID, currentUser?.id, isPlatformAdmin, roles, groups, groupMembers, bindings, scopeChoices); }
  function grantableRolesForScope(scopeID: string) { if (!scopeID) return []; const permissions = new Set(actorPermissionsAtScope(scopeID)); return roles.filter((role) => role.scope_type === scopeType(scopeID) && role.permissions.every((permission) => permissions.has(permission))); }
  function canManageTeam(_team: Team) { return isPlatformAdmin; }
  function canManageUser(user: User) { return user.can_manage ?? (accessCanManageUsers && user.id !== currentUser?.id); }
  function userScopeNames(userID: string) { return [...new Set(userRoleBindings(userID).map((binding) => scopeName(binding.scope_id)))]; }
  function resourceVisibleToScope(viewerScopeID: string, resourceScopeID: string) { return isResourceVisibleToScope(scopeChoices, viewerScopeID, resourceScopeID); }
  function viewerResourceRoleAllowed(resourceRole: ResourceRoleDefinition) { return isViewerResourceRoleAllowed(resourceRole); }

  export let teamIconComponent: (icon: string) => any = () => null;
  export let onNotice: (message: string) => void = () => {};
  export let onError: (message: string) => void = () => {};

  function toggleTeamAccess(id: string) { teamAccessExpanded = { ...teamAccessExpanded, [id]: !teamAccessExpanded[id] }; }
  function requestDisable(kind: DisableTarget['kind'], ids: string[]) { if (ids.length > 0) disableTarget = { kind, ids: [...ids] }; }
  async function confirmDisable() {
    if (!disableTarget) return;
    const target = disableTarget;
    await action(async () => {
      if (target.kind === 'team') {
        const updated = await Promise.all(target.ids.map((id) => api.updateTeam(id, { status: 'disabled' })));
        const byID = new Map(updated.map((team) => [team.id, team])); teams = teams.map((team) => byID.get(team.id) ?? team); selectedAccessTeamIds = [];
      } else {
        const updated = await Promise.all(target.ids.map((id) => api.updateUser(id, { status: 'disabled' })));
        const byID = new Map(updated.map((user) => [user.id, user])); users = users.map((user) => byID.get(user.id) ?? user); selectedAccessUserIds = [];
      }
      disableTarget = null; onNotice(`${target.ids.length} 个${target.kind === 'team' ? '团队' : '用户'}已禁用`);
    });
  }
  function resetUserDialog() {
    newUserUsername = ''; newUserEmail = ''; newUserPhone = ''; newUserDisplayName = ''; newUserPassword = ''; newUserPasswordMode = 'generated'; createdUserCredentials = null;
    const preferred = scopeChoices.find((scope) => scope.id === preferredScopeId) ?? manageableScopeChoices[0];
    newUserGrants = preferred ? [{ scopeType: preferred.type as NewUserGrant['scopeType'], scopeID: preferred.id, roleID: '', resourceGrants: [] }] : [];
  }
  function addNewUserGrant() { const scope = manageableScopeChoices[0]; if (scope) newUserGrants = [...newUserGrants, { scopeType: scope.type as NewUserGrant['scopeType'], scopeID: scope.id, roleID: '', resourceGrants: [] }]; }
  function updateNewUserGrant(index: number, updates: Partial<NewUserGrant>) { newUserGrants = newUserGrants.map((grant, i) => i === index ? { ...grant, ...updates } : grant); }
  function chooseNewUserGrantType(index: number, type: NewUserGrant['scopeType']) { const scope = manageableScopeChoices.find((item) => item.type === type); updateNewUserGrant(index, { scopeType: type, scopeID: scope?.id ?? '', roleID: '', resourceGrants: [] }); }
  function removeNewUserGrant(index: number) { newUserGrants = newUserGrants.filter((_, i) => i !== index); }
  function newUserGrantScopes(type: NewUserGrant['scopeType']) { return manageableScopeChoices.filter((scope) => scope.type === type); }
  function newUserGrantRoles(grant: NewUserGrant) { return grantableRolesForScope(grant.scopeID); }
  function newUserGrantIsScopeViewer(grant: NewUserGrant) { return roles.find((role) => role.id === grant.roleID)?.name === resourceGrantViewerRole(grant.scopeType); }
  function newUserGrantResources(grant: NewUserGrant) { return resources.filter((resource) => resourceVisibleToScope(grant.scopeID, resource.scope_id) && resource.status === 'active'); }
  function newUserGrantResourceRoles(grant: NewUserGrant) { return resourceRoles.filter((role) => viewerResourceRoleAllowed(role) && role.permissions.every((permission) => actorPermissionsAtScope(grant.scopeID).includes(String(permission)))); }
  function addNewUserResourceGrant(index: number) { const grant = newUserGrants[index]; if (grant) updateNewUserGrant(index, { resourceGrants: [...grant.resourceGrants, { resourceID: '', roleID: '' }] }); }
  function updateNewUserResourceGrant(gi: number, ri: number, updates: Partial<NewUserResourceGrant>) { const grant = newUserGrants[gi]; if (grant) updateNewUserGrant(gi, { resourceGrants: grant.resourceGrants.map((item, i) => i === ri ? { ...item, ...updates } : item) }); }
  function removeNewUserResourceGrant(gi: number, ri: number) { const grant = newUserGrants[gi]; if (grant) updateNewUserGrant(gi, { resourceGrants: grant.resourceGrants.filter((_, i) => i !== ri) }); }
  function updateNewUserUsername(value: string) { if (!newUserDisplayName || newUserDisplayName === newUserUsername) newUserDisplayName = value; newUserUsername = value; }
  function openTeamDialog() { teamName = ''; teamCode = ''; teamIcon = teamIconOptions[Math.floor(Math.random() * teamIconOptions.length)]?.value ?? 'UsersRound'; teamDialogOpen = true; }
  function openTeamIconPicker(target: 'create' | 'edit') { iconPickerTarget = target; }
  function selectTeamIcon(icon: string) { if (iconPickerTarget === 'create') teamIcon = icon; if (iconPickerTarget === 'edit') editTeamIcon = icon; iconPickerTarget = null; }
  function openEditTeam(team: Team) { editingTeam = team; editTeamName = team.name; editTeamIcon = team.icon; editTeamStatus = team.status; }
  function openEditUser(user: User) { editingUser = user; editUserDisplayName = user.display_name || user.username; passwordResetCredentials = null; const direct = bindings.filter((binding) => binding.subject_type === 'user' && binding.subject_id === user.id); editUserScopeId = direct.find((binding) => manageableScopeChoices.some((scope) => scope.id === binding.scope_id))?.scope_id ?? manageableScopeChoices[0]?.id ?? ''; editUserRoleIds = direct.filter((binding) => binding.scope_id === editUserScopeId).map((binding) => binding.role_id); }
  function chooseEditUserScope(scopeID: string) { editUserScopeId = scopeID; editUserRoleIds = editingUser ? bindings.filter((binding) => binding.subject_type === 'user' && binding.subject_id === editingUser?.id && binding.scope_id === scopeID).map((binding) => binding.role_id) : []; editUserResourceRoleId = ''; editUserResourceId = ''; }
  async function createTeam() { await action(async () => { const created = await api.createTeam({ name: teamName, code: teamCode, icon: teamIcon, labels: {} }); teams = [...teams, created]; teamDialogOpen = false; onNotice(`团队“${created.name}”已创建`); }); }
  async function createUser() { await action(async () => { const result = await api.createUser({ username: newUserUsername, email: newUserEmail, phone: newUserPhone, display_name: newUserDisplayName, password: newUserPassword, password_mode: newUserPasswordMode, grants: newUserGrants.map((grant) => ({ scope_id: grant.scopeID, role_id: grant.roleID, resource_grants: grant.resourceGrants.map((item) => ({ resource_id: item.resourceID, role_id: item.roleID })) })) }); users = [...users, result.user]; bindings = [...bindings, ...result.bindings]; createdUserCredentials = { username: result.user.username, password: result.one_time_password }; onNotice(`用户“${result.user.display_name || result.user.username}”已创建并完成授权`); }); }
  async function saveTeam() { if (!editingTeam) return; await action(async () => { const updated = await api.updateTeam(editingTeam!.id, { name: editTeamName, icon: editTeamIcon, status: editTeamStatus }); teams = teams.map((team) => team.id === updated.id ? updated : team); editingTeam = null; onNotice(`团队“${updated.name}”已更新`); }); }
  async function saveUser() { if (!editingUser || !editUserScopeId) return; await action(async () => { const userID = editingUser!.id; const existing = bindings.filter((binding) => binding.subject_type === 'user' && binding.subject_id === userID && binding.scope_id === editUserScopeId); const desired = new Set(editUserRoleIds); const created: RoleBinding[] = []; for (const roleID of editUserRoleIds) if (!existing.some((binding) => binding.role_id === roleID)) created.push(await api.createBinding({ subject_type: 'user', subject_id: userID, role_id: roleID, scope_id: editUserScopeId })); for (const binding of existing) if (!desired.has(binding.role_id)) await api.deleteBinding(binding.id); bindings = [...bindings.filter((binding) => !existing.some((old) => old.id === binding.id && !desired.has(old.role_id))), ...created]; editingUser = null; onNotice('用户授权已更新'); }); }
  async function resetManagedUserPassword() { if (!editingUser) return; await action(async () => { const result = await api.resetUserPassword(editingUser!.id); passwordResetCredentials = { username: editingUser!.username, password: result.one_time_password }; onNotice('已生成一次性密码'); }); }
  async function grantScopeViewerResource() { if (!editingUser || !editUserResourceRoleId || !editUserResourceId) return; await action(async () => { const binding = await api.createResourceBinding({ subject_type: 'user', subject_id: editingUser!.id, role_id: editUserResourceRoleId, resource_id: editUserResourceId }); resourceBindings = [...resourceBindings, binding]; editUserResourceRoleId = ''; editUserResourceId = ''; }); }
  async function revokeScopeViewerResource(binding: ResourceRoleBinding) { await action(async () => { await api.deleteResourceBinding(binding.id); resourceBindings = resourceBindings.filter((item) => item.id !== binding.id); }); }
  async function copyOneTimePassword() { if (!createdUserCredentials) return; try { await navigator.clipboard.writeText(createdUserCredentials.password); copiedControl = 'created-password'; onNotice('一次性密码已复制'); } catch { onError('无法访问剪贴板，请手动复制。'); } }
  async function copyPasswordResetCredentials(includePassword: boolean) { if (!passwordResetCredentials) return; try { await navigator.clipboard.writeText(includePassword ? `用户名：${passwordResetCredentials.username}\n一次性密码：${passwordResetCredentials.password}` : passwordResetCredentials.username); copiedControl = includePassword ? 'reset-credentials' : 'reset-username'; } catch { onError('无法访问剪贴板，请手动复制。'); } }
</script>

        <section class="access-page">
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
                  {@const TeamIcon = teamIconComponent(team.icon)}
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
                          ><TeamIcon size={17} strokeWidth={1.8} /></span
                        ><span
                          ><strong>{team.name}</strong><small>{team.code}</small
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
          </section>
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
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">TEAM</p>
                  <h2 id="team-dialog-title">新增团队</h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
                <button
                  class="icon-button"
                  type="button"
                  aria-label="关闭"
                  on:click={() => (teamDialogOpen = false)}>×</button
                >
              </div>
              <form class="stack-form" on:submit|preventDefault={createTeam}>
                <div class="team-identity-field">
                  <button
                    class="team-icon-picker-trigger"
                    type="button"
                    aria-label="选择团队图标"
                    data-tooltip="选择团队图标"
                    on:click={() => openTeamIconPicker('create')}
                    ><span class="entity-icon team-icon"
                      ><svelte:component
                        this={teamIconComponent(teamIcon)}
                        size={16}
                        strokeWidth={1.8}
                      /></span
                    ></button
                  ><label
                    >名称<input
                      bind:value={teamName}
                      required
                      maxlength="120"
                      placeholder="例如：支付平台"
                    /></label
                  >
                </div>
                <label
                  >团队编码<input
                    bind:value={teamCode}
                    required
                    placeholder="例如：payments"
                  /></label
                ><label
                  >图标<input bind:value={teamIcon} placeholder="team" /></label
                >
                <div class="form-actions">
                  <button
                    class="secondary"
                    type="button"
                    on:click={() => (teamDialogOpen = false)}>取消</button
                  ><button class="primary" disabled={busy}>创建团队</button>
                </div>
              </form>
            </dialog>
          </div>
        {/if}
        {#if userDialogOpen}
          <div
            class="dialog-backdrop"
            role="presentation"
            on:click={(event) => {
              if (event.currentTarget === event.target) userDialogOpen = false;
            }}
          >
            <dialog open class="dialog" aria-labelledby="user-dialog-title">
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">USER ACCESS</p>
                  <h2 id="user-dialog-title">新增用户</h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
                <button
                  class="icon-button"
                  type="button"
                  aria-label="关闭"
                  on:click={() => (userDialogOpen = false)}>×</button
                >
              </div>
              <form class="stack-form" on:submit|preventDefault={createUser}>
                <div class="form-row">
                  <label
                    ><span
                      >用户名<span class="required-mark" aria-hidden="true"
                        >*</span
                      ></span
                    ><input
                      value={newUserUsername}
                      on:input={(event) =>
                        updateNewUserUsername(event.currentTarget.value)}
                      required
                      placeholder="登录用户名"
                    /></label
                  ><label
                    >显示名<input
                      bind:value={newUserDisplayName}
                      placeholder="默认使用用户名"
                    /></label
                  >
                </div>
                <div class="form-row">
                  <label
                    >邮箱<input
                      type="email"
                      bind:value={newUserEmail}
                      placeholder="name@example.com"
                    /></label
                  ><label
                    >手机号<input
                      bind:value={newUserPhone}
                      placeholder="+86"
                    /></label
                  >
                </div>
                <fieldset class="preference-group">
                  <legend>一次性密码</legend>
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
                    <label
                      >一次性密码<input
                        type="password"
                        bind:value={newUserPassword}
                        required
                        minlength="8"
                        autocomplete="new-password"
                        placeholder="至少 8 位"
                      /></label
                    >
                  {:else}
                    <p class="form-help">
                      创建后显示一次性密码，仅可查看和复制一次。
                    </p>
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
                      disabled={busy || manageableScopeChoices.length === 0}
                      ><Plus size={15} aria-hidden="true" />添加授权</button
                    >
                  </div>
                  <div class="new-user-grant-header" aria-hidden="true">
                    <span>级别</span><span>对象</span><span>角色</span><span
                      >操作</span
                    >
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
                                event.currentTarget
                                  .value as NewUserGrant['scopeType']
                              )}
                            >{#each ['platform', 'team', 'project'] as type}
                              {#if newUserGrantScopes(type as NewUserGrant['scopeType']).length > 0}
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
                              >{#each newUserGrantScopes(grant.scopeType) as scope}
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
                                  >{#each newUserGrantResources(grant) as resource}
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
                                  removeNewUserResourceGrant(
                                    grantIndex,
                                    resourceIndex
                                  )}
                                ><Trash2 size={14} aria-hidden="true" /></button
                              >
                            </div>
                          {/each}
                          <button
                            class="secondary"
                            type="button"
                            on:click={() => addNewUserResourceGrant(grantIndex)}
                            disabled={busy}
                            ><Plus
                              size={14}
                              aria-hidden="true"
                            />添加资源权限</button
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
                        >一次性密码：<strong
                          >{createdUserCredentials.password}</strong
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
                          />{:else}<Copy
                            size={15}
                            aria-hidden="true"
                          />{/if}</button
                      >
                    </div>
                  {/if}
                  <button
                    class="secondary"
                    type="button"
                    on:click={() => (userDialogOpen = false)}>取消</button
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
                      )}>创建用户并授权</button
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
            <dialog
              open
              class="dialog"
              aria-labelledby="edit-team-dialog-title"
            >
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">TEAM</p>
                  <h2 id="edit-team-dialog-title">编辑团队</h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
                <button
                  class="icon-button"
                  type="button"
                  aria-label="关闭"
                  on:click={() => (editingTeam = null)}>×</button
                >
              </div>
              <form class="stack-form" on:submit|preventDefault={saveTeam}>
                <div class="team-identity-field">
                  <button
                    class="team-icon-picker-trigger"
                    type="button"
                    aria-label="选择团队图标"
                    data-tooltip="选择团队图标"
                    on:click={() => openTeamIconPicker('edit')}
                    ><span class="entity-icon team-icon"
                      ><svelte:component
                        this={teamIconComponent(editTeamIcon)}
                        size={16}
                        strokeWidth={1.8}
                      /></span
                    ></button
                  ><label
                    >名称<input
                      bind:value={editTeamName}
                      required
                      maxlength="120"
                      placeholder="例如：支付平台"
                    /></label
                  >
                </div>
                <label
                  >状态<select bind:value={editTeamStatus}
                    ><option value="active">启用</option><option
                      value="disabled">禁用</option
                    ></select
                  ></label
                >
                <div class="form-actions">
                  <button
                    class="secondary"
                    type="button"
                    on:click={() => (editingTeam = null)}>取消</button
                  ><button class="primary" disabled={busy}>保存团队</button>
                </div>
              </form>
            </dialog>
          </div>
        {/if}
        {#if iconPickerTarget}
          <div
            class="dialog-backdrop"
            role="presentation"
            on:click={(event) => {
              if (event.currentTarget === event.target) iconPickerTarget = null;
            }}
          >
            <dialog
              open
              class="dialog icon-picker-dialog"
              aria-labelledby="icon-picker-title"
            >
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">ICON PICKER</p>
                  <h2 id="icon-picker-title">选择图标</h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
                <button
                  class="icon-button"
                  type="button"
                  aria-label="关闭"
                  on:click={() => (iconPickerTarget = null)}>×</button
                >
              </div>
              <div class="icon-picker-body">
                <label class="icon-search"
                  ><Search size={16} aria-hidden="true" /><span class="sr-only"
                    >搜索图标</span
                  ><input
                    bind:value={teamIconSearch}
                    placeholder="搜索图标，如 Kubernetes、数据库"
                    aria-label="搜索图标"
                  /></label
                >
                <div class="team-icon-grid" aria-label="团队图标列表">
                  {#each filteredTeamIconOptions as option}
                    {@const TeamIcon = teamIconComponent(option.value)}
                    <button
                      class:active={(iconPickerTarget === 'create'
                        ? teamIcon
                        : editTeamIcon) === option.value}
                      type="button"
                      on:click={() => selectTeamIcon(option.value)}
                      aria-label={`选择图标 ${option.label}`}
                      ><span class="entity-icon team-icon"
                        ><TeamIcon size={18} strokeWidth={1.8} /></span
                      ><span>{option.label}</span></button
                    >
                  {:else}
                    <p class="icon-picker-empty">没有匹配的图标。</p>
                  {/each}
                </div>
              </div>
            </dialog>
          </div>
        {/if}
        {#if editingUser}
          <div
            class="dialog-backdrop"
            role="presentation"
            on:click={(event) => {
              if (event.currentTarget === event.target) editingUser = null;
            }}
          >
            <dialog
              open
              class="dialog wide-dialog"
              aria-labelledby="edit-user-dialog-title"
            >
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">USER ACCESS</p>
                  <h2 id="edit-user-dialog-title">编辑用户与授权</h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
                <button
                  class="icon-button"
                  type="button"
                  aria-label="关闭"
                  on:click={() => (editingUser = null)}>×</button
                >
              </div>
              <form class="stack-form" on:submit|preventDefault={saveUser}>
                <div class="form-row">
                  <label
                    >用户名<input
                      value={editingUser.username}
                      disabled
                      aria-label="用户名不可修改"
                    /></label
                  >
                  <label
                    >显示名<input
                      bind:value={editUserDisplayName}
                      required
                      maxlength="120"
                      placeholder="默认使用用户名"
                    /></label
                  >
                </div>
                <label
                  >授权 Scope<select
                    value={editUserScopeId}
                    on:change={(event) =>
                      chooseEditUserScope(event.currentTarget.value)}
                    >{#each manageableScopeChoices as scope}<option
                        value={scope.id}>{scope.name} · {scope.type}</option
                      >{/each}</select
                  ></label
                >
                <fieldset class="role-picker" disabled={!editUserScopeId}>
                  <legend>直接授权角色</legend>
                  {#each availableEditUserRoles as role}<label class="check-row"
                      ><input
                        type="checkbox"
                        bind:group={editUserRoleIds}
                        value={role.id}
                      /><span
                        ><strong>{roleLabel(role.name)}</strong><small
                          >{role.permissions.length} 项权限</small
                        ></span
                      ></label
                    >{:else}<p class="muted">
                      当前账号在该 Scope 没有可授予角色。
                    </p>{/each}
                </fieldset>
                <p class="form-help">
                  成员组继承的角色保持不变；这里只调整所选 Scope 下的直接角色。
                </p>
                {#if editingScopeViewer}
                  <section class="scope-viewer-resource-access">
                    <div class="scope-viewer-resource-heading">
                      <div>
                        <strong>范围资源权限</strong>
                        <p>
                          {grantScopeLabel(
                            scopeType(
                              editUserScopeId
                            ) as NewUserGrant['scopeType']
                          )}观察员默认可读取该范围资源；可为指定资源追加操作或管理权限。
                        </p>
                      </div>
                      <ShieldCheck size={17} aria-hidden="true" />
                    </div>
                    <div class="form-row">
                      <label
                        >资源角色<select bind:value={editUserResourceRoleId}>
                          <option value="">选择资源角色</option>
                          {#each availableScopeViewerResourceRoles as resourceRole}
                            <option value={resourceRole.id}
                              >{roleLabel(resourceRole.name)}</option
                            >
                          {/each}
                        </select></label
                      >
                      <label
                        >具体资源<select bind:value={editUserResourceId}>
                          <option value="">选择范围内资源</option>
                          {#each scopeViewerResources as resource}
                            <option value={resource.id}
                              >{resource.name} · {resource.kind}</option
                            >
                          {/each}
                        </select></label
                      >
                    </div>
                    <button
                      class="secondary"
                      type="button"
                      disabled={busy ||
                        !editUserResourceRoleId ||
                        !editUserResourceId}
                      on:click={grantScopeViewerResource}
                    >
                      <Plus size={15} aria-hidden="true" />添加资源权限
                    </button>
                    <div class="scope-viewer-resource-list">
                      {#each scopeViewerResourceBindings as resourceBinding}
                        <div class="scope-viewer-resource-item">
                          <span
                            ><strong>{resourceBinding.resource_name}</strong
                            ><small
                              >{roleLabel(resourceBinding.role_name)}</small
                            ></span
                          >
                          <button
                            class="icon-button danger-action"
                            type="button"
                            aria-label="移除资源权限"
                            data-tooltip="移除资源权限"
                            on:click={() =>
                              revokeScopeViewerResource(resourceBinding)}
                          >
                            <Trash2 size={14} aria-hidden="true" />
                          </button>
                        </div>
                      {:else}
                        <span class="permission-empty"
                          >尚未添加具体资源权限</span
                        >
                      {/each}
                    </div>
                  </section>
                {/if}
                {#if passwordResetCredentials}
                  <section class="role-preview" aria-live="polite">
                    <strong>一次性密码已生成</strong>
                    <label
                      >用户名<input
                        value={passwordResetCredentials.username}
                        readonly
                      /></label
                    >
                    <label
                      >一次性密码<input
                        value={passwordResetCredentials.password}
                        readonly
                      /></label
                    >
                    <div class="form-actions">
                      <button
                        class="secondary"
                        type="button"
                        on:click={() => copyPasswordResetCredentials(false)}
                        >{#if copiedControl === 'reset-username'}<ClipboardCheck
                            size={15}
                            aria-hidden="true"
                          />已复制{:else}<Copy
                            size={15}
                            aria-hidden="true"
                          />复制用户名{/if}</button
                      >
                      <button
                        class="primary"
                        type="button"
                        on:click={() => copyPasswordResetCredentials(true)}
                        >{#if copiedControl === 'reset-credentials'}<ClipboardCheck
                            size={15}
                            aria-hidden="true"
                          />已复制{:else}<Copy
                            size={15}
                            aria-hidden="true"
                          />复制用户名和密码{/if}</button
                      >
                    </div>
                  </section>
                {/if}
                <div class="form-actions">
                  <button
                    class="secondary"
                    type="button"
                    disabled={busy}
                    on:click={resetManagedUserPassword}>重置密码</button
                  >
                  <button
                    class="secondary"
                    type="button"
                    on:click={() => (editingUser = null)}>取消</button
                  ><button class="primary" disabled={busy || !editUserScopeId}
                    >保存授权</button
                  >
                </div>
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
                    删除{disableTarget.ids.length} 个{disableTarget.kind ===
                    'team'
                      ? '团队'
                      : '用户'}？
                  </h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
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
                  on:click={confirmDisable}
                  >{busy ? '正在处理' : '确认删除'}</button
                >
              </div>
            </dialog>
          </div>
        {/if}
