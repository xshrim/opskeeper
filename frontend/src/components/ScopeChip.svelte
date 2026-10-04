<script lang="ts">
  export let label = '';
  export let count: string | number = '';
  export let active = false;
  export let ariaLabel = '';
</script>

<button class:active type="button" class="scope-chip" aria-pressed={active} aria-label={ariaLabel || label} on:click>
  <strong>{label}</strong>
  {#if count !== ''}<span>{count}</span>{/if}
</button>

<style>
  .scope-chip {
    display: inline-flex;
    align-items: center;
    justify-content: flex-start;
    flex: 0 1 auto;
    gap: 10px;
    min-width: 0;
    min-height: 29px;
    padding: 5px 8px;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-subtle);
    border: 1px solid transparent;
    border-radius: 4px;
    text-align: left;
  }
  .scope-chip:hover {
    color: var(--theme-fg);
    background: var(--theme-bg-hover);
  }
  .scope-chip.active {
    color: var(--theme-fg-strong);
    background: var(--theme-bg-selected);
    border-color: var(--theme-border-focus);
  }
  .scope-chip strong {
    overflow: hidden;
    font-size: 11px;
    font-weight: 550;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .scope-chip span {
    flex: 0 0 auto;
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .scope-chip.active span {
    color: var(--theme-fg-strong);
  }
</style>
