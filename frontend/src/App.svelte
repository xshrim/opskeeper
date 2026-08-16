<script lang="ts">
  import { onMount } from 'svelte';
  import { fetchHealth, toStatusRows, type HealthReport } from './lib/health';
  import {
    api,
    ApiError,
    type Group,
    type Platform,
    type Project,
    type Relation,
    type Resource,
    type ResourceSchema,
    type RoleBinding,
    type RoleDefinition,
    type Team,
    type TopologyNode,
    type User
  } from './lib/api';

  type View = 'overview' | 'organization' | 'resources' | 'access';
  type ScopeChoice = {
    id: string;
    type: string;
    name: string;
    parentId?: string;
  };

  let authState: 'loading' | 'login' | 'ready' = 'loading';
  let currentUser: User | null = null;
  let email = '';
  let password = '';
  let loginError = '';
  let notice = '';
  let errorMessage = '';
  let busy = false;
  let view: View = 'overview';
  let platform: Platform | null = null;
  let teams: Team[] = [];
  let projects: Project[] = [];
  let resources: Resource[] = [];
  let schemas: ResourceSchema[] = [];
  let health: HealthReport | null = null;
  let selectedScopeId = '';
  let selectedResourceId = '';
  let relations: Relation[] = [];
  let topology: TopologyNode[] = [];
  let users: User[] = [];
  let groups: Group[] = [];
  let roles: RoleDefinition[] = [];
  let bindings: RoleBinding[] = [];
  let accessLoaded = false;

  let teamName = '';
  let teamCode = '';
  let teamIcon = 'team';
  let projectTeamId = '';
  let projectName = '';
  let projectCode = '';
  let projectIcon = 'project';
  let resourceKind = '';
  let resourceName = '';
  let resourceStatus = 'active';
  let resourceLabels = '';
  let resourceConfig = '{}';
  let resourceConfigValues: Record<string, string> = {};
  let resourceSensitiveValues: Record<string, string> = {};
  let editResourceName = '';
  let editResourceStatus = 'active';
  let editResourceLabels = '';
  let editResourceConfig = '{}';
  let editResourceSensitiveValues: Record<string, string> = {};
  let relationTarget = '';
  let relationType = 'depends_on';
  let groupScopeId = '';
  let groupName = '';
  let groupDescription = '';
  let bindingSubjectType = 'user';
  let bindingSubjectId = '';
  let bindingRoleId = '';
  let bindingScopeId = '';

  $: scopeChoices = buildScopeChoices(platform, teams, projects);
  $: activeScope =
    scopeChoices.find((scope) => scope.id === selectedScopeId) ??
    scopeChoices[0];
  $: visibleProjects = selectedScopeId
    ? projects.filter(
        (project) =>
          project.scope.id === selectedScopeId ||
          project.team_id === selectedScopeId
      )
    : projects;
  $: visibleResources = selectedScopeId
    ? resources.filter((resource) => resource.scope_id === selectedScopeId)
    : resources;
  $: selectedResource =
    resources.find((resource) => resource.id === selectedResourceId) ?? null;
  $: rows = toStatusRows(health);
  $: selectedSchema = schemas.find(
    (schema) => schema.kind === selectedResource?.kind
  );
  $: createSchema = schemas.find((schema) => schema.kind === resourceKind);

  onMount(() => {
    void bootstrap();
    const controller = new AbortController();
    const checkHealth = async () => {
      try {
        health = await fetchHealth(controller.signal);
      } catch {
        health = null;
      }
    };
    void checkHealth();
    const interval = window.setInterval(checkHealth, 15_000);
    return () => {
      controller.abort();
      window.clearInterval(interval);
    };
  });

  async function bootstrap() {
    try {
      currentUser = await api.me();
      authState = 'ready';
      await loadWorkspace();
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        authState = 'login';
      } else {
        authState = 'login';
        loginError = '无法恢复会话，请重新登录。';
      }
    }
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
      const projectPages = await Promise.all(
        teams.map((team) => api.projects(team.id))
      );
      projects = projectPages.flatMap((page) => page.items);
      selectedScopeId = selectedScopeId || platform.scope.id;
    } catch (error) {
      errorMessage = describeError(error, '工作区数据加载失败');
    }
  }

  async function login() {
    busy = true;
    loginError = '';
    try {
      currentUser = await api.login(email.trim(), password);
      password = '';
      authState = 'ready';
      await loadWorkspace();
    } catch (error) {
      loginError = describeError(error, '登录失败，请检查邮箱和密码');
    } finally {
      busy = false;
    }
  }

  async function logout() {
    busy = true;
    try {
      await api.logout();
    } finally {
      currentUser = null;
      authState = 'login';
      view = 'overview';
      busy = false;
    }
  }

  function chooseView(nextView: View) {
    view = nextView;
    notice = '';
    errorMessage = '';
    if (nextView === 'access' && !accessLoaded) void loadAccess();
  }

  async function loadAccess() {
    accessLoaded = true;
    try {
      const result = await Promise.allSettled([
        api.users(),
        api.groups(),
        api.roles(),
        api.bindings()
      ]);
      users = result[0].status === 'fulfilled' ? result[0].value : [];
      groups = result[1].status === 'fulfilled' ? result[1].value : [];
      roles = result[2].status === 'fulfilled' ? result[2].value : [];
      bindings = result[3].status === 'fulfilled' ? result[3].value : [];
      if (result.every((item) => item.status === 'rejected')) {
        errorMessage = '当前账号没有成员与角色管理权限。';
      }
    } catch {
      errorMessage = '成员和角色数据加载失败。';
    }
  }

  async function createTeam() {
    await action(async () => {
      const created = await api.createTeam({
        name: teamName,
        code: teamCode,
        icon: teamIcon,
        labels: {}
      });
      teams = [...teams, created];
      teamName = '';
      teamCode = '';
      teamIcon = 'team';
      notice = `团队“${created.name}”已创建`;
    });
  }

  async function createProject() {
    await action(async () => {
      const created = await api.createProject(projectTeamId, {
        name: projectName,
        code: projectCode,
        icon: projectIcon,
        labels: {}
      });
      projects = [...projects, created];
      projectName = '';
      projectCode = '';
      projectIcon = 'project';
      notice = `项目“${created.name}”已创建`;
    });
  }

  async function createResource() {
    await action(async () => {
      const config = buildSchemaConfig(
        createSchema,
        resourceConfigValues,
        resourceConfig
      );
      const credentialId = await createResourceCredential(
        createSchema,
        resourceSensitiveValues
      );
      const created = await api.createResource({
        scope_id: selectedScopeId,
        kind: resourceKind,
        name: resourceName,
        status: resourceStatus,
        labels: parseLabels(resourceLabels),
        config,
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      resources = [created, ...resources];
      selectedResourceId = created.id;
      resourceName = '';
      resourceLabels = '';
      resourceConfig = '{}';
      resourceConfigValues = {};
      resourceSensitiveValues = {};
      notice = `资源“${created.name}”已创建`;
      await loadResourceDetails(created.id);
    });
  }

  async function deleteSelectedResource() {
    if (!selectedResource) return;
    await action(async () => {
      await api.deleteResource(selectedResource.id);
      resources = resources.filter(
        (resource) => resource.id !== selectedResource.id
      );
      selectedResourceId = '';
      relations = [];
      topology = [];
      notice = '资源已停用并从当前列表移除';
    });
  }

  async function loadResourceDetails(id: string) {
    selectedResourceId = id;
    const resource = resources.find((item) => item.id === id);
    if (resource) syncResourceEditor(resource);
    try {
      const [loadedRelations, loadedTopology] = await Promise.all([
        api.relations(id),
        api.topology(id)
      ]);
      relations = loadedRelations;
      topology = loadedTopology.items;
    } catch (error) {
      errorMessage = describeError(error, '资源关系加载失败');
    }
  }

  async function updateSelectedResource() {
    if (!selectedResource) return;
    await action(async () => {
      const config = buildSchemaConfig(
        selectedSchema,
        resourceConfigValues,
        editResourceConfig
      );
      const credentialId = await createResourceCredential(
        selectedSchema,
        editResourceSensitiveValues
      );
      const updated = await api.updateResource(selectedResource.id, {
        name: editResourceName,
        status: editResourceStatus,
        labels: parseLabels(editResourceLabels),
        config,
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      resources = resources.map((resource) =>
        resource.id === updated.id ? updated : resource
      );
      syncResourceEditor(updated);
      notice = `资源“${updated.name}”已更新`;
    });
  }

  function syncResourceEditor(resource: Resource) {
    editResourceName = resource.name;
    editResourceStatus = resource.status;
    editResourceLabels = Object.entries(resource.labels ?? {})
      .map(([key, value]) => `${key}=${value}`)
      .join(', ');
    editResourceConfig = JSON.stringify(resource.config ?? {}, null, 2);
    resourceConfigValues = Object.fromEntries(
      Object.entries(resource.config ?? {}).map(([key, value]) => [
        key,
        String(value)
      ])
    );
    editResourceSensitiveValues = {};
  }

  function resetResourceConfig() {
    resourceConfigValues = {};
    resourceSensitiveValues = {};
    resourceConfig = '{}';
  }

  function buildSchemaConfig(
    schema: ResourceSchema | undefined,
    values: Record<string, string>,
    raw: string
  ) {
    if (!schema?.schema.properties)
      return JSON.parse(raw) as Record<string, unknown>;
    return Object.fromEntries(
      Object.entries(schema.schema.properties)
        .filter(
          ([key, field]) =>
            !field.sensitive && (values[key] ?? '').trim() !== ''
        )
        .map(([key, field]) => [
          key,
          normalizeFieldValue(values[key], field.type)
        ])
    );
  }

  function normalizeFieldValue(value: string, type?: string): unknown {
    if (type === 'integer' || type === 'number') return Number(value);
    if (type === 'boolean') return value === 'true';
    if (type === 'array')
      return value
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean);
    return value;
  }

  async function createResourceCredential(
    schema: ResourceSchema | undefined,
    values: Record<string, string>
  ) {
    const secret = Object.fromEntries(
      Object.entries(values).filter(([, value]) => value.trim() !== '')
    );
    const scopeId = selectedResource?.scope_id ?? selectedScopeId;
    if (!scopeId || !schema || Object.keys(secret).length === 0) return '';
    const credential = await api.createCredential({
      scope_id: scopeId,
      name: `${resourceName || selectedResource?.name || schema.display_name} connection`,
      purpose: `${schema.display_name} 敏感连接信息`,
      secret: JSON.stringify(secret)
    });
    return credential.id;
  }

  async function createRelation() {
    if (!selectedResource) return;
    await action(async () => {
      await api.createRelation(selectedResource.id, {
        target_resource_id: relationTarget,
        relation_type: relationType,
        attributes: {},
        confirmed: true
      });
      relationTarget = '';
      notice = '资源关系已建立';
      await loadResourceDetails(selectedResource.id);
    });
  }

  async function deleteRelation(relation: Relation) {
    if (!selectedResource) return;
    await action(async () => {
      await api.deleteRelation(selectedResource.id, relation.id);
      await loadResourceDetails(selectedResource.id);
      notice = '资源关系已删除';
    });
  }

  async function createGroup() {
    await action(async () => {
      const created = await api.createGroup({
        scope_id: groupScopeId,
        name: groupName,
        description: groupDescription
      });
      groups = [...groups, created];
      groupName = '';
      groupDescription = '';
      notice = `成员组“${created.name}”已创建`;
    });
  }

  async function createBinding() {
    await action(async () => {
      const created = await api.createBinding({
        subject_type: bindingSubjectType,
        subject_id: bindingSubjectId,
        role_id: bindingRoleId,
        scope_id: bindingScopeId
      });
      bindings = [...bindings, created];
      notice = '角色绑定已创建';
    });
  }

  async function deleteBinding(binding: RoleBinding) {
    await action(async () => {
      await api.deleteBinding(binding.id);
      bindings = bindings.filter((item) => item.id !== binding.id);
      notice = '角色绑定已删除';
    });
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
      return error.message || fallback;
    }
    if (error instanceof SyntaxError) return '配置必须是有效的 JSON 对象。';
    return fallback;
  }

  function buildScopeChoices(
    currentPlatform: Platform | null,
    currentTeams: Team[],
    currentProjects: Project[]
  ): ScopeChoice[] {
    const choices: ScopeChoice[] = currentPlatform
      ? [
          {
            id: currentPlatform.scope.id,
            type: 'platform',
            name: currentPlatform.name
          }
        ]
      : [];
    for (const team of currentTeams)
      choices.push({
        id: team.scope.id,
        type: 'team',
        name: team.name,
        parentId: team.platform_id
      });
    for (const project of currentProjects)
      choices.push({
        id: project.scope.id,
        type: 'project',
        name: project.name,
        parentId: project.team_id
      });
    return choices;
  }

  function scopeName(id: string) {
    return (
      scopeChoices.find((scope) => scope.id === id)?.name ?? id.slice(0, 8)
    );
  }

  function scopeType(id: string) {
    return scopeChoices.find((scope) => scope.id === id)?.type ?? 'scope';
  }

  function parseLabels(value: string): Record<string, string> {
    return Object.fromEntries(
      value
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean)
        .map((item) => {
          const [key, ...rest] = item.split('=');
          return [key.trim(), rest.join('=').trim()];
        })
    );
  }

  function formatDate(value: string) {
    return new Date(value).toLocaleString('zh-CN', {
      dateStyle: 'medium',
      timeStyle: 'short'
    });
  }

  function iconGlyph(icon: string | undefined) {
    const glyphs: Record<string, string> = {
      platform: '▣',
      team: '♟',
      project: '▰',
      kubernetes: '☸',
      application: '⌘',
      endpoint: '↗',
      schedule: '◷',
      postgresql: '◉',
      redis: '◒',
      kafka: '◫',
      search: '⌕',
      middleware: '◇',
      llm: '✦',
      mcp: '⌁',
      skill: '✧',
      metrics: '▥',
      logs: '≋',
      traces: '⌁',
      observability: '◌',
      api: '⇄',
      notification: '♢',
      runbook: '☷',
      storage: '▤',
      credential: '⚿',
      resource: '◇'
    };
    return glyphs[icon ?? 'resource'] ?? icon?.slice(0, 1).toUpperCase() ?? '◇';
  }

  function schemaName(schema: ResourceSchema) {
    return schema.display_name || schema.kind;
  }

  function resourceSchemaName(kind: string) {
    const schema = schemas.find((item) => item.kind === kind);
    return schema ? schemaName(schema) : kind;
  }
</script>

<svelte:head>
  <meta name="description" content="OpsKeeper platform control plane" />
</svelte:head>

{#if authState === 'loading'}
  <div class="loading-screen">
    <span class="spinner"></span>
    <p>正在恢复工作区会话…</p>
  </div>
{:else if authState === 'login'}
  <main class="login-shell">
    <section class="login-panel" aria-labelledby="login-heading">
      <div class="brand large">
        <span class="brand-mark" aria-hidden="true">O</span><span
          >OpsKeeper</span
        >
      </div>
      <p class="eyebrow">CONTROL PLANE</p>
      <h1 id="login-heading">登录管理控制台</h1>
      <p class="muted">使用本地账号访问组织、权限和资源工作区。</p>
      {#if loginError}<div class="alert error" role="alert">
          {loginError}
        </div>{/if}
      <form class="stack-form" on:submit|preventDefault={login}>
        <label
          >邮箱<input
            type="email"
            bind:value={email}
            autocomplete="username"
            required
            placeholder="admin@example.com"
          /></label
        >
        <label
          >密码<input
            type="password"
            bind:value={password}
            autocomplete="current-password"
            required
          /></label
        >
        <button class="primary full" type="submit" disabled={busy}
          >{busy ? '登录中…' : '登录'}</button
        >
      </form>
    </section>
  </main>
{:else}
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark" aria-hidden="true">O</span><span
          >OpsKeeper</span
        >
      </div>
      <div class="workspace-label">WORKSPACE</div>
      <nav aria-label="主导航">
        <button
          class:active={view === 'overview'}
          class="nav-item"
          on:click={() => chooseView('overview')}
          ><span aria-hidden="true">⌂</span>总览</button
        >
        <button
          class:active={view === 'organization'}
          class="nav-item"
          on:click={() => chooseView('organization')}
          ><span aria-hidden="true">▦</span>组织</button
        >
        <button
          class:active={view === 'resources'}
          class="nav-item"
          on:click={() => chooseView('resources')}
          ><span aria-hidden="true">◇</span>资源</button
        >
        <button
          class:active={view === 'access'}
          class="nav-item"
          on:click={() => chooseView('access')}
          ><span aria-hidden="true">♙</span>成员与角色</button
        >
      </nav>
      <div class="sidebar-footer">
        <span class="status-dot"></span><span
          >{health?.status === 'ready' ? '服务正常' : '检查服务状态'}</span
        >
      </div>
    </aside>

    <main class="main-content">
      <header class="topbar">
        <div>
          <p class="breadcrumb">
            {activeScope?.name ?? 'Platform'} / {view === 'overview'
              ? 'Overview'
              : view === 'organization'
                ? 'Organization'
                : view === 'resources'
                  ? 'Resources'
                  : 'Access'}
          </p>
          <h1>
            {view === 'overview'
              ? '平台总览'
              : view === 'organization'
                ? '组织管理'
                : view === 'resources'
                  ? '资源目录'
                  : '成员与角色'}
          </h1>
        </div>
        <div class="topbar-actions">
          <span class="user-chip"
            ><span class="avatar"
              >{(currentUser?.display_name || currentUser?.email || 'U')
                .slice(0, 1)
                .toUpperCase()}</span
            ><span>{currentUser?.display_name || currentUser?.email}</span
            ></span
          ><button class="quiet-button" on:click={logout} disabled={busy}
            >退出</button
          >
        </div>
      </header>

      <div class="scope-bar">
        <label class="scope-picker"
          ><span>当前作用域</span><select
            bind:value={selectedScopeId}
            on:change={() => {
              selectedResourceId = '';
            }}
            ><option value="" disabled>选择作用域</option
            >{#each scopeChoices as scope}<option value={scope.id}
                >{scope.type === 'platform'
                  ? '平台'
                  : scope.type === 'team'
                    ? '团队'
                    : '项目'} · {scope.name}</option
              >{/each}</select
          ></label
        >
        <div class="scope-trail">
          <span class="scope-type">{activeScope?.type ?? 'scope'}</span><span
            >{activeScope?.name ?? '未选择'}</span
          >
        </div>
      </div>

      {#if notice}<div class="alert success" role="status">{notice}</div>{/if}
      {#if errorMessage}<div class="alert error" role="alert">
          {errorMessage}
        </div>{/if}

      {#if view === 'overview'}
        <section class="content-grid">
          <div class="metric-grid">
            <article class="metric">
              <span class="metric-label">团队</span><strong
                >{teams.length}</strong
              ><span class="metric-note">可访问的组织单元</span>
            </article>
            <article class="metric">
              <span class="metric-label">项目</span><strong
                >{visibleProjects.length}</strong
              ><span class="metric-note">当前作用域内</span>
            </article>
            <article class="metric">
              <span class="metric-label">资源</span><strong
                >{visibleResources.length}</strong
              ><span class="metric-note">已登记资源</span>
            </article>
          </div>
          <section class="panel wide-panel" aria-labelledby="health-heading">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">SYSTEM</p>
                <h2 id="health-heading">控制平面状态</h2>
              </div>
              <span
                class:healthy={health?.status === 'ready'}
                class="status-pill"
                ><span class="status-dot"></span>{health?.status === 'ready'
                  ? 'Ready'
                  : 'Checking'}</span
              >
            </div>
            <div class="status-table">
              <div class="table-header">
                <span>服务</span><span>状态</span><span>延迟</span>
              </div>
              {#each rows as row}<div class="table-row">
                  <span class="service-name">{row.name}</span><span
                    class:up={row.status === 'up'}
                    class:down={row.status === 'down'}
                    class="service-status"
                    ><span class="status-dot"></span>{row.status === 'up'
                      ? 'Operational'
                      : row.status === 'down'
                        ? 'Unavailable'
                        : 'Checking'}</span
                  ><span class="latency">{row.latency ?? '—'}</span>
                </div>{/each}
            </div>
          </section>
          <section class="panel recent-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">CATALOG</p>
                <h2>最近资源</h2>
              </div>
              <button
                class="text-button"
                on:click={() => chooseView('resources')}>查看全部 →</button
              >
            </div>
            {#if visibleResources.length === 0}<div class="empty-state">
                当前作用域还没有资源。
              </div>{:else}<div class="compact-list">
                {#each visibleResources.slice(0, 5) as resource}<button
                    class="compact-row"
                    on:click={() => {
                      selectedResourceId = resource.id;
                      chooseView('resources');
                      void loadResourceDetails(resource.id);
                    }}
                    ><span class="entity-summary"
                      ><span class="entity-icon resource-icon"
                        >{iconGlyph(
                          schemas.find(
                            (schema) => schema.kind === resource.kind
                          )?.icon
                        )}</span
                      ><span
                        ><strong>{resource.name}</strong><small
                          >{resourceSchemaName(resource.kind)} · {scopeName(
                            resource.scope_id
                          )}</small
                        ></span
                      ></span
                    ><span class="status-label {resource.status}"
                      >{resource.status}</span
                    ></button
                  >{/each}
              </div>{/if}
          </section>
        </section>
      {:else if view === 'organization'}
        <section class="content-grid two-column">
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">STRUCTURE</p>
                <h2>团队</h2>
              </div>
              <span class="count">{teams.length}</span>
            </div>
            <div class="table-list">
              {#each teams as team}<button
                  class:selected={selectedScopeId === team.scope.id}
                  class="list-row"
                  on:click={() => {
                    selectedScopeId = team.scope.id;
                    projectTeamId = team.id;
                  }}
                  ><span class="entity-summary"
                    ><span class="entity-icon team-icon"
                      >{iconGlyph(team.icon)}</span
                    ><span
                      ><strong>{team.name}</strong><small
                        >{team.code} · {team.status}</small
                      ></span
                    ></span
                  ><span class="row-arrow">→</span></button
                >{:else}<div class="empty-state">暂无团队</div>{/each}
            </div>
            <form class="inline-form" on:submit|preventDefault={createTeam}>
              <input
                bind:value={teamIcon}
                placeholder="图标，如 team 或 ♟"
                aria-label="团队图标"
              />
              <input
                bind:value={teamName}
                required
                placeholder="团队名称"
                aria-label="团队名称"
              /><input
                bind:value={teamCode}
                required
                placeholder="编码"
                aria-label="团队编码"
              /><button class="primary" disabled={busy}>新增团队</button>
            </form>
          </section>
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">STRUCTURE</p>
                <h2>项目</h2>
              </div>
              <span class="count">{visibleProjects.length}</span>
            </div>
            <div class="table-list">
              {#each visibleProjects as project}<button
                  class:selected={selectedScopeId === project.scope.id}
                  class="list-row"
                  on:click={() => (selectedScopeId = project.scope.id)}
                  ><span class="entity-summary"
                    ><span class="entity-icon project-icon"
                      >{iconGlyph(project.icon)}</span
                    ><span
                      ><strong>{project.name}</strong><small
                        >{project.code} · {scopeName(project.team_id)}</small
                      ></span
                    ></span
                  ><span class="status-label {project.status}"
                    >{project.status}</span
                  ></button
                >{:else}<div class="empty-state">当前作用域暂无项目</div>{/each}
            </div>
            <form
              class="stack-form compact-form"
              on:submit|preventDefault={createProject}
            >
              <label
                >所属团队<select bind:value={projectTeamId} required
                  ><option value="" disabled>选择团队</option
                  >{#each teams as team}<option value={team.id}
                      >{team.name}</option
                    >{/each}</select
                ></label
              >
              <div class="form-row">
                <input
                  bind:value={projectIcon}
                  placeholder="图标，如 project 或 ▰"
                  aria-label="项目图标"
                />
                <input
                  bind:value={projectName}
                  required
                  placeholder="项目名称"
                  aria-label="项目名称"
                /><input
                  bind:value={projectCode}
                  required
                  placeholder="编码"
                  aria-label="项目编码"
                />
              </div>
              <button class="primary" disabled={busy || !projectTeamId}
                >新增项目</button
              >
            </form>
          </section>
        </section>
      {:else if view === 'resources'}
        <section class="resources-layout">
          <section class="panel resource-list-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">CATALOG</p>
                <h2>资源目录</h2>
              </div>
              <span class="count">{visibleResources.length}</span>
            </div>
            <div class="filter-row">
              <select bind:value={resourceKind}
                ><option value="">全部类型</option
                >{#each schemas as schema}<option value={schema.kind}
                    >{schemaName(schema)}</option
                  >{/each}</select
              >
            </div>
            <div class="table-list resource-list">
              {#each visibleResources.filter((item) => !resourceKind || item.kind === resourceKind) as resource}<button
                  class:selected={selectedResourceId === resource.id}
                  class="list-row"
                  on:click={() => void loadResourceDetails(resource.id)}
                  ><span class="entity-summary"
                    ><span class="entity-icon resource-icon"
                      >{iconGlyph(
                        schemas.find((schema) => schema.kind === resource.kind)
                          ?.icon
                      )}</span
                    ><span
                      ><strong>{resource.name}</strong><small
                        >{resourceSchemaName(resource.kind)} · {scopeName(
                          resource.scope_id
                        )}</small
                      ></span
                    ></span
                  ><span class="status-label {resource.status}"
                    >{resource.status}</span
                  ></button
                >{:else}<div class="empty-state">没有匹配的资源。</div>{/each}
            </div>
          </section>
          <section class="resource-workspace">
            <section class="panel">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">NEW RESOURCE</p>
                  <h2>登记资源</h2>
                </div>
                <span class="scope-type">{activeScope?.type ?? 'scope'}</span>
              </div>
              <form
                class="stack-form"
                on:submit|preventDefault={createResource}
              >
                <div class="resource-type-picker" aria-label="选择资源类型">
                  <div class="resource-type-grid">
                    {#each schemas as schema}
                      <button
                        type="button"
                        class="resource-type-card"
                        class:selected={resourceKind === schema.kind}
                        on:click={() => {
                          resourceKind = schema.kind;
                          resetResourceConfig();
                        }}
                      >
                        <span class="type-icon">{iconGlyph(schema.icon)}</span>
                        <span>
                          <strong>{schemaName(schema)}</strong>
                          <small
                            >{schema.description || '资源连接与运行信息'}</small
                          >
                        </span>
                      </button>
                    {/each}
                  </div>
                </div>
                <label
                  >名称<input
                    bind:value={resourceName}
                    required
                    placeholder="例如 production-postgres"
                  /></label
                >
                <div class="form-row">
                  <label
                    >状态<select bind:value={resourceStatus}
                      ><option value="active">active</option><option
                        value="disabled">disabled</option
                      ><option value="unknown">unknown</option></select
                    ></label
                  ><label
                    >标签<input
                      bind:value={resourceLabels}
                      placeholder="env=prod, owner=platform"
                    /></label
                  >
                </div>
                {#if createSchema?.schema.properties}
                  <div class="schema-inputs">
                    <p class="eyebrow">SCHEMA FIELDS</p>
                    {#each Object.entries(createSchema.schema.properties) as [key, field]}
                      <label
                        >{field.title || key}{#if field.sensitive}<input
                            type="password"
                            bind:value={resourceSensitiveValues[key]}
                            placeholder="敏感信息将加密保存"
                          />{:else if field.enum}<select
                            bind:value={resourceConfigValues[key]}
                            ><option value="">未设置</option
                            >{#each field.enum as option}<option value={option}
                                >{option}</option
                              >{/each}</select
                          >{:else}<input
                            bind:value={resourceConfigValues[key]}
                            type={field.type === 'number' ||
                            field.type === 'integer'
                              ? 'number'
                              : field.type === 'url'
                                ? 'url'
                                : 'text'}
                            placeholder={field.description || key}
                          />{/if}</label
                      >
                    {/each}
                  </div>
                {:else}
                  <label
                    >配置 JSON<textarea
                      bind:value={resourceConfig}
                      rows="4"
                      spellcheck="false"
                    ></textarea></label
                  >
                {/if}<button class="primary" disabled={busy || !selectedScopeId}
                  >创建资源</button
                >
              </form>
            </section>
            {#if selectedResource}<section class="panel detail-panel">
                <div class="panel-heading">
                  <div>
                    <p class="eyebrow">RESOURCE DETAIL</p>
                    <h2>{selectedResource.name}</h2>
                    <p class="muted">
                      {selectedResource.kind} · {scopeName(
                        selectedResource.scope_id
                      )}
                    </p>
                  </div>
                  <button
                    class="danger-button"
                    on:click={deleteSelectedResource}
                    disabled={busy}>停用</button
                  >
                </div>
                <div class="detail-meta">
                  <span>状态 <strong>{selectedResource.status}</strong></span
                  ><span>Schema v{selectedResource.schema_version}</span><span
                    >更新于 {formatDate(selectedResource.updated_at)}</span
                  >
                </div>
                <form
                  class="stack-form editor-form"
                  on:submit|preventDefault={updateSelectedResource}
                >
                  <div class="form-row">
                    <label
                      >名称<input
                        bind:value={editResourceName}
                        required
                      /></label
                    ><label
                      >状态<select bind:value={editResourceStatus}
                        ><option value="active">active</option><option
                          value="disabled">disabled</option
                        ><option value="unknown">unknown</option></select
                      ></label
                    >
                  </div>
                  <label
                    >标签<input
                      bind:value={editResourceLabels}
                      placeholder="env=prod, owner=platform"
                    /></label
                  >
                  {#if selectedSchema?.schema.properties}
                    <div class="schema-inputs">
                      <p class="eyebrow">SCHEMA FIELDS</p>
                      {#each Object.entries(selectedSchema.schema.properties) as [key, field]}
                        <label
                          >{field.title || key}{#if field.sensitive}<input
                              type="password"
                              bind:value={editResourceSensitiveValues[key]}
                              placeholder={selectedResource.credential_id
                                ? '已有关联凭据，留空保持不变'
                                : '敏感信息将加密保存'}
                            />{:else if field.enum}<select
                              bind:value={resourceConfigValues[key]}
                              ><option value="">未设置</option
                              >{#each field.enum as option}<option
                                  value={option}>{option}</option
                                >{/each}</select
                            >{:else}<input
                              bind:value={resourceConfigValues[key]}
                              type={field.type === 'number' ||
                              field.type === 'integer'
                                ? 'number'
                                : field.type === 'url'
                                  ? 'url'
                                  : 'text'}
                            />{/if}</label
                        >
                      {/each}
                    </div>
                  {:else}
                    <label
                      >配置 JSON<textarea
                        bind:value={editResourceConfig}
                        rows="4"
                        spellcheck="false"
                      ></textarea></label
                    >
                  {/if}
                  <button class="secondary" disabled={busy}>保存修改</button>
                </form>
                {#if selectedSchema?.schema.properties}<div
                    class="schema-fields"
                  >
                    <p class="eyebrow">SCHEMA FIELDS</p>
                    {#each Object.entries(selectedSchema.schema.properties) as [key, field]}<div
                      >
                        <span>{field.title || key}</span><code
                          >{field.sensitive
                            ? selectedResource.credential_id
                              ? '已由加密凭据保存'
                              : '未设置'
                            : String(
                                selectedResource.config[key] ?? '未设置'
                              )}</code
                        >
                      </div>{/each}
                  </div>{:else}<pre class="config-preview">{JSON.stringify(
                      selectedResource.config,
                      null,
                      2
                    )}</pre>{/if}
                <div class="relation-section">
                  <div class="subheading">
                    <h3>关系与拓扑</h3>
                    <span
                      >{relations.length} 条关系 · {topology.length} 个节点</span
                    >
                  </div>
                  <form
                    class="relation-form"
                    on:submit|preventDefault={createRelation}
                  >
                    <select bind:value={relationTarget} required
                      ><option value="" disabled>选择目标资源</option
                      >{#each resources.filter((item) => item.id !== selectedResource.id) as resource}<option
                          value={resource.id}
                          >{resource.name} · {resource.kind}</option
                        >{/each}</select
                    ><select bind:value={relationType}
                      ><option value="depends_on">depends_on</option><option
                        value="contains">contains</option
                      ><option value="deployed_on">deployed_on</option><option
                        value="exposes">exposes</option
                      ><option value="uses_provider">uses_provider</option
                      ></select
                    ><button class="secondary" disabled={busy}>建立关系</button>
                  </form>
                  {#if relations.length}<div class="relation-list">
                      {#each relations as relation}<div class="relation-row">
                          <span
                            ><strong>{relation.relation_type}</strong><small
                              >{resources.find(
                                (item) =>
                                  item.id ===
                                  (relation.source_resource_id ===
                                  selectedResource.id
                                    ? relation.target_resource_id
                                    : relation.source_resource_id)
                              )?.name ?? '关联资源'}</small
                            ></span
                          ><button
                            class="icon-button"
                            title="删除关系"
                            aria-label="删除关系"
                            on:click={() => deleteRelation(relation)}>×</button
                          >
                        </div>{/each}
                    </div>{/if}{#if topology.length}<div class="topology-list">
                      {#each topology as node}<span
                          class="topology-node"
                          style={`--depth: ${node.depth}`}
                          >{node.depth} · {node.resource.name}</span
                        >{/each}
                    </div>{/if}
                </div>
              </section>{:else}<section class="panel empty-detail">
                <div class="empty-state">
                  <span class="empty-icon">◇</span>
                  <h2>选择一个资源</h2>
                  <p>从左侧目录选择资源查看作用域、配置和关系。</p>
                </div>
              </section>{/if}
          </section>
        </section>
      {:else}
        <section class="content-grid two-column">
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">PEOPLE</p>
                <h2>用户</h2>
              </div>
              <span class="count">{users.length}</span>
            </div>
            <div class="table-list">
              {#each users as user}<div class="list-row static">
                  <span
                    ><strong>{user.display_name || user.email}</strong><small
                      >{user.email}</small
                    ></span
                  ><span class="status-label {user.status}">{user.status}</span>
                </div>{:else}<div class="empty-state">
                  没有可见用户，或当前账号没有管理权限。
                </div>{/each}
            </div>
          </section>
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">GROUPS</p>
                <h2>成员组</h2>
              </div>
              <span class="count">{groups.length}</span>
            </div>
            <div class="table-list">
              {#each groups as group}<div class="list-row static">
                  <span
                    ><strong>{group.name}</strong><small
                      >{group.description || '无描述'} · {scopeName(
                        group.scope_id
                      )}</small
                    ></span
                  ><span class="status-label {group.status}"
                    >{group.status}</span
                  >
                </div>{:else}<div class="empty-state">暂无成员组</div>{/each}
            </div>
            <form
              class="stack-form compact-form"
              on:submit|preventDefault={createGroup}
            >
              <label
                >作用域<select bind:value={groupScopeId} required
                  ><option value="" disabled>选择作用域</option
                  >{#each scopeChoices as scope}<option value={scope.id}
                      >{scope.name}</option
                    >{/each}</select
                ></label
              ><input
                bind:value={groupName}
                required
                placeholder="成员组名称"
              /><input
                bind:value={groupDescription}
                placeholder="描述"
              /><button class="primary" disabled={busy}>新增成员组</button>
            </form>
          </section>
          <section class="panel wide-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">AUTHORIZATION</p>
                <h2>角色绑定</h2>
              </div>
              <span class="count">{bindings.length}</span>
            </div>
            <div class="table-list">
              {#each bindings as binding}<div class="list-row static">
                  <span
                    ><strong>{binding.role_name}</strong><small
                      >{binding.subject_type} · {binding.subject_id.slice(0, 8)} ·
                      {scopeName(binding.scope_id)}</small
                    ></span
                  ><button
                    class="icon-button"
                    title="删除绑定"
                    aria-label="删除绑定"
                    on:click={() => deleteBinding(binding)}>×</button
                  >
                </div>{:else}<div class="empty-state">
                  暂无可见角色绑定
                </div>{/each}
            </div>
            <form class="binding-form" on:submit|preventDefault={createBinding}>
              <select bind:value={bindingSubjectType}
                ><option value="user">用户</option><option value="group"
                  >成员组</option
                ></select
              ><input
                bind:value={bindingSubjectId}
                required
                placeholder="主体 ID"
                aria-label="主体 ID"
              /><select bind:value={bindingRoleId} required
                ><option value="" disabled>角色</option
                >{#each roles as role}<option value={role.id}
                    >{role.name} · {role.scope_type}</option
                  >{/each}</select
              ><select bind:value={bindingScopeId} required
                ><option value="" disabled>作用域</option
                >{#each scopeChoices as scope}<option value={scope.id}
                    >{scope.name}</option
                  >{/each}</select
              ><button class="secondary" disabled={busy}>绑定角色</button>
            </form>
          </section>
        </section>
      {/if}
    </main>
  </div>
{/if}
