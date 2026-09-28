<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { Search } from 'lucide-svelte';

  export let value = '';
  export let placeholder = '搜索';
  export let ariaLabel = '';
  export let width = '100%';
  export let height = '35px';
  export let iconSize = 14;
  export let className = '';
  export let disabled = false;

  const dispatch = createEventDispatcher<{ value: string; input: Event }>();

  $: resolvedAriaLabel = ariaLabel || placeholder;

  function handleInput(event: Event) {
    value = (event.currentTarget as HTMLInputElement).value;
    dispatch('value', value);
    dispatch('input', event);
  }
</script>

<label
  class={`search-input ${className}`}
  style={`--search-input-width:${width};--search-input-height:${height}`}
>
  <span class="search-input-prefix"><slot name="prefix" /></span>
  <input
    type="text"
    {value}
    {placeholder}
    {disabled}
    aria-label={resolvedAriaLabel}
    on:input={handleInput}
  />
  <span class="search-input-suffix"><Search size={iconSize} aria-hidden="true" /><slot name="suffix" /></span>
</label>

<style>
  .search-input {
    display: flex;
    align-items: center;
    gap: 5px;
    width: var(--search-input-width);
    min-width: 0;
    height: var(--search-input-height);
    min-height: var(--search-input-height);
    padding: 0 8px;
    overflow: hidden;
    color: var(--theme-fg-muted);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border);
    border-radius: 5px;
    box-sizing: border-box;
  }
  .search-input:focus-within {
    border-color: var(--theme-border-focus);
    box-shadow: 0 0 0 2px var(--theme-focus);
  }
  .search-input-prefix,
  .search-input-suffix {
    display: flex;
    align-items: center;
    flex: 0 0 auto;
    gap: 4px;
    min-width: 0;
  }
  .search-input input {
    flex: 1 1 auto;
    width: 100%;
    min-width: 0;
    height: 100%;
    min-height: 0;
    padding: 0;
    color: var(--theme-fg);
    background: transparent;
    border: 0;
    border-radius: 0;
    outline: 0;
    font-size: 12px;
  }
  .search-input input:focus {
    border: 0;
    box-shadow: none;
  }
  :global(:root) .search-input.search-input input:focus-visible {
    border-color: transparent;
    background: transparent;
    box-shadow: none;
    outline: 0;
  }
  .search-input input::placeholder {
    color: var(--theme-fg-muted);
  }
  .search-input-suffix :global(svg) {
    flex: 0 0 auto;
    pointer-events: none;
  }
</style>
