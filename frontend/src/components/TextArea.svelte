<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let value = '';
  export let rows = 3;
  export let placeholder = '';
  export let required = false;
  export let disabled = false;
  export let readonly = false;
  export let spellcheck: boolean | 'true' | 'false' = false;
  export let ariaLabel = '';
  export let className = '';
  export let maxlength: number | undefined = undefined;
  export let minlength: number | undefined = undefined;

  const dispatch = createEventDispatcher<{ input: Event; change: Event }>();
</script>

<span class={`textarea-wrap ${className}`.trim()}>
  <textarea
    bind:value
    {rows}
    {placeholder}
    {required}
    {disabled}
    {readonly}
    {spellcheck}
    {maxlength}
    {minlength}
    aria-label={ariaLabel || undefined}
    on:input={(event) => dispatch('input', event)}
    on:change={(event) => dispatch('change', event)}
  ></textarea>
</span>

<style>
  .textarea-wrap {
    display: block;
    width: 100%;
    min-width: 0;
  }

  textarea {
    display: block;
    width: 100%;
    min-height: 36px;
    padding: 8px 10px;
    color: var(--theme-fg);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border-strong);
    border-radius: 4px;
    outline: 0;
    font: inherit;
    font-size: 12px;
    line-height: 1.45;
    resize: vertical;
    transition: border-color 120ms ease, box-shadow 120ms ease;
  }

  textarea::placeholder { color: var(--theme-fg-subtle); }
  textarea:focus-visible {
    border-color: var(--theme-border-focus);
    box-shadow: 0 0 0 3px var(--theme-focus);
  }
  textarea:disabled,
  textarea:read-only { color: var(--theme-fg-muted); background: var(--theme-bg-subtle); }
</style>
