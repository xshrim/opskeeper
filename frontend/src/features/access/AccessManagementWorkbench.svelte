<script lang="ts">
  import { ChevronDown, FolderKanban, Pencil, Plus, RefreshCw, Search, ShieldCheck, Trash2, UsersRound } from 'lucide-svelte';
  import IconValue from '../../components/IconValue.svelte';
  import type { Project, RoleDefinition, Team, User } from '../../lib/api';

  type DisableKind = 'team' | 'user';
  type Selection = { kind: 'platform' | 'team' | 'project'; id: string; name: string; teamID?: string };

  export let accessSearch = '';
  export let visibleAccessTeams: Team[] = [];
  export let visibleAccessUsers: User[] = [];
  export let projects: Project[] = [];
  export let roles: RoleDefinition[] = [];
  export let accessTeamUsers: Record<string, User[]> = {};
  export let teamAccessExpanded: Record<string, boolean> = {};
  export let selectedAccessUserIds: string[] = [];
  export let accessLoading = false;
  export let accessLoadError = '';
  export let accessCanCreateTeam = false;
  export let accessCanCreateUser = false;
  export let busy = false;
  export let currentUser: User | null = null;
  export let onAddTeam: () => void = () => {};
  export let onAddUser: () => void = () => {};
  export let onEditTeam: (team: Team) => void = () => {};
  export let onEditUser: (user: User) => void = () => {};
  export let onDisable: (kind: DisableKind, ids: string[]) => void = () => {};
  export let onToggleTeam: (id: string) => void = () => {};
  export let onReload: () => void = () => {};
  export let canManageTeam: (team: Team) => boolean = () => false;
  export let canManageUser: (user: User) => boolean = () => false;
  export let userRoles: (userID: string) => string[] = () => [];
  export let userScopeNames: (userID: string) => string[] = () => [];
  export let userPermissions: (userID: string) => string[] = () => [];
  export let roleLabel: (name: string) => string = (name) => name;
  export let roleScopeLabel: (type: string) => string = (type) => type;
  export let permissionDescription: (permission: string) => string = (permission) => permission;

  let selected: Selection = { kind: 'platform', id: 'platform', name: '平台' };
  let memberQuery = '';
  $: selectedTeam = visibleAccessTeams.find((team) => team.id === selected.id);
  $: selectedProject = projects.find((project) => project.id === selected.id);
  $: scopedUsers = dedupeUsers(selected.kind === 'platform'
    ? visibleAccessUsers
    : selected.kind === 'team'
      ? accessTeamUsers[selected.id] ?? []
      : visibleAccessUsers.filter((user) => userScopeNames(user.id).some((scope) => scope === selectedProject?.name || scope === selectedProject?.code)));
  $: visibleScopedUsers = memberQuery ? scopedUsers.filter((user) => `${user.display_name} ${user.username} ${user.email}`.toLowerCase().includes(memberQuery.toLowerCase())) : scopedUsers;
  $: manageableUsers = visibleScopedUsers.filter(canManageUser);
  $: allUsersSelected = manageableUsers.length > 0 && manageableUsers.every((user) => selectedAccessUserIds.includes(user.id));
  $: selectedProjects = selected.kind === 'team' ? projects.filter((project) => project.team_id === selected.id) : selected.kind === 'project' ? (selectedProject ? [selectedProject] : []) : projects;

  function dedupeUsers(items: User[]) { return [...new Map(items.map((user) => [user.id, user])).values()]; }
  function statusLabel(status: string) { return status === 'active' ? '启用' : status === 'locked' ? '已锁定' : '已禁用'; }
  function initials(user: User) { return (user.display_name || user.username).slice(0, 1).toUpperCase(); }
  function selectPlatform() { selected = { kind: 'platform', id: 'platform', name: '平台' }; memberQuery = ''; }
  function selectTeam(team: Team) { selected = { kind: 'team', id: team.id, name: team.name }; memberQuery = ''; }
  function selectProject(project: Project) { selected = { kind: 'project', id: project.id, name: project.name, teamID: project.team_id }; memberQuery = ''; }
  function toggleAllUsers(checked: boolean) { selectedAccessUserIds = checked ? manageableUsers.map((user) => user.id) : []; }
  function memberCount(project: Project) { return dedupeUsers(visibleAccessUsers.filter((user) => userScopeNames(user.id).some((scope) => scope === project.name || scope === project.code))).length; }
</script>

