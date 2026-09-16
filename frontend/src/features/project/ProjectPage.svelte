<script lang="ts">
  import { Plus, Upload, Sparkles, ChevronDown, Server, AlertTriangle, Network, Pencil } from 'lucide-svelte';
  import { api, ApiError, type Application, type Project, type ProjectWorkspace, type Team } from '../../lib/api';
  import IconPicker from '../../components/IconPicker.svelte';
  import IconValue from '../../components/IconValue.svelte';

  export let teams: Team[] = [];
  export let visibleProjects: Project[] = [];
  export let selectedScopeId = '';
  export let selectedProjectId = '';
  export let busy = false;
  export let onSelectTeam: (team: Team) => void;
  export let onSelectProject: (project: Project) => void;
  export let onOpenTeamDialog: () => void;
  export let onProjectCreated: (project: Project) => void;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;
  export let onOpenDiagnosis: (project: Project) => void;
  export let onOpenApplicationDiagnosis: (project: Project, application: Application) => void = () => {};

  let projectTeamId = '';
  let projectName = '';
  let projectCode = '';
  let projectIcon = 'FolderKanban';
  let creatingProject = false;
  let workspace: ProjectWorkspace | null = null;
  let loadingWorkspace = false;
  let expanded = new Set<string>();
  let showApplicationForm = false;
  let applicationName = '';
  let applicationCode = '';
  let applicationDescription = '';
  let applicationIcon = 'AppWindow';
  let creatingApplication = false;
  let editingAppId = '';
  let editName = '';
  let editDescription = '';
  let editIcon = 'AppWindow';
  let relationAppId = '';
  let relationMode: 'instance' | 'dependency' = 'instance';
  let relationName = '';
  let relationResourceId = '';
  let relationRuntime = 'virtual_machine';
  let relationKind = 'database';
  let relationBinding = '';
  let savingRelation = false;
  let importInput: HTMLInputElement;
  let projectImportInput: HTMLInputElement;

  $: if (!projectTeamId && teams[0]) projectTeamId = teams[0].id;
  $: selectedProject = visibleProjects.find((item) => item.id === selectedProjectId) ?? null;
  $: if (selectedProjectId) void loadWorkspace(selectedProjectId);

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) return error.message || fallback;
    return error instanceof Error ? error.message || fallback : fallback;
  }
  async function loadWorkspace(id: string) {
    loadingWorkspace = true;
    try { workspace = await api.projectWorkspace(id); } catch (error) { onError(describeError(error, '项目工作台加载失败')); } finally { loadingWorkspace = false; }
  }
  async function createProject() {
    if (!projectTeamId) return;
    creatingProject = true; onError('');
    try { const created = await api.createProject(projectTeamId, { name: projectName, code: projectCode, icon: projectIcon, labels: {} }); onProjectCreated(created); onSelectProject(created); projectName = ''; projectCode = ''; projectIcon = 'FolderKanban'; onNotice(`项目“${created.name}”已创建`); }
    catch (error) { onError(describeError(error, '创建项目失败')); } finally { creatingProject = false; }
  }
  async function createApplication() {
    if (!selectedProject) return;
    creatingApplication = true; onError('');
    try { await api.createApplication(selectedProject.id, { name: applicationName, code: applicationCode, description: applicationDescription, icon: applicationIcon, labels: {} }); showApplicationForm = false; applicationName = ''; applicationCode = ''; applicationDescription = ''; applicationIcon = 'AppWindow'; await loadWorkspace(selectedProject.id); onNotice('应用已添加'); }
    catch (error) { onError(describeError(error, '添加应用失败')); } finally { creatingApplication = false; }
  }
  function toggle(app: Application) { const next = new Set(expanded); next.has(app.id) ? next.delete(app.id) : next.add(app.id); expanded = next; }
  function beginEdit(app: Application) { editingAppId = app.id; editName = app.name; editDescription = app.description; editIcon = app.icon; }
  async function saveEdit(app: Application) { try { await api.updateApplication(selectedProjectId, app.id, { name: editName, description: editDescription, icon: editIcon }); editingAppId = ''; await loadWorkspace(selectedProjectId); onNotice('应用已更新'); } catch (error) { onError(describeError(error, '更新应用失败')); } }
  function beginRelation(app: Application, mode: 'instance' | 'dependency') { relationAppId = app.id; relationMode = mode; relationName = ''; relationResourceId = workspace?.resources[0]?.id ?? ''; relationBinding = ''; }
  async function saveRelation() {
    if (!relationAppId || !relationResourceId) return;
    savingRelation = true;
    try {
      const binding = relationBinding.trim() ? JSON.parse(relationBinding) : {};
      if (relationMode === 'instance') await api.createApplicationInstance(selectedProjectId, relationAppId, { name: relationName || 'default', runtime_kind: relationRuntime, target_resource_id: relationResourceId, selector: binding });
      else await api.createApplicationDependency(selectedProjectId, relationAppId, { target_resource_id: relationResourceId, dependency_kind: relationKind, binding, required: true });
      relationAppId = ''; await loadWorkspace(selectedProjectId); onNotice('关联已保存');
    } catch (error) { onError(describeError(error, '保存关联失败')); } finally { savingRelation = false; }
  }
  function importApplication(event: Event) {
    const file = (event.target as HTMLInputElement).files?.[0]; if (!file || !selectedProject) return;
    const reader = new FileReader(); reader.onload = async () => { try { await api.importApplication(selectedProject.id, JSON.parse(String(reader.result))); await loadWorkspace(selectedProject.id); onNotice('应用已导入'); } catch (error) { onError(describeError(error, '导入应用失败')); } }; reader.readAsText(file); (event.target as HTMLInputElement).value = '';
  }
  function importProject(event: Event) {
    const file = (event.target as HTMLInputElement).files?.[0]; if (!file) return;
    const reader = new FileReader(); reader.onload = async () => { try { const body = JSON.parse(String(reader.result)); const teamId = body.team_id || projectTeamId; const created = await api.createProject(teamId, body); onProjectCreated(created); onSelectProject(created); onNotice('项目已导入'); } catch (error) { onError(describeError(error, '导入项目失败')); } }; reader.readAsText(file); (event.target as HTMLInputElement).value = '';
  }
