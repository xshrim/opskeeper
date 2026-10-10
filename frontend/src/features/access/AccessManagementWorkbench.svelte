<script lang="ts">
  import ExpandableTable from '../../components/ExpandableTable.svelte';
  import ReadonlyTable from '../../components/ReadonlyTable.svelte';
  import type { ReadonlyTableColumn } from '../../components/ReadonlyTable.svelte';
  import StatusBadge from '../../components/StatusBadge.svelte';
  import {
    ChevronDown,
    Pencil,
    Plus,
    Trash2,
    UsersRound
  } from 'lucide-svelte';
  import EntityBrandIcon from '../../components/EntityBrandIcon.svelte';
  import ScopeChip from '../../components/ScopeChip.svelte';
  import SearchInput from '../../components/SearchInput.svelte';
  import type {
    Group,
    Project,
    ResourceRoleBinding,
    Team,
    User
  } from '../../lib/api';

  type DisableKind = 'team' | 'user';
  type Selection = {
    kind: 'platform' | 'team' | 'project';
    id: string;
    name: string;
    teamID?: string;
  };
  type ScopeRoleEntry = {
    scopeName: string;
    scopeType: 'platform' | 'team' | 'project';
    roleName: string;
  };

  export let visibleAccessTeams: Team[] = [];
  export let visibleAccessUsers: User[] = [];
  export let projects: Project[] = [];
  export let scopeChoices: Array<{
    id: string;
    type: string;
    name: string;
    parentId?: string;
  }> = [];
  export let resourceBindings: ResourceRoleBinding[] = [];
  export let groups: Group[] = [];
  export let groupMembers: Record<string, string[]> = {};
  export let selectedTeamId = '';
  export let selectedProjectId = '';
  export let canViewPlatform = false;
  export let platformName = '平台';
  export let accessTeamUsers: Record<string, User[]> = {};
  export let accessProjectUsers: Record<string, User[]> = {};
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
  export let userScopeRoles: (
    userID: string,
    selection: Selection
  ) => ScopeRoleEntry[] = () => [];

  function globalSelection(): Selection {
    const project = projects.find((item) => item.id === selectedProjectId);
    if (project)
      return {
        kind: 'project',
        id: project.id,
        name: project.name,
        teamID: project.team_id
      };
    const team = visibleAccessTeams.find((item) => item.id === selectedTeamId);
    return team
      ? { kind: 'team', id: team.id, name: team.name }
      : { kind: 'platform', id: 'platform', name: platformName };
  }
  let selected: Selection = {
    kind: 'platform',
    id: 'platform',
    name: platformName
  };
  let previousGlobalScopeKey = '';
  let activeProjectID = '';
  let memberQuery = '';
  let memberFieldValues: Record<string, string> = {};
  const memberSearchFields = [
    { key: 'username', label: '用户名', placeholder: '输入用户名' },
    { key: 'name', label: '姓名', placeholder: '输入姓名' },
    { key: 'email', label: '邮箱', placeholder: '输入邮箱' },
    { key: 'phone', label: '电话', placeholder: '输入电话' },
    { key: 'status', label: '状态', placeholder: '输入启用、锁定或禁用' }
  ];
  const memberColumns: ReadonlyTableColumn[] = [
    { key: 'user', label: '用户', width: '15%' },
    { key: 'contact', label: '联络', width: '25%' },
    { key: 'permissions', label: '权限', width: '60%' },
    { key: 'status', label: '状态', width: '50px' },
    { key: 'actions', label: '操作', width: '60px', className: 'access-member-actions-heading' }
  ];
  const roleColumns: ReadonlyTableColumn[] = [
    { key: 'level', label: '级别', width: '80px' },
    { key: 'object', label: '授权对象' },
    { key: 'role', label: '角色' },
    { key: 'extra', label: '额外授权' }
  ];
  $: globalScopeSelection = globalSelection();
  $: globalScopeKey = `${selectedTeamId}|${selectedProjectId}|${globalScopeSelection.kind}|${globalScopeSelection.id}|${globalScopeSelection.name}`;
  $: if (globalScopeKey !== previousGlobalScopeKey) {
    previousGlobalScopeKey = globalScopeKey;
    selected = globalScopeSelection;
    activeProjectID = '';
    memberQuery = '';
    memberFieldValues = {};
    selectedAccessUserIds = [];
  }
  $: if (selected.kind === 'platform' && selected.name !== platformName)
    selected = { ...selected, name: platformName };
  $: selectedProject = projects.find((project) => project.id === selected.id);
  $: scopedUsers = dedupeUsers(
    selected.kind === 'platform'
      ? visibleAccessUsers
      : selected.kind === 'team'
        ? (accessTeamUsers[selected.id] ?? [])
        : (accessProjectUsers[selected.id] ?? [])
  );
  $: projectScopedUserIDs = new Set(
    (accessProjectUsers[activeProjectID] ?? []).map((user) => user.id)
  );
  $: projectFilteredUsers = activeProjectID
    ? scopedUsers.filter((user) => projectScopedUserIDs.has(user.id))
    : scopedUsers;
  $: memberFreeQuery = memberQuery
    .replace(/(?:^|\s)(?:username|name|email|phone|status):(?:"[^"]*"|'[^']*'|\S*)/g, ' ')
    .trim()
    .toLocaleLowerCase();
  $: visibleScopedUsers = projectFilteredUsers.filter((user) => {
    const searchValues = {
      username: user.username,
      name: user.display_name,
      email: user.email,
      phone: user.phone,
      status: `${user.status} ${statusLabel(user.status)}`
    };
    if (
      Object.entries(memberFieldValues).some(([key, filterValue]) =>
        key in searchValues &&
        !searchValues[key as keyof typeof searchValues]
          .toLocaleLowerCase()
          .includes(filterValue.toLocaleLowerCase())
      )
    )
      return false;
    return !memberFreeQuery ||
      Object.values(searchValues)
        .join(' ')
        .toLocaleLowerCase()
        .includes(memberFreeQuery);
  });
  $: visibleProjects = projects.filter((project) =>
    visibleAccessTeams.some((team) => team.id === project.team_id)
  );
  $: selectedProjects =
    selected.kind === 'team'
      ? projects.filter((project) => project.team_id === selected.id)
      : selected.kind === 'project'
        ? selectedProject
          ? [selectedProject]
          : []
        : visibleProjects;

  function dedupeUsers(items: User[]) {
    return [...new Map(items.map((user) => [user.id, user])).values()];
  }
  function statusLabel(status: string) {
    return status === 'active'
      ? '启用'
      : status === 'locked'
        ? '已锁定'
        : '已禁用';
  }
  function dateTimeLabel(value: string) {
    return value ? new Date(value).toLocaleString() : '—';
  }
  function scopeTypeLabel(type: ScopeRoleEntry['scopeType']) {
    return { platform: '平台', team: '团队', project: '项目' }[type];
  }
  function roleTone(roleName: string) {
    if (roleName.includes('管理员')) return 'admin';
    if (roleName.includes('操作员')) return 'operator';
    return 'viewer';
  }
  function resourceRoleLabel(roleName: string) {
    if (roleName.endsWith('Admin')) return '管理员';
    if (roleName.endsWith('Operator')) return '操作员';
    if (roleName.endsWith('Viewer')) return '观察员';
    return roleName;
  }
  function initials(user: User) {
    return (user.display_name || user.username).slice(0, 1).toUpperCase();
  }
  function selectPlatform() {
    selected = { kind: 'platform', id: 'platform', name: platformName };
    activeProjectID = '';
    memberQuery = '';
    selectedAccessUserIds = [];
  }
  function selectTeam(team: Team) {
    selected = { kind: 'team', id: team.id, name: team.name };
    activeProjectID = '';
    memberQuery = '';
    selectedAccessUserIds = [];
  }
  function selectProject(project: Project) {
    selected = {
      kind: 'project',
      id: project.id,
      name: project.name,
      teamID: project.team_id
    };
    activeProjectID = '';
    memberQuery = '';
    selectedAccessUserIds = [];
  }
  function toggleProjectFilter(projectID: string) {
    activeProjectID = activeProjectID === projectID ? '' : projectID;
    selectedAccessUserIds = [];
  }
  function memberCount(project: Project) {
    return dedupeUsers(accessProjectUsers[project.id] ?? []).length;
  }
  function selectionScopeIDs(selection: Selection) {
    if (selection.kind === 'platform') return new Set(scopeChoices.map((scope) => scope.id));
    if (selection.kind === 'project') {
      const project = projects.find((item) => item.id === selection.id);
      return new Set(project ? [project.scope.id] : []);
    }
    const teamScope = visibleAccessTeams.find((item) => item.id === selection.id)?.scope.id;
    return new Set([
      ...(teamScope ? [teamScope] : []),
      ...projects
        .filter((project) => project.team_id === selection.id)
        .map((project) => project.scope.id),
    ]);
  }
  function userResourceRoleBindings(userID: string, selection: Selection) {
    const groupIDs = new Set(
      groups
        .filter(
          (group) =>
            group.status === 'active' && groupMembers[group.id]?.includes(userID)
        )
        .map((group) => group.id)
    );
    const scopeIDs = selectionScopeIDs(selection);
    const seen = new Set<string>();
    return resourceBindings.filter(
      (binding) =>
        scopeIDs.has(binding.scope_id) &&
        ((binding.subject_type === 'user' && binding.subject_id === userID) ||
          (binding.subject_type === 'group' && groupIDs.has(binding.subject_id))) &&
        !seen.has(`${binding.resource_id}:${binding.role_id}`) &&
        Boolean(seen.add(`${binding.resource_id}:${binding.role_id}`))
    );
  }
</script>

<section class="access-management-workbench">
  {#if accessLoading}<div class="access-state" aria-live="polite">
      正在加载管理数据...
    </div>{:else if accessLoadError}<div class="access-state access-error">
      <span>{accessLoadError}</span><button
        class="secondary"
        type="button"
        on:click={onReload}>重试</button
      >
    </div>{:else}
    <div class="access-scope-desk">
      <aside class="access-scope-tree panel">
        <div class="access-scope-tree-head">
          <div>
            <h3>团队</h3>
            <p>团队拓扑</p>
          </div>
          {#if accessCanCreateTeam}<button
              class="primary"
              type="button"
              aria-label="添加团队"
              on:click={onAddTeam}><Plus size={15} />添加团队</button
            >{/if}
        </div>
        {#if canViewPlatform && !selectedTeamId && !selectedProjectId}<button
            class:active={selected.kind === 'platform'}
            class="access-scope-root"
            type="button"
            on:click={selectPlatform}
            ><span class="scope-tree-icon"><UsersRound size={16} /></span><span
              class="scope-tree-label"
              ><strong>{platformName}</strong><small class="scope-tree-count"
                >{visibleAccessTeams.length} 个团队 · {visibleProjects.length} 个项目
                · {visibleAccessUsers.length} 位用户</small
              ></span
            ></button
          >{/if}
        <div class="access-scope-team-list">
          {#each visibleAccessTeams as team}
            {@const teamProjects = projects.filter(
              (project) => project.team_id === team.id
            )}
            {@const teamUsers = selectedProjectId
              ? (accessProjectUsers[selectedProjectId] ?? [])
              : (accessTeamUsers[team.id] ?? [])}
            {@const globallyScopedProjectTeam = teamProjects.some(
              (project) => project.id === selectedProjectId
            )}
            {@const expanded =
              teamAccessExpanded[team.id] || globallyScopedProjectTeam}
            <div class="access-scope-team-node">
              <div
                class:active={selected.kind === 'team' &&
                  selected.id === team.id}
                class="access-scope-team-row"
              >
                <button
                  class="scope-tree-expander"
                  type="button"
                  aria-label={`${expanded ? '折叠' : '展开'} ${team.name}`}
                  aria-expanded={expanded}
                  disabled={globallyScopedProjectTeam}
                  on:click={() => onToggleTeam(team.id)}
                  ><ChevronDown
                    size={14}
                    class={expanded ? 'expanded' : undefined}
                  /></button
                ><button
                  class="scope-tree-select"
                  type="button"
                  disabled={Boolean(selectedProjectId)}
                  on:click={() => selectTeam(team)}
                  ><EntityBrandIcon kind="Team" fallback={team.icon || 'lucide:UsersRound'} size={16} className="scope-tree-icon" /><span
                    ><strong>{team.name}</strong><small class="scope-tree-count"
                      >{teamProjects.length} 个项目 · {teamUsers.length} 位用户</small
                    ></span
                  ></button
                >
                <div class="scope-tree-actions">
                  {#if canManageTeam(team)}<button
                      class="icon-button"
                      type="button"
                      aria-label={`编辑团队 ${team.name}`}
                      data-tooltip="编辑团队"
                      on:click={() => onEditTeam(team)}
                      ><Pencil size={13} /></button
                    >{/if}
                </div>
              </div>
              {#if expanded}<div class="access-scope-project-list">
                  {#each teamProjects as project}<button
                      class:active={selected.kind === 'project' &&
                        selected.id === project.id}
                      class="access-scope-project-row"
                      type="button"
                      on:click={() => selectProject(project)}
                      ><EntityBrandIcon kind="Project" fallback={project.icon || 'lucide:FolderKanban'} size={14} className="access-project-icon" /><span
                        ><strong>{project.name}</strong><small
                          class="scope-tree-count"
                          >{memberCount(project)} 位用户</small
                        ></span
                      ></button
                    >{:else}<span class="directory-empty">暂无项目</span>{/each}
                </div>{/if}
            </div>
          {:else}<div class="directory-empty">没有匹配的团队</div>{/each}
        </div>
      </aside>

      <section class="access-user-desk panel">
        <div class="access-user-desk-head">
          <div class="access-user-desk-title">
            <div class="access-user-desk-title-line">
              <h3>成员</h3>
              <small class="access-heading-code">{selected.name}</small>
            </div>
            <p>当前组织级别内去重后的全部用户</p>
          </div>
          <div class="access-user-desk-actions">
            <SearchInput
              bind:value={memberQuery}
              className="access-member-search"
              width="260px"
              height="35px"
              fieldSearchable
              fieldOptions={memberSearchFields}
              placeholder="筛选成员或输入 name:张"
              ariaLabel="筛选当前成员"
              on:fields={(event) => (memberFieldValues = event.detail)}
            /><button
              class="secondary danger-action"
              type="button"
              on:click={() => onDisable('user', selectedAccessUserIds)}
              disabled={selectedAccessUserIds.length === 0 || busy}
              ><Trash2 size={14} />批量禁用</button
            >{#if accessCanCreateUser}<button
                class="primary"
                type="button"
                on:click={onAddUser}><Plus size={15} />添加用户</button
              >{/if}
          </div>
        </div>
        <div class="access-project-strip">
          {#each selectedProjects as project}<ScopeChip
              label={project.name}
              count={`${memberCount(project)} 人`}
              active={activeProjectID === project.id}
              ariaLabel={`筛选项目 ${project.name}`}
              on:click={() => toggleProjectFilter(project.id)}
            />{:else}<span class="access-project-empty">当前范围暂无项目</span
            >{/each}
        </div>
        <div class="access-member-section">
          <ExpandableTable
            columns={memberColumns}
            rows={visibleScopedUsers}
            rowKey={(user) => user.id}
            expandable
            selectable
            selectedIds={selectedAccessUserIds}
            canSelectRow={(user) => canManageUser(user) && user.status === 'active'}
            onSelectionChange={(ids) => (selectedAccessUserIds = ids)}
            className="access-member-table"
            emptyText="当前范围没有可见成员。"
          >
            <svelte:fragment slot="cell" let:row let:column>
              {@const user = row as User}
              {@const roleSelection = activeProjectID ? ({ kind: 'project', id: activeProjectID, name: '' } as Selection) : selected}
              {@const scopeRoles = userScopeRoles(user.id, roleSelection)}
              {#if column.key === 'user'}
                  <div class="access-user-main">
                    <span class="avatar access-avatar">{initials(user)}</span
                    ><span
                      ><strong>{user.display_name || user.username}</strong><small
                        >@{user.username}</small
                      ></span
                    >
                  </div>
              {:else if column.key === 'contact'}
                  <div class="access-user-contact">
                    <span>{user.phone || '未填写电话'}</span>
                    <span>{user.email || '未填写邮箱'}</span>
                  </div>
              {:else if column.key === 'permissions'}
                  <div class="access-user-permissions">
                    {#if scopeRoles.length > 0}
                      {#each scopeRoles.slice(0, 2) as entry}
                        <div
                          class={`permission-card permission-${entry.scopeType} permission-role-${roleTone(entry.roleName)}`}
                          title={`${entry.scopeName} · ${entry.roleName}`}
                        >
                          <span class="permission-card-object">
                            <small>{scopeTypeLabel(entry.scopeType)}</small>
                            <strong>{entry.scopeName}</strong>
                          </span>
                          <strong class="permission-card-role">{entry.roleName}</strong>
                        </div>
                      {/each}
                      {#if scopeRoles.length > 2}
                        <span
                          class="permission-overflow"
                          title={`还有 ${scopeRoles.length - 2} 项权限`}
                          >+{scopeRoles.length - 2}</span
                        >
                      {/if}
                    {:else}
                      <span class="permission-empty">当前范围无角色</span>
                    {/if}
                  </div>
              {:else if column.key === 'status'}
                  <StatusBadge className="status-label" tone={user.status} label={statusLabel(user.status)} />
              {:else if column.key === 'actions'}
                  <div class="access-row-actions">
                    {#if canManageUser(user)}
                      <button
                        class="icon-button"
                        type="button"
                        aria-label={`编辑用户 ${user.display_name || user.username}`}
                        data-tooltip="编辑用户与授权"
                        on:click|stopPropagation={() => onEditUser(user)}
                        ><Pencil size={15} /></button
                      ><button
                        class="icon-button danger-action"
                        type="button"
                        aria-label={`禁用用户 ${user.display_name || user.username}`}
                        data-tooltip="禁用用户"
                        disabled={user.status !== 'active' ||
                          user.id === currentUser?.id}
                        on:click|stopPropagation={() =>
                          onDisable('user', [user.id])}
                        ><Trash2 size={15} /></button
                      >{:else}
                      <span class="read-only-label">
                        {user.id === currentUser?.id ? '当前账号' : '只读'}
                      </span>
                    {/if}
                  </div>
              {/if}
            </svelte:fragment>
            <svelte:fragment slot="details" let:row>
              {@const user = row as User}
              {@const roleSelection = activeProjectID ? ({ kind: 'project', id: activeProjectID, name: '' } as Selection) : selected}
              {@const scopeRoles = userScopeRoles(user.id, roleSelection)}
              {@const resourceScopeRoles = userResourceRoleBindings(user.id, roleSelection)}
              <div class="access-user-details">
                  <div class="access-user-details-grid">
                    <div><span>用户名</span><strong>{user.username}</strong></div>
                    <div><span>姓名</span><strong>{user.display_name || '未填写'}</strong></div>
                    <div><span>电话</span><strong>{user.phone || '未填写电话'}</strong></div>
                    <div><span>邮箱</span><strong>{user.email || '未填写邮箱'}</strong></div>
                    <div><span>密码状态</span><strong>{user.must_change_password ? '需要修改一次性密码' : '无需强制修改'}</strong></div>
                    <div><span>创建时间</span><strong>{dateTimeLabel(user.created_at)}</strong></div>
                    <div><span>更新时间</span><strong>{dateTimeLabel(user.updated_at)}</strong></div>
                    <div><span>状态</span><strong>{statusLabel(user.status)}</strong></div>
                  </div>
                  <section class="access-user-details-section access-user-roles-section">
                    <ReadonlyTable columns={roleColumns} rows={[...scopeRoles.map((entry) => ({ level: scopeTypeLabel(entry.scopeType), object: entry.scopeName, role: entry.roleName, extra: '—', tone: roleTone(entry.roleName) })), ...resourceScopeRoles.map((entry) => ({ level: '资源', object: `${entry.resource_kind} · ${entry.resource_name}`, role: '—', extra: resourceRoleLabel(entry.role_name), tone: roleTone(resourceRoleLabel(entry.role_name)) }))]} emptyText="当前范围无角色">
                      <svelte:fragment slot="cell" let:row let:column>
                        {@const roleRow = row as Record<string, string>}
                        <span class={column.key === 'role' || column.key === 'extra' ? `permission-role-${roleRow.tone}` : ''}>{roleRow[column.key]}</span>
                      </svelte:fragment>
                    </ReadonlyTable>
                  </section>
                </div>
            </svelte:fragment>
          </ExpandableTable>
        </div>
      </section>
    </div>
  {/if}
</section>
