<script lang="ts">
  import { onMount, tick } from 'svelte';
  import {
    Building2,
    Check,
    ChevronDown,
    ChevronRight,
    FolderKanban,
    Layers3
  } from 'lucide-svelte';
  import MessageBanner from '../components/MessageBanner.svelte';
  import SearchInput from '../components/SearchInput.svelte';

  type Team = { id: string; name: string };
  type Project = { id: string; name: string; team_id: string };
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
  export let projects: Project[] = [];
  export let chooseTeam: (teamID: string) => void;
  export let chooseProject: (projectID: string) => void;

  let pickerElement: HTMLDivElement;
  let scopeTriggerButton: HTMLButtonElement;
  let scopeMenuOpen = false;
  let scopeQuery = '';
  let expandedTeamId = '';
  let collapsedAutoTeamId = '';

  $: normalizedScopeQuery = scopeQuery.trim().toLocaleLowerCase();
  $: selectedTeam = teams.find((team) => team.id === selectedTeamId);
  $: selectedProject = projects.find(
    (project) => project.id === selectedProjectId
  );
  $: visibleTeams = teams.filter((team) => {
    if (!normalizedScopeQuery) return true;
    if ('全部项目'.includes(normalizedScopeQuery)) return true;
    return (
      team.name.toLocaleLowerCase().includes(normalizedScopeQuery) ||
      projects.some(
        (project) =>
          project.team_id === team.id &&
          project.name.toLocaleLowerCase().includes(normalizedScopeQuery)
      )
    );
  });

  onMount(() => {
    const closeWhenOutside = (event: PointerEvent) => {
      if (
        event.target instanceof Node &&
        !pickerElement?.contains(event.target)
      ) {
        scopeMenuOpen = false;
      }
    };
    document.addEventListener('pointerdown', closeWhenOutside);
    return () => document.removeEventListener('pointerdown', closeWhenOutside);
  });

  async function toggleScopeMenu() {
    scopeMenuOpen = !scopeMenuOpen;
    if (!scopeMenuOpen) return;
    expandedTeamId = selectedTeamId;
    collapsedAutoTeamId = '';
    scopeQuery = '';
    await tick();
    pickerElement
      ?.querySelector<HTMLInputElement>('.global-scope-search input')
      ?.focus();
  }

  async function closeScopeMenu() {
    scopeMenuOpen = false;
    scopeQuery = '';
    collapsedAutoTeamId = '';
    await tick();
    scopeTriggerButton?.focus();
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && scopeMenuOpen) closeScopeMenu();
  }

  function teamProjects(teamID: string) {
    return projects.filter((project) => project.team_id === teamID);
  }

  function visibleProjects(team: Team) {
    return teamProjects(team.id).filter(
      (project) =>
        !normalizedScopeQuery ||
        project.name.toLocaleLowerCase().includes(normalizedScopeQuery)
    );
  }

  function selectAllTeams() {
    if (!hasPlatformRole) return;
    chooseTeam('');
    closeScopeMenu();
  }

  function selectTeamProjects(teamID: string) {
    chooseTeam(teamID);
    closeScopeMenu();
  }

  function selectProject(projectID: string) {
    chooseProject(projectID);
    closeScopeMenu();
  }

  function toggleTeam(teamID: string, isExpanded: boolean) {
    if (isExpanded) {
      if (expandedTeamId === teamID) expandedTeamId = '';
      collapsedAutoTeamId = teamID;
      return;
    }
    expandedTeamId = teamID;
    collapsedAutoTeamId = '';
  }
</script>

<svelte:window on:keydown={handleKeydown} />