</script>

<section class="project-layout">
  <aside class="project-nav panel">
    <div class="panel-heading"><div><p class="eyebrow">WORKSPACE</p><h2>项目</h2></div><span class="count">{visibleProjects.length}</span></div>
    <div class="table-list team-list">
      {#each teams as team}
        <button class:selected={selectedScopeId === team.scope.id} class="list-row" on:click={() => onSelectTeam(team)}><span class="entity-summary"><span class="entity-icon team-icon"><IconValue value={team.icon} size={16} /></span><span><strong>{team.name}</strong><small>{team.code}</small></span></span><span class="row-arrow">›</span></button>
      {/each}
    </div>
    <div class="project-list">
      {#each visibleProjects as project}
        <button class:selected={selectedProjectId === project.id} class="project-row" on:click={() => onSelectProject(project)}><span class="entity-icon project-icon"><IconValue value={project.icon} size={17} /></span><span><strong>{project.name}</strong><small>{project.code}</small></span><span class="status-dot {project.status}"></span></button>
      {:else}<div class="empty-state">暂无项目</div>{/each}
    </div>
    <div class="inline-form"><button class="primary" type="button" disabled={busy} on:click={onOpenTeamDialog}><Plus size={15} />添加团队</button><input bind:this={projectImportInput} type="file" accept="application/json" hidden on:change={importProject} /><button class="secondary" type="button" on:click={() => projectImportInput?.click()}><Upload size={14} />导入项目</button></div>
    <form class="stack-form compact-form" on:submit|preventDefault={createProject}><label>所属团队<select bind:value={projectTeamId} required><option value="" disabled>选择团队</option>{#each teams as team}<option value={team.id}>{team.name}</option>{/each}</select></label><div class="form-row"><input bind:value={projectName} required placeholder="项目名称" /><input bind:value={projectCode} required placeholder="编码" /></div><div class="icon-field"><IconPicker value={projectIcon} onSelect={(icon) => (projectIcon = icon)} ariaLabel="选择项目图标" /><span>项目图标</span></div><button class="secondary" disabled={creatingProject || !projectTeamId}><Plus size={14} />新增项目</button></form>
  </aside>

  <section class="project-workspace">
    {#if !selectedProject}
      <div class="panel empty-workspace"><Network size={28} /><h2>选择一个项目</h2><p>项目的应用、资源依赖和运行状态将在这里集中展示。</p></div>
    {:else}
      <header class="workspace-header"><div><p class="eyebrow">PROJECT / {selectedProject.code}</p><h1>{selectedProject.name}</h1><p>应用与运行资源工作台</p></div><div class="header-actions"><input bind:this={importInput} type="file" accept="application/json" hidden on:change={importApplication} /><button class="secondary" on:click={() => importInput?.click()}><Upload size={15} />导入应用</button><button class="primary" on:click={() => (showApplicationForm = !showApplicationForm)}><Plus size={15} />添加应用</button><button class="diagnose" title="进入 AI 诊断" on:click={() => onOpenDiagnosis(selectedProject)}><Sparkles size={15} />AI 诊断</button></div></header>
      <div class="stat-grid">{#each [['应用', workspace?.summary.applications ?? 0], ['实例', workspace?.summary.instances ?? 0], ['关联资源', workspace?.summary.resources ?? 0], ['依赖', workspace?.summary.dependencies ?? 0], ['告警', workspace?.summary.alerts ?? 0]] as stat}<div class="stat"><span>{stat[0]}</span><strong>{stat[1]}</strong></div>{/each}</div>
      {#if showApplicationForm}<form class="panel app-form" on:submit|preventDefault={createApplication}><div class="form-row"><label>应用名称<input bind:value={applicationName} required placeholder="payments-api" /></label><label>编码<input bind:value={applicationCode} required pattern="[a-z0-9][a-z0-9-]*" placeholder="payments-api" /></label><div class="icon-field"><IconPicker value={applicationIcon} onSelect={(icon) => (applicationIcon = icon)} ariaLabel="选择应用图标" /><span>应用图标</span></div></div><label>描述<textarea bind:value={applicationDescription} rows="2" placeholder="应用职责与边界"></textarea></label><div class="form-actions"><button class="secondary" type="button" on:click={() => (showApplicationForm = false)}>取消</button><button class="primary" disabled={creatingApplication}>保存应用</button></div></form>{/if}
      {#if relationAppId}<form class="panel app-form" on:submit|preventDefault={saveRelation}><div class="form-row"><label>关联资源<select bind:value={relationResourceId} required>{#each workspace?.resources ?? [] as resource}<option value={resource.id}>{resource.name} · {resource.kind}</option>{/each}</select></label>{#if relationMode === 'instance'}<label>实例名称<input bind:value={relationName} placeholder="default" /></label>{:else}<label>依赖类型<select bind:value={relationKind}><option value="database">数据库</option><option value="messaging">消息队列</option><option value="cache">缓存</option><option value="configuration">配置中心</option><option value="repository">代码仓库</option><option value="storage">存储</option></select></label>{/if}</div>{#if relationMode === 'instance'}<label>运行方式<select bind:value={relationRuntime}><option value="virtual_machine">虚拟机</option><option value="containerized">容器</option><option value="cloud_native">Kubernetes</option></select></label>{/if}<label>绑定字段 JSON<textarea bind:value={relationBinding} rows="2" placeholder="例如 namespace 或 topic 的 JSON"></textarea></label><div class="form-actions"><button class="secondary" type="button" on:click={() => (relationAppId = '')}>取消</button><button class="primary" disabled={savingRelation}>保存关联</button></div></form>{/if}
      <div class="workspace-columns"><div class="workspace-side"><section class="panel side-section"><div class="panel-heading"><div><p class="eyebrow">DEPENDENCIES</p><h2>关联资源</h2></div><Server size={17} /></div>{#each workspace?.resources ?? [] as resource}<div class="resource-line"><span class="resource-kind">{resource.kind}</span><span><strong>{resource.name}</strong><small>{resource.role}</small></span><span class="status-dot {resource.status}"></span></div>{:else}<div class="empty-state">暂无关联资源</div>{/each}</section><section class="panel side-section"><div class="panel-heading"><div><p class="eyebrow">SIGNALS</p><h2>告警</h2></div><AlertTriangle size={17} /></div>{#each workspace?.alerts ?? [] as alert}<div class="alert-line"><span class="severity {alert.severity}"></span><span>{alert.title}</span></div>{:else}<div class="empty-state">当前没有告警</div>{/each}</section></div>
        <section class="apps-section"><div class="section-heading"><div><p class="eyebrow">APPLICATIONS</p><h2>应用列表</h2></div><span class="muted">{workspace?.applications.length ?? 0} 个应用</span></div>{#if loadingWorkspace}<div class="panel loading">正在加载项目工作台...</div>{:else}{#each workspace?.applications ?? [] as app}<article class="panel app-card" class:expanded={expanded.has(app.id)}><div class="app-card-head"><button class="app-card-toggle" on:click={() => toggle(app)}><span class="app-icon"><IconValue value={app.icon} size={18} /></span><span class="app-title"><strong>{app.name}</strong><small>{app.code} · {app.description || '未填写描述'}</small></span></button><span class="app-meta"><span>{app.instances.length} 实例</span><span>{app.dependencies.length} 依赖</span><span class="status-label {app.status}">{app.status}</span><button class="icon-action" title="AI 诊断" on:click|stopPropagation={() => onOpenApplicationDiagnosis(selectedProject, app)}><Sparkles size={14} /></button><button class="icon-action" title="编辑应用" on:click|stopPropagation={() => beginEdit(app)}><Pencil size={14} /></button><button class="icon-action" title="展开应用" on:click={() => toggle(app)}><ChevronDown class={expanded.has(app.id) ? 'rotate' : ''} size={18} /></button></span></div>{#if editingAppId === app.id}<form class="inline-edit" on:submit|preventDefault={() => saveEdit(app)}><IconPicker value={editIcon} onSelect={(icon) => (editIcon = icon)} ariaLabel="选择应用图标" /><input bind:value={editName} required /><input bind:value={editDescription} placeholder="描述" /><button class="primary">保存</button><button type="button" class="secondary" on:click={() => (editingAppId = '')}>取消</button></form>{/if}{#if expanded.has(app.id)}<div class="app-detail"><div class="detail-column"><div class="detail-heading"><h3>实例</h3><button class="text-button" on:click={() => beginRelation(app, 'instance')}><Plus size={13} />添加</button></div>{#each app.instances as instance}<div class="detail-row"><span class="instance-mark">{instance.runtime_kind === 'cloud_native' ? 'K8s' : instance.runtime_kind === 'containerized' ? 'Docker' : 'VM'}</span><span><strong>{instance.name}</strong><small>{instance.target_resource_name || instance.target_resource_id}</small></span><code>{JSON.stringify(instance.selector)}</code></div>{:else}<div class="empty-state">未配置实例</div>{/each}</div><div class="detail-column"><div class="detail-heading"><h3>关联依赖</h3><button class="text-button" on:click={() => beginRelation(app, 'dependency')}><Plus size={13} />添加</button></div>{#each app.dependencies as dep}<div class="detail-row"><span class="dependency-mark">{dep.dependency_kind}</span><span><strong>{dep.target_resource_name || dep.target_resource_id}</strong><small>{dep.target_resource_kind}</small></span><code>{Object.entries(dep.binding).map(([key, value]) => `${key}: ${value}`).join(' · ') || '未设置绑定字段'}</code></div>{:else}<div class="empty-state">未配置依赖</div>{/each}</div><div class="topology"><h3><Network size={15} />关联拓扑</h3><div class="topology-flow"><div class="topology-node app-node">{app.name}</div>{#each [...app.instances, ...app.dependencies] as edge}<span class="topology-line"></span><div class="topology-node">{edge.target_resource_name || edge.target_resource_id}</div>{/each}</div></div></div>{/if}</article>{:else}<div class="panel empty-workspace"><Network size={26} /><p>还没有应用，添加或导入一个应用开始配置。</p></div>{/each}{/if}</section></div>
    {/if}
  </section>
</section>

<style>
  .project-layout{display:grid;grid-template-columns:minmax(230px,1fr) minmax(0,2fr);gap:20px;align-items:start}.project-nav{position:sticky;top:20px;padding:18px}.team-list{margin:0 -18px}.project-list{border-top:1px solid var(--theme-divider);margin-top:12px;padding-top:8px}.project-row{width:100%;display:flex;align-items:center;gap:10px;border:0;background:none;padding:10px 8px;border-radius:6px;color:var(--theme-text);text-align:left;cursor:pointer}.project-row:hover,.project-row.selected{background:var(--theme-surface-muted)}.project-row span:nth-child(2){flex:1;min-width:0}.project-row strong,.project-row small,.app-title strong,.app-title small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.project-row small,.app-title small,.resource-line small,.detail-row small{color:var(--theme-text-muted);font-size:12px;margin-top:3px}.status-dot{width:7px;height:7px;border-radius:50%;background:var(--theme-text-muted)}.status-dot.active{background:#2f9e6f}.status-dot.disabled{background:#d97757}.workspace-header{display:flex;justify-content:space-between;gap:20px;align-items:flex-start;margin-bottom:18px}.workspace-header h1{margin:3px 0 4px;font-size:28px}.workspace-header p:not(.eyebrow){margin:0;color:var(--theme-text-muted)}.header-actions{display:flex;gap:8px;flex-wrap:wrap}.header-actions button,.app-form button{display:inline-flex;align-items:center;gap:7px}.diagnose{border:1px solid color-mix(in srgb,#bd7b3c 60%,var(--theme-divider));background:transparent;color:#b66d2f;border-radius:6px;padding:8px 12px;cursor:pointer}.stat-grid{display:grid;grid-template-columns:repeat(5,1fr);gap:10px;margin-bottom:18px}.stat{background:var(--theme-surface);border:1px solid var(--theme-divider);padding:13px 15px;border-radius:6px}.stat span{display:block;color:var(--theme-text-muted);font-size:12px}.stat strong{font-size:24px;line-height:1.2}.app-form{padding:16px;margin-bottom:18px}.app-form label{display:grid;gap:6px;font-size:13px;color:var(--theme-text-muted)}.app-form input,.app-form textarea{width:100%;box-sizing:border-box}.form-actions{display:flex;justify-content:flex-end;gap:8px;margin-top:12px}.workspace-columns{display:grid;grid-template-columns:minmax(210px,1fr) minmax(0,2fr);gap:18px}.workspace-side{display:grid;gap:18px;align-content:start}.side-section{padding:16px}.side-section .panel-heading{margin-bottom:8px}.resource-line,.alert-line{display:flex;align-items:center;gap:9px;padding:10px 0;border-bottom:1px solid var(--theme-divider)}.resource-line:last-child,.alert-line:last-child{border-bottom:0}.resource-line>span:nth-child(2){flex:1;min-width:0}.resource-kind,.severity{font-size:10px;color:var(--theme-text-muted);font-family:monospace}.alert-line{font-size:13px}.severity{width:7px;height:7px;border-radius:50%;background:#d97757}.severity.warning{background:#d4a13b}.apps-section{min-width:0}.section-heading{display:flex;justify-content:space-between;align-items:end;margin:0 0 10px}.section-heading h2{margin:3px 0 0}.muted{color:var(--theme-text-muted);font-size:13px}.app-card{overflow:hidden;margin-bottom:10px}.app-card-head{width:100%;display:flex;align-items:center;gap:12px;padding:15px;border:0;background:transparent;color:var(--theme-text);text-align:left;cursor:pointer}.app-card-head:hover{background:var(--theme-surface-muted)}.app-icon{width:34px;height:34px;display:grid;place-items:center;background:var(--theme-surface-muted);border-radius:6px;color:#b66d2f}.app-title{flex:1;min-width:0}.app-meta{display:flex;align-items:center;gap:12px;color:var(--theme-text-muted);font-size:12px}.app-meta .status-label{font-size:11px}.rotate{transform:rotate(180deg)}.app-detail{display:grid;grid-template-columns:1fr 1fr;gap:16px;border-top:1px solid var(--theme-divider);padding:16px}.detail-column h3,.topology h3{font-size:13px;margin:0 0 9px;display:flex;align-items:center;gap:6px}.detail-row{display:grid;grid-template-columns:auto minmax(80px,1fr);gap:8px;align-items:center;padding:9px 0;border-bottom:1px solid var(--theme-divider)}.detail-row code{grid-column:2;color:var(--theme-text-muted);font-size:10px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.instance-mark,.dependency-mark{font-size:10px;padding:3px 5px;border:1px solid var(--theme-divider);border-radius:4px;color:var(--theme-text-muted)}.dependency-mark{color:#b66d2f}.topology{grid-column:1/-1;border-top:1px solid var(--theme-divider);padding-top:14px}.topology-flow{display:flex;align-items:center;gap:8px;overflow:auto;padding-bottom:4px}.topology-node{white-space:nowrap;padding:8px 10px;border:1px solid var(--theme-divider);border-radius:5px;background:var(--theme-surface-muted);font-size:12px}.app-node{border-color:#b66d2f;color:#b66d2f}.topology-line{width:18px;height:1px;background:var(--theme-divider);flex:none}.empty-workspace{min-height:220px;display:grid;place-content:center;justify-items:center;gap:10px;color:var(--theme-text-muted);text-align:center}.empty-workspace h2,.empty-workspace p{margin:0}.loading{padding:22px;color:var(--theme-text-muted)}
  .icon-field{display:flex;align-items:center;gap:8px;color:var(--theme-text-muted);font-size:12px}.inline-edit{display:flex;align-items:center;gap:8px;padding:0 15px 15px}.inline-edit input{min-width:0;flex:1}.inline-edit :global(.icon-picker-trigger){width:32px;height:32px}
  @media(max-width:900px){.project-layout,.workspace-columns{grid-template-columns:1fr}.project-nav{position:static}.stat-grid{grid-template-columns:repeat(3,1fr)}.workspace-header{display:block}.header-actions{margin-top:14px}.app-detail{grid-template-columns:1fr}}
  @media(max-width:540px){.stat-grid{grid-template-columns:repeat(2,1fr)}.app-meta span:not(.status-label){display:none}.workspace-header h1{font-size:24px}}
</style>
