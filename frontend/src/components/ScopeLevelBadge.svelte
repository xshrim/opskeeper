<script lang="ts">
  export let type: 'platform' | 'team' | 'project' | 'scope' = 'scope';
  export let label = '';
  export let className = '';
</script>

<span
  class={`scope-level-badge scope-${type} ${className}`}
  aria-label={label || undefined}
>
  <slot>{label}</slot>
</span>

<style>
  .scope-level-badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    box-sizing: border-box;
    min-width: 0;
    min-height: 20px;
    padding: 2px 6px;
    border: 1px solid var(--theme-scope-platform-border);
    border-radius: 4px;
    color: var(--theme-scope-platform-fg);
    background: var(--theme-scope-platform-bg);
    font-size: 10px;
    font-weight: 650;
    line-height: 1.2;
    white-space: nowrap;
  }

  .scope-level-badge.scope-team {
    color: var(--theme-scope-team-fg);
    background: var(--theme-scope-team-bg);
    border-color: var(--theme-scope-team-border);
  }

  .scope-level-badge.scope-project {
    color: var(--theme-scope-project-fg);
    background: var(--theme-scope-project-bg);
    border-color: var(--theme-scope-project-border);
  }
</style>
