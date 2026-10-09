<script context="module" lang="ts">
  export type SearchFieldOption = {
    key: string;
    label: string;
    placeholder?: string;
  };
</script>

<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { ListFilter, Search, X } from 'lucide-svelte';

  export let value = '';
  export let placeholder = '搜索';
  export let ariaLabel = '';
  export let width = '100%';
  export let height = '35px';
  export let iconSize = 14;
  export let className = '';
  export let disabled = false;
  export let fieldSearchable = false;
  export let fieldOptions: SearchFieldOption[] = [];
  export let fieldValues: Record<string, string> = {};

  const dispatch = createEventDispatcher<{
    value: string;
    input: Event;
    clear: void;
    fields: Record<string, string>;
    search: string;
  }>();
  let inputElement: HTMLInputElement;
  let fieldPanel: HTMLDivElement;
  let fieldOpen = false;

  $: resolvedAriaLabel = ariaLabel || placeholder;

  function handleInput(event: Event) {
    value = (event.currentTarget as HTMLInputElement).value;
    fieldValues = parseFields(value);
    dispatch('value', value);
    dispatch('fields', fieldValues);
    dispatch('input', event);
  }

  function clearValue() {
    value = '';
    fieldValues = {};
    dispatch('value', value);
    dispatch('fields', fieldValues);
    dispatch('clear');
    inputElement?.focus();
  }

  function parseFields(query: string) {
    const configured = new Set(fieldOptions.map((field) => field.key));
    const parsed: Record<string, string> = {};
    const pattern = /(?:^|\s)([\w.-]+):("[^"]*"|'[^']*'|\S+)/g;
    let match: RegExpExecArray | null;
    while ((match = pattern.exec(query))) {
      const key = match[1];
      if (!configured.has(key)) continue;
      parsed[key] = match[2].replace(/^("|')|("|')$/g, '');
    }
    return parsed;
  }

  function selectField(key: string) {
    const prefix = value.trim();
    value = `${prefix}${prefix ? ' ' : ''}${key}:`;
    dispatch('value', value);
    inputElement?.focus();
    fieldOpen = false;
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key !== 'Enter') return;
    dispatch('search', value.trim());
    fieldOpen = false;
  }

  function openFieldSearch() {
    fieldOpen = true;
  }

  function closeFields(event: MouseEvent) {
    if (fieldPanel && !fieldPanel.parentElement?.contains(event.target as Node)) fieldOpen = false;
  }
</script>

<svelte:window on:click={closeFields} />

<div class="search-input-shell" style={`--search-input-width:${width};--search-input-height:${height}`}>
  <label class={`search-input ${className}`}>
    <span class="search-input-prefix"><Search size={iconSize} aria-hidden="true" /><slot name="prefix" /></span>
    <input
      bind:this={inputElement}
      type="text"
      {value}
      {placeholder}
      {disabled}
      aria-label={resolvedAriaLabel}
      on:input={handleInput}
      on:focus={openFieldSearch}
      on:keydown={handleKeydown}
    />
    <span class="search-input-suffix">
      {#if value}
        <button class="search-input-clear" type="button" aria-label="清空搜索" title="清空搜索" data-tooltip="清空搜索" on:click|preventDefault|stopPropagation={clearValue}><X size={iconSize} aria-hidden="true" /></button>
      {/if}
      {#if fieldSearchable && fieldOptions.length > 0}
        <button class:active={fieldOpen} class="search-input-fields" type="button" aria-label="字段筛选" aria-expanded={fieldOpen} title="字段筛选" on:click|preventDefault|stopPropagation={() => (fieldOpen = !fieldOpen)}><ListFilter size={iconSize} aria-hidden="true" /></button>
      {/if}
      <slot name="suffix" />
    </span>
  </label>
  {#if fieldSearchable && fieldOpen && fieldOptions.length > 0}
    <div class="search-input-field-panel" bind:this={fieldPanel} role="listbox" aria-label="字段搜索">
      <div class="search-input-field-help">选择字段后直接输入值，回车搜索</div>
      {#each fieldOptions as field (field.key)}
        <button class="search-input-field-option" type="button" role="option" aria-selected="false" on:click={() => selectField(field.key)}><span>{field.label}</span><code>{field.key}:</code></button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .search-input-shell {
    position: relative;
    width: var(--search-input-width);
    min-width: 0;
  }
  .search-input {
    display: flex;
    align-items: center;
    gap: 5px;
    width: 100%;
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
  .search-input-prefix :global(svg),
  .search-input-suffix :global(svg) {
    flex: 0 0 auto;
    pointer-events: none;
  }
  .search-input-clear {
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    flex: 0 0 24px;
    padding: 0;
    color: var(--theme-fg-subtle);
    background: transparent;
    border: 0;
    border-radius: 4px;
    cursor: pointer;
  }
  .search-input-clear:hover {
    color: var(--theme-fg);
    background: var(--theme-bg-hover);
  }
  .search-input-fields {
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    flex: 0 0 24px;
    padding: 0;
    color: var(--theme-fg-subtle);
    background: transparent;
    border: 0;
    border-radius: 4px;
    cursor: pointer;
  }
  .search-input-fields:hover,
  .search-input-fields.active {
    color: var(--theme-accent);
    background: var(--theme-bg-hover);
  }
  .search-input-field-panel {
    position: absolute;
    z-index: 50;
    top: calc(100% + 6px);
    right: 0;
    width: min(300px, 100vw - 24px);
    padding: 9px;
    color: var(--theme-fg);
    background: var(--theme-bg-raised);
    border: 1px solid var(--theme-border-strong);
    border-radius: 5px;
    box-shadow: var(--theme-shadow);
  }
  .search-input-field-help {
    margin-bottom: 7px;
    color: var(--theme-fg-muted);
    font-size: 10px;
  }
  .search-input-field-option {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    width: 100%;
    min-height: 30px;
    padding: 5px 7px;
    color: var(--theme-fg);
    background: transparent;
    border: 0;
    border-radius: 4px;
    text-align: left;
    cursor: pointer;
    font-size: 11px;
  }
  .search-input-field-option:hover,
  .search-input-field-option:focus-visible {
    background: var(--theme-bg-selected);
    outline: 0;
  }
  .search-input-field-option code {
    color: var(--theme-fg-subtle);
    font-size: 10px;
  }
  .search-input-clear:focus-visible {
    color: var(--theme-accent);
    outline: 2px solid var(--theme-focus);
    outline-offset: 1px;
  }
</style>