<header class="topbar">
  <div>
    <p class="breadcrumb">{breadcrumb}</p>
    <h1>{title}</h1>
  </div>
  <div class="topbar-actions">
    <div class="global-scope-picker" bind:this={pickerElement}>
      <button
        bind:this={scopeTriggerButton}
        class="global-scope-trigger"
        type="button"
        aria-label={`切换范围：${selectedTeam?.name ?? (hasPlatformRole ? '全部团队' : '团队')}，${selectedProject?.name ?? '全部项目'}`}
        aria-haspopup="dialog"
        aria-expanded={scopeMenuOpen}
        aria-controls="global-scope-menu"
        on:click={toggleScopeMenu}
      >
        <span class="global-scope-mark"
          ><Layers3 size={16} strokeWidth={1.8} aria-hidden="true" /></span
        >
        <span class="global-scope-path">
          <span
            >{selectedTeam?.name ??
              (hasPlatformRole ? '全部团队' : '团队')}</span
          >
          <ChevronRight size={12} aria-hidden="true" />
          <span>{selectedProject?.name ?? '全部项目'}</span>
        </span>
        <ChevronDown
          class={`global-scope-chevron${scopeMenuOpen ? ' open' : ''}`}
          size={15}
          aria-hidden="true"
        />
      </button>

      {#if scopeMenuOpen}
        <dialog
          open
          class="global-scope-menu"
          id="global-scope-menu"
          aria-label="选择团队和项目"
        >
          <SearchInput
            className="global-scope-search"
            bind:value={scopeQuery}
            on:value={() => (collapsedAutoTeamId = '')}
            width="calc(100% - 24px)"
            height="34px"
            placeholder="搜索团队或项目"
            ariaLabel="搜索团队或项目"
          />

          <div class="global-scope-tree">
            {#if hasPlatformRole && (!normalizedScopeQuery || '全部团队'.includes(normalizedScopeQuery) || '全部项目'.includes(normalizedScopeQuery))}
              <button
                type="button"
                class="global-scope-option global-scope-all"
                class:current={!selectedTeamId}
                aria-pressed={!selectedTeamId}
                on:click={selectAllTeams}
              >
                <span class="global-scope-option-icon"
                  ><Building2 size={16} aria-hidden="true" /></span
                >
                <span class="global-scope-option-copy"
                  ><strong>全部团队</strong><small>平台范围 · 全部项目</small
                  ></span
                >
                {#if !selectedTeamId}<Check
                    class="global-scope-check"
                    size={16}
                    aria-label="当前范围"
                  />{/if}
              </button>
              <div class="global-scope-divider"></div>
            {/if}

            {#if visibleTeams.length}
              <p class="global-scope-section-label">
                团队 <span
                  >{visibleTeams.length}{#if normalizedScopeQuery}
                    / {teams.length}{/if}</span
                >
              </p>
              {#each visibleTeams as team (team.id)}
                {@const teamProjectList = visibleProjects(team)}
                {@const teamNameMatches = team.name
                  .toLocaleLowerCase()
                  .includes(normalizedScopeQuery)}
                {@const isExpanded =
                  expandedTeamId === team.id ||
                  Boolean(
                    normalizedScopeQuery &&
                    visibleTeams.length === 1 &&
                    collapsedAutoTeamId !== team.id
                  )}
                <div class="global-scope-team-node">
                  <button
                    type="button"
                    class="global-scope-team-row"
                    class:expanded={isExpanded}
                    aria-expanded={isExpanded}
                    on:click={() => toggleTeam(team.id, isExpanded)}
                  >
                    <ChevronRight
                      class="global-scope-disclosure"
                      size={14}
                      aria-hidden="true"
                    />
                    <Building2
                      class="global-scope-team-icon"
                      size={15}
                      aria-hidden="true"
                    />
                    <span class="global-scope-team-name">{team.name}</span>
                    <span class="global-scope-team-count"
                      >{#if normalizedScopeQuery && !teamNameMatches && !'全部项目'.includes(normalizedScopeQuery)}{teamProjectList.length}
                        个匹配{:else}{teamProjects(team.id).length} 个项目{/if}</span
                    >
                  </button>

                  {#if isExpanded}
                    <div class="global-scope-children">
                      {#if !normalizedScopeQuery || teamNameMatches || '全部项目'.includes(normalizedScopeQuery)}
                        <button
                          type="button"
                          class="global-scope-option global-scope-project-option"
                          class:current={selectedTeamId === team.id &&
                            !selectedProjectId}
                          aria-pressed={selectedTeamId === team.id &&
                            !selectedProjectId}
                          aria-label={`切换到${team.name}下的全部项目`}
                          on:click={() => selectTeamProjects(team.id)}
                        >
                          <span class="global-scope-option-icon"
                            ><Layers3 size={14} aria-hidden="true" /></span
                          >
                          <span class="global-scope-option-copy"
                            ><strong>全部项目</strong><small>{team.name}</small
                            ></span
                          >
                          {#if selectedTeamId === team.id && !selectedProjectId}<Check
                              class="global-scope-check"
                              size={15}
                              aria-label="当前范围"
                            />{/if}
                        </button>
                      {/if}
                      {#if normalizedScopeQuery && !teamNameMatches && !'全部项目'.includes(normalizedScopeQuery) && teamProjectList.length === 0}
                        <p class="global-scope-no-projects">没有匹配的项目</p>
                      {/if}
                      {#each teamProjectList as project (project.id)}
                        <button
                          type="button"
                          class="global-scope-option global-scope-project-option"
                          class:current={selectedProjectId === project.id}
                          aria-pressed={selectedProjectId === project.id}
                          on:click={() => selectProject(project.id)}
                        >
                          <span class="global-scope-option-icon"
                            ><FolderKanban size={14} aria-hidden="true" /></span
                          >
                          <span class="global-scope-option-copy"
                            ><strong>{project.name}</strong><small
                              >{team.name}</small
                            ></span
                          >
                          {#if selectedProjectId === project.id}<Check
                              class="global-scope-check"
                              size={15}
                              aria-label="当前范围"
                            />{/if}
                        </button>
                      {/each}
                    </div>
                  {/if}
                </div>
              {/each}
            {:else if normalizedScopeQuery && !('全部团队'.includes(normalizedScopeQuery) || '全部项目'.includes(normalizedScopeQuery))}
              <div class="global-scope-empty">没有找到匹配的团队或项目</div>
            {:else}
              <div class="global-scope-empty">当前没有可用团队</div>
            {/if}
          </div>

          <footer class="global-scope-footer">
            <span>{teams.length} 个团队</span>
            <span>{projects.length} 个项目</span>
          </footer>
        </dialog>
      {/if}
    </div>
  </div>
  {#if activeMessage && !messageInChildSurface}
    <div class="topbar-message-slot">
      <MessageBanner message={activeMessage} tone={activeMessageTone} />
    </div>
  {/if}
</header>
