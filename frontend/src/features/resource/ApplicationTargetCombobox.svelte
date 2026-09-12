<script context="module" lang="ts">
  export type ApplicationTargetOption = {
    value: string;
    label: string;
    description?: string;
    id?: string;
    metadata?: Record<string, unknown>;
  };
</script>

<script lang="ts">
  import { ChevronDown, RefreshCw } from 'lucide-svelte';

  export let value = '';
  export let options: ApplicationTargetOption[] = [];
  export let placeholder = '';
  export let loading = false;
  export let error = '';
  export let invalid = false;
  export let disabled = false;
  export let onInput: (value: string) => void = () => {};
  export let onFocus: () => void = () => {};
  export let onRefresh: () => void = () => {};
  export let onSelect: (option: ApplicationTargetOption) => void = () => {};

  let open = false;
  let blurTimer: ReturnType<typeof setTimeout> | undefined;

  $: normalizedValue = value.trim().toLowerCase();
  $: visibleOptions = normalizedValue
    ? options.filter((option) => `${option.label} ${option.value} ${option.description ?? ''}`.toLowerCase().includes(normalizedValue))
    : options;

  function focusInput() {
    if (blurTimer) clearTimeout(blurTimer);
    open = true;
    onFocus();
  }

  function blurInput() {
    blurTimer = setTimeout(() => (open = false), 160);
  }

  function inputChanged(event: Event) {
    const next = (event.currentTarget as HTMLInputElement).value;
    open = true;
    onInput(next);
  }

  function selectOption(option: ApplicationTargetOption) {
    if (blurTimer) clearTimeout(blurTimer);
    open = false;
    onSelect(option);
  }
</script>

<div class:invalid class="application-target-combobox">
  <div class="application-target-input-wrap">
    <input
      value={value}
      {placeholder}
      {disabled}
      autocomplete="off"
      aria-invalid={invalid}
      on:focus={focusInput}
      on:blur={blurInput}
      on:input={inputChanged}
    />
    <ChevronDown size={15} strokeWidth={1.8} class="application-target-chevron" aria-hidden="true" />
    <button
      class="application-target-refresh"
      type="button"
      title="刷新候选项"
      aria-label="刷新候选项"
      disabled={disabled || loading}
      on:mousedown|preventDefault
      on:click={onRefresh}
    >
      <RefreshCw size={14} strokeWidth={1.8} class={loading ? 'spin' : ''} aria-hidden="true" />
    </button>
  </div>

  {#if open && !disabled && (loading || visibleOptions.length || error || value.trim())}
    <div class="application-target-options" role="listbox">
      {#if loading}
        <div class="application-target-option-state">正在获取候选项…</div>
      {:else if error}
        <div class="application-target-option-state error">{error}</div>
      {:else if visibleOptions.length === 0}
        <div class="application-target-option-state">暂无匹配候选项，可继续手动填写</div>
      {:else}
        {#each visibleOptions as option}
          <button
            type="button"
            class="application-target-option"
            role="option"
            aria-selected="false"
            on:mousedown|preventDefault
            on:click={() => selectOption(option)}
          >
            <strong>{option.label}</strong>
            {#if option.description}<small>{option.description}</small>{/if}
          </button>
        {/each}
      {/if}
    </div>
  {/if}

  {#if error && !open}<small class="application-target-error">{error}</small>{/if}
</div>

<style>
  .application-target-combobox {
    position: relative;
  }

  .application-target-input-wrap {
    position: relative;
    display: flex;
    align-items: center;
  }

  .application-target-input-wrap input {
    width: 100%;
    padding-right: 4.8rem;
  }

  :global(.application-target-chevron) {
    position: absolute;
    right: 2.9rem;
    color: var(--ink-muted, #78848f);
    pointer-events: none;
  }

  .application-target-refresh {
    position: absolute;
    right: 0.45rem;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.8rem;
    height: 1.8rem;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--ink-muted, #78848f);
    cursor: pointer;
  }

  .application-target-refresh:disabled {
    cursor: wait;
    opacity: 0.55;
  }

  .application-target-refresh:hover:not(:disabled) {
    color: var(--ink, #17212b);
  }

  .application-target-options {
    position: absolute;
    z-index: 20;
    top: calc(100% + 0.25rem);
    right: 0;
    left: 0;
    max-height: 15rem;
    overflow: auto;
    border: 1px solid var(--line, #d6dde3);
    border-radius: 6px;
    background: var(--surface, #fff);
    box-shadow: 0 10px 24px rgb(30 45 60 / 14%);
  }

  .application-target-option,
  .application-target-option-state {
    display: block;
    width: 100%;
    padding: 0.55rem 0.7rem;
    text-align: left;
  }

  .application-target-option {
    border: 0;
    background: transparent;
    color: inherit;
    cursor: pointer;
  }

  .application-target-option:hover,
  .application-target-option:focus-visible {
    background: var(--surface-muted, #f2f5f7);
    outline: 0;
  }

  .application-target-option strong,
  .application-target-option small {
    display: block;
  }

  .application-target-option small,
  .application-target-option-state,
  .application-target-error {
    color: var(--ink-muted, #78848f);
    font-size: 0.78rem;
  }

  .application-target-option-state.error,
  .application-target-error {
    color: var(--danger, #b42318);
  }

  .application-target-combobox.invalid .application-target-input-wrap input {
    border-color: var(--danger, #b42318);
    box-shadow: 0 0 0 2px rgb(180 35 24 / 10%);
  }

  :global(.spin) {
    animation: application-target-spin 0.9s linear infinite;
  }

  @keyframes application-target-spin {
    to { transform: rotate(360deg); }
  }
</style>