<section class="access-management-workbench">
  <div class="access-workbench-toolbar">
    <div class="access-workbench-heading"><div><p class="eyebrow">ACCESS MANAGEMENT</p><h2>团队与用户</h2><p>按团队和项目定位成员，在用户管理中完成账号与授权配置。</p></div><span class="access-count">{visibleAccessTeams.length} 个团队 · {visibleAccessUsers.length} 个用户</span></div>
    <label class="access-search"><Search size={15} aria-hidden="true" /><span class="sr-only">搜索团队或用户</span><input bind:value={accessSearch} placeholder="搜索团队名称、编码、姓名、邮箱或手机号" /></label>
    <div class="access-heading-actions"><button class="secondary" type="button" on:click={onReload} disabled={busy}><RefreshCw size={14} />刷新</button>{#if accessCanCreateTeam}<button class="primary" type="button" on:click={onAddTeam}><Plus size={15} />添加团队</button>{/if}{#if accessCanCreateUser}<button class="primary" type="button" on:click={onAddUser}><Plus size={15} />添加用户</button>{/if}</div>
  </div>

  {#if accessLoading}<div class="access-state" aria-live="polite">正在加载管理数据...</div>{:else if accessLoadError}<div class="access-state access-error"><span>{accessLoadError}</span><button class="secondary" type="button" on:click={onReload}>重试</button></div>{:else}
    <div class="access-scope-desk">
      <aside class="access-scope-tree panel">
        <div class="access-scope-tree-head"><div><h3>组织范围</h3><p>选择团队或项目</p></div>{#if accessCanCreateTeam}<button class="icon-button" type="button" aria-label="添加团队" data-tooltip="添加团队" on:click={onAddTeam}><Plus size={15} /></button>{/if}</div>
        <button class:active={selected.kind === 'platform'} class="access-scope-root" type="button" on:click={selectPlatform}><span class="scope-tree-icon"><UsersRound size={16} /></span><span><strong>平台</strong><small>{visibleAccessUsers.length} 位用户 · {projects.length} 个项目</small></span></button>
        <div class="access-scope-team-list">
          {#each visibleAccessTeams as team}
            {@const teamProjects = projects.filter((project) => project.team_id === team.id)}
            <div class="access-scope-team-node"><div class:active={selected.kind === 'team' && selected.id === team.id} class="access-scope-team-row"><button class="scope-tree-expander" type="button" aria-label={`展开 ${team.name}`} aria-expanded={teamAccessExpanded[team.id]} on:click={() => onToggleTeam(team.id)}><ChevronDown size={14} class={teamAccessExpanded[team.id] ? 'expanded' : undefined} /></button><button class="scope-tree-select" type="button" on:click={() => selectTeam(team)}><IconValue value={team.icon} size={16} /><span><strong>{team.name}</strong><small>{(accessTeamUsers[team.id] ?? []).length} 位成员</small></span></button><div class="scope-tree-actions">{#if canManageTeam(team)}<button class="icon-button" type="button" aria-label={`编辑团队 ${team.name}`} data-tooltip="编辑团队" on:click={() => onEditTeam(team)}><Pencil size={13} /></button>{/if}</div></div>{#if teamAccessExpanded[team.id]}<div class="access-scope-project-list">{#each teamProjects as project}<button class:active={selected.kind === 'project' && selected.id === project.id} class="access-scope-project-row" type="button" on:click={() => selectProject(project)}><FolderKanban size={14} /><span><strong>{project.name}</strong><small>{memberCount(project)} 位成员</small></span></button>{:else}<span class="directory-empty">暂无项目</span>{/each}</div>{/if}</div>
          {:else}<div class="directory-empty">没有匹配的团队</div>{/each}
        </div>
      </aside>

      <section class="access-user-desk panel">
        <div class="access-user-desk-head"><div><p class="eyebrow">{selected.kind === 'platform' ? 'PLATFORM' : selected.kind === 'team' ? 'TEAM' : 'PROJECT'}</p><h3>{selected.name}</h3><p>{selected.kind === 'platform' ? '平台范围内的项目与成员' : selected.kind === 'team' ? '团队范围内的项目与成员' : '项目范围内的成员列表'}</p></div><div class="access-user-desk-actions"><span class="access-count">{visibleScopedUsers.length} 位成员</span><button class="secondary danger-action" type="button" on:click={() => onDisable('user', selectedAccessUserIds)} disabled={selectedAccessUserIds.length === 0 || busy}><Trash2 size={14} />批量禁用</button>{#if accessCanCreateUser}<button class="primary" type="button" on:click={onAddUser}><Plus size={15} />添加用户</button>{/if}</div></div>
        <div class="access-project-strip"><div class="access-section-label"><FolderKanban size={14} />项目</div>{#each selectedProjects as project}<button class:active={selected.kind === 'project' && selected.id === project.id} class="access-project-chip" type="button" on:click={() => selectProject(project)}><strong>{project.name}</strong><span>{memberCount(project)} 人</span></button>{:else}<span class="access-project-empty">当前范围暂无项目</span>{/each}</div>
        <div class="access-member-section"><div class="access-member-heading"><div><div class="access-section-label"><UsersRound size={14} />成员</div><span>当前粒度下去重后的全部用户</span></div><label class="access-member-search"><Search size={14} /><span class="sr-only">筛选当前成员</span><input bind:value={memberQuery} placeholder="筛选当前成员" /></label></div><div class="access-member-table"><div class="access-member-table-head"><input type="checkbox" aria-label="选择全部可管理成员" checked={allUsersSelected} on:change={(event) => toggleAllUsers(event.currentTarget.checked)} /><span>用户</span><span>授权范围</span><span>角色与权限</span><span>状态</span><span>操作</span></div>{#each visibleScopedUsers as user}<article class="access-member-row"><input type="checkbox" aria-label={`选择用户 ${user.display_name || user.username}`} disabled={!canManageUser(user) || user.status !== 'active'} bind:group={selectedAccessUserIds} value={user.id} /><div class="access-user-main"><span class="avatar access-avatar">{initials(user)}</span><span><strong>{user.display_name || user.username}</strong><small>@{user.username}{user.email ? ` · ${user.email}` : ''}</small></span></div><div class="access-user-scopes">{#each userScopeNames(user.id).slice(0, 3) as scope}<span>{scope}</span>{:else}<span class="permission-empty">无可见范围</span>{/each}</div><div class="access-user-auth"><div class="access-user-roles">{#each userRoles(user.id).slice(0, 2) as role}<span class="role-chip">{roleLabel(role)}</span>{:else}<span class="role-chip muted-chip">未分配角色</span>{/each}</div><div class="access-user-permissions">{#each userPermissions(user.id).slice(0, 2) as permission}<span data-tooltip={permissionDescription(permission)} title={permissionDescription(permission)}>{permission}</span>{:else}<span class="permission-empty">暂无权限</span>{/each}{#if userPermissions(user.id).length > 2}<span>+{userPermissions(user.id).length - 2}</span>{/if}</div></div><span class="status-label {user.status}">{statusLabel(user.status)}</span><div class="access-row-actions">{#if canManageUser(user)}<button class="icon-button" type="button" aria-label={`编辑用户 ${user.display_name || user.username}`} data-tooltip="编辑用户与授权" on:click={() => onEditUser(user)}><Pencil size={15} /></button><button class="icon-button danger-action" type="button" aria-label={`禁用用户 ${user.display_name || user.username}`} data-tooltip="禁用用户" disabled={user.status !== 'active' || user.id === currentUser?.id} on:click={() => onDisable('user', [user.id])}><Trash2 size={15} /></button>{:else}<span class="read-only-label">{user.id === currentUser?.id ? '当前账号' : '只读'}</span>{/if}</div></article>{:else}<div class="access-state">当前范围没有可见成员。</div>{/each}</div></div>
      </section>
    </div>

    <section class="access-management-roles panel"><div class="access-panel-heading"><div><h3>角色目录</h3><p>角色是用户新增或编辑时的授权模板。</p></div><div class="access-role-boundary"><ShieldCheck size={16} />按当前账号可完整授予的角色筛选</div></div>{#if roles.length > 0}<div class="access-role-cards">{#each roles as role}<article class="role-catalog-item"><div class="role-card-heading"><div><strong>{roleLabel(role.name)}</strong><small>{roleScopeLabel(role.scope_type)}{role.builtin ? ' · 内置' : ''}</small></div><span class="role-permission-count">{role.permissions.length} 项权限</span></div><div class="permission-list">{#each role.permissions as permission}<span data-tooltip={permissionDescription(String(permission))} title={permissionDescription(String(permission))}>{permission}</span>{/each}</div></article>{/each}</div>{:else}<div class="access-state">当前账号没有角色目录查看权限。</div>{/if}</section>
  {/if}
</section>
