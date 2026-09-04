<script lang="ts">
  import { Plus } from 'lucide-svelte';
  import type { Project, Team } from '../../lib/api';

  export let teams: Team[] = [];
  export let visibleProjects: Project[] = [];
  export let selectedScopeId = '';
  export let projectTeamId = '';
  export let projectName = '';
  export let projectCode = '';
  export let projectIcon = 'FolderKanban';
  export let busy = false;
  export let teamIconComponent: (icon?: string) => any;
  export let iconGlyph: (icon?: string) => string;
  export let scopeName: (id: string) => string;
  export let onSelectTeam: (team: Team) => void;
  export let onSelectProject: (project: Project) => void;
  export let onOpenTeamDialog: () => void;
  export let onCreateProject: () => void | Promise<void>;
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
    <form class="stack-form compact-form" on:submit|preventDefault={onCreateProject}>
      <label>所属团队<select bind:value={projectTeamId} required><option value="" disabled>选择团队</option>{#each teams as team}<option value={team.id}>{team.name}</option>{/each}</select></label>
      <div class="form-row"><input bind:value={projectIcon} placeholder="图标，如 project 或 ▰" aria-label="项目图标" /><input bind:value={projectName} required placeholder="项目名称" aria-label="项目名称" /><input bind:value={projectCode} required placeholder="编码" aria-label="项目编码" /></div>
      <button class="primary" disabled={busy || !projectTeamId}>新增项目</button>
    </form>
  </section>
</section>
