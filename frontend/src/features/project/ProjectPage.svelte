<script lang="ts">
  import { Plus } from 'lucide-svelte';
  import { api, ApiError, type Project, type Team } from '../../lib/api';
  import { iconGlyph, teamIconComponent } from '../../lib/icons';

  export let teams: Team[] = [];
  export let visibleProjects: Project[] = [];
  export let selectedScopeId = '';
  export let busy = false;
  export let scopeName: (id: string) => string;
  export let onSelectTeam: (team: Team) => void;
  export let onSelectProject: (project: Project) => void;
  export let onOpenTeamDialog: () => void;
  export let onProjectCreated: (project: Project) => void;
  export let onNotice: (message: string) => void;
  export let onError: (message: string) => void;
  let projectTeamId = '';
  let projectName = '';
  let projectCode = '';
  let projectIcon = 'FolderKanban';
  let creatingProject = false;
  $: if (!projectTeamId && teams[0]) projectTeamId = teams[0].id;
  function describeError(error: unknown, fallback: string) { if (error instanceof ApiError) { if (error.status === 403) return '当前账号没有执行此操作的权限。'; if (error.status === 401) return '会话已过期，请重新登录。'; return error.message || fallback; } return error instanceof Error ? error.message || fallback : fallback; }
  async function createProject() { if (!projectTeamId) return; creatingProject = true; onError(''); try { const created = await api.createProject(projectTeamId, { name: projectName, code: projectCode, icon: projectIcon, labels: {} }); onProjectCreated(created); projectName = ''; projectCode = ''; projectIcon = 'FolderKanban'; onNotice(`项目“${created.name}”已创建`); } catch (error) { onError(describeError(error, '创建项目失败')); } finally { creatingProject = false; } }
</script>

<section class="content-grid two-column">
  <section class="panel">
    <div class="panel-heading"><div><p class="eyebrow">STRUCTURE</p><h2>团队</h2></div><span class="count">{teams.length}</span></div>
    <div class="table-list">
      {#each teams as team}
        {@const TeamIcon = teamIconComponent(team.icon)}
        <button class:selected={selectedScopeId === team.scope.id} class="list-row" on:click={() => onSelectTeam(team)}><span class="entity-summary"><span class="entity-icon team-icon"><svelte:component this={TeamIcon} size={17} strokeWidth={1.8} /></span><span><strong>{team.name}</strong><small>{team.code} · {team.status}</small></span></span><span class="row-arrow">→</span></button>
      {:else}<div class="empty-state">暂无团队</div>{/each}
    </div>
    <div class="inline-form"><button class="primary" type="button" disabled={busy} on:click={onOpenTeamDialog}><Plus size={15} aria-hidden="true" />添加团队</button></div>
  </section>
  <section class="panel">
    <div class="panel-heading"><div><p class="eyebrow">STRUCTURE</p><h2>项目</h2></div><span class="count">{visibleProjects.length}</span></div>
    <div class="table-list">
      {#each visibleProjects as project}<button class:selected={selectedScopeId === project.scope.id} class="list-row" on:click={() => onSelectProject(project)}><span class="entity-summary"><span class="entity-icon project-icon">{iconGlyph(project.icon)}</span><span><strong>{project.name}</strong><small>{project.code} · {scopeName(project.team_id)}</small></span></span><span class="status-label {project.status}">{project.status}</span></button>{:else}<div class="empty-state">当前作用域暂无项目</div>{/each}
    </div>
    <form class="stack-form compact-form" on:submit|preventDefault={createProject}>
      <label>所属团队<select bind:value={projectTeamId} required><option value="" disabled>选择团队</option>{#each teams as team}<option value={team.id}>{team.name}</option>{/each}</select></label>
      <div class="form-row"><input bind:value={projectIcon} placeholder="图标，如 project 或 ▰" aria-label="项目图标" /><input bind:value={projectName} required placeholder="项目名称" aria-label="项目名称" /><input bind:value={projectCode} required placeholder="编码" aria-label="项目编码" /></div>
      <button class="primary" disabled={busy || creatingProject || !projectTeamId}>新增项目</button>
    </form>
  </section>
</section>
