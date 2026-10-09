<script context="module" lang="ts">
  export type TabOption = { value: string; label: string; count?: string | number };
</script>

<script lang="ts">
  export let items: TabOption[] = [];
  export let value = '';
  export let ariaLabel = '';
  export let className = '';
  export let onChange: (value: string) => void = () => {};
</script>

<div class={className} role="tablist" aria-label={ariaLabel}>
  {#each items as item (item.value)}
    <button type="button" role="tab" aria-selected={value === item.value} class:active={value === item.value} on:click={() => onChange(item.value)}>
      {item.label}{#if item.count !== undefined}<span>{item.count}</span>{/if}
    </button>
  {/each}
</div>
