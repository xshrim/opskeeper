<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let value: string | number = '';
  export let type: 'text' | 'email' | 'url' | 'number' | 'password' | 'tel' = 'text';
  export let placeholder = '';
  export let required = false;
  export let invalid = false;
  export let invalidMode: 'border' | 'bubble' = 'border';
  export let invalidMessage = '请填写此必填项';
  export let disabled = false;
  export let readonly = false;
  export let autocomplete: 'off' | 'on' | 'name' | 'email' | 'url' | 'username' | 'current-password' | 'new-password' = 'off';
  export let ariaLabel = '';
  export let className = '';
  export let width = '100%';
  export let maxWidth = '100%';
  export let min = '';
  export let max = '';
  export let maxlength: number | undefined = undefined;
  export let minlength: number | undefined = undefined;
  export let step: number | 'any' | undefined = undefined;

  const dispatch = createEventDispatcher<{ input: Event; change: Event }>();

  function handleInput(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    value = type === 'number' && input.value !== '' ? input.valueAsNumber : input.value;
    dispatch('input', event);
  }

  function handleChange(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    value = type === 'number' && input.value !== '' ? input.valueAsNumber : input.value;
    dispatch('change', event);
  }
</script>

<span class={`text-input-wrap ${className}`.trim()} class:invalid class:bubble={invalid && invalidMode === 'bubble'} style={`--text-input-width:${width};--text-input-max-width:${maxWidth}`} data-error={invalid && invalidMode === 'bubble' ? invalidMessage : undefined}>
  <input
    {value}
    {type}
    {placeholder}
    {required}
    {disabled}
    {readonly}
    {autocomplete}
    {min}
    {max}
    {maxlength}
    {minlength}
    {step}
    aria-label={ariaLabel || undefined}
    aria-invalid={invalid || undefined}
    on:input={handleInput}
    on:change={handleChange}
  />
</span>

<style>
  .text-input-wrap {
    position: relative;
    display: block;
    width: var(--text-input-width);
    max-width: var(--text-input-max-width);
    min-width: 0;
  }
  .text-input-wrap input {
    width: 100%;
    min-height: 36px;
    padding: 8px 10px;
    color: var(--theme-fg);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border-strong);
    border-radius: 4px;
    outline: 0;
    font-size: 12px;
    transition: border-color 120ms ease, box-shadow 120ms ease;
  }
  .text-input-wrap input::placeholder { color: var(--theme-fg-subtle); }
  .text-input-wrap input:hover:not(:disabled) { border-color: var(--theme-border-strong); }
  .text-input-wrap input:focus-visible {
    border-color: var(--theme-border-focus);
    box-shadow: 0 0 0 3px var(--theme-focus);
  }
  .text-input-wrap.invalid:not(.bubble) input { border-color: var(--theme-danger); box-shadow: 0 0 0 1px var(--theme-danger); }
  .text-input-wrap input:disabled,
  .text-input-wrap input:read-only { color: var(--theme-fg-muted); background: var(--theme-bg-subtle); }
  .text-input-wrap.bubble::after {
    position: absolute;
    z-index: 30;
    bottom: calc(100% + 7px);
    left: 0;
    max-width: min(280px, 90vw);
    padding: 6px 9px;
    color: var(--theme-fg-strong);
    background: var(--theme-bg-raised);
    border: 1px solid var(--theme-danger);
    border-radius: 4px;
    box-shadow: var(--theme-shadow-soft);
    content: attr(data-error);
    font-size: 11px;
    line-height: 1.35;
    white-space: normal;
  }
</style>
