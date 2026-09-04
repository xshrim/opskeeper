<script lang="ts">
  import MessageBanner from '../components/MessageBanner.svelte';

  type Team = { id: string; name: string };
  type Project = { id: string; name: string };
  type Tone = 'success' | 'error' | 'warning' | 'info';

  export let breadcrumb = '';
  export let title = '';
  export let activeMessage = '';
  export let activeMessageTone: Tone = 'info';
  export let messageInChildSurface = false;
  export let hasPlatformRole = false;
  export let selectedTeamId = '';
  export let selectedProjectId = '';
  export let teams: Team[] = [];
  export let workspaceProjects: Project[] = [];
  export let chooseTeam: (teamID: string) => void;
  export let chooseProject: (projectID: string) => void;
</script>

<header class="topbar">
  <div>
    <p class="breadcrumb">{breadcrumb}</p>
    <h1>{title}</h1>
  </div>
  <div class="topbar-actions">
    <div class="workspace-switcher topbar-workspace-switcher">
      <label class="workspace-team workspace-team-select"><select aria-label="切换团队" value={selectedTeamId} on:change={(event) => chooseTeam((event.currentTarget as HTMLSelectElement).value)}>
        {#if hasPlatformRole}<option value="">全部团队</option>{/if}
        {#each teams as team}<option value={team.id}>{team.name}</option>{/each}
      </select></label>
      <label class="workspace-project"><select aria-label="切换项目" value={selectedProjectId} disabled={!workspaceProjects.length} on:change={(event) => chooseProject((event.currentTarget as HTMLSelectElement).value)}>
        <option value="">全部项目</option>
        {#each workspaceProjects as project}<option value={project.id}>{project.name}</option>{/each}
      </select></label>
    </div>
  </div>
  {#if activeMessage && !messageInChildSurface}
    <div class="topbar-message-slot"><MessageBanner message={activeMessage} tone={activeMessageTone} /></div>
  {/if}
</header>
