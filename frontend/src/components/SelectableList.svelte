<script context="module" lang="ts">
  export type SelectableOption = { value: string; label: string; description?: string; disabled?: boolean };
</script>

<script lang="ts">
  export let items: SelectableOption[] = [];
  export let value: string[] = [];
  export let className = '';
  export let ariaLabel = '';
  export let role = 'group';
  export let onChange: (value: string, checked: boolean) => void = () => {};
</script>

<div class={className} {role} aria-label={ariaLabel || undefined}>
  {#each items as item (item.value)}
    <label class:selected={value.includes(item.value)}>
      <span><strong>{item.label}</strong>{#if item.description}<small>{item.description}</small>{/if}</span>
      <input type="checkbox" checked={value.includes(item.value)} disabled={item.disabled} on:change={(event) => onChange(item.value, (event.currentTarget as HTMLInputElement).checked)} />
    </label>
  {/each}
  <slot />
</div>
