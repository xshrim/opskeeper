<script lang="ts">
  import { X } from 'lucide-svelte';
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
  export let tagged = false;
  export let tagSeparators: string[] = [',', ' '];

  const dispatch = createEventDispatcher<{ input: Event; change: Event }>();
  let tags: string[] = [];
  let tagDraft = '';
  let lastPublishedTags = '';

  $: if (tagged && typeof value === 'string' && value !== lastPublishedTags) {
    tags = splitTags(value);
    lastPublishedTags = serializeTags(tags);
  }

  function splitTags(input: string) {
    const separators = tagSeparators.filter(Boolean).map(escapeRegExp);
    if (separators.length === 0) return input.trim() ? [input.trim()] : [];
    return input
      .split(new RegExp(`(?:${separators.join('|')})+`))
      .map((item) => item.trim())
      .map((item) => (item.endsWith('=') ? item.slice(0, -1).trim() : item))
      .filter(Boolean);
  }

  function escapeRegExp(input: string) {
    return input.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  function serializeTags(items: string[]) {
    return items.join(', ');
  }

  function publishTags(event: Event) {
    const next = serializeTags(tags);
    lastPublishedTags = next;
    value = next;
    dispatch('input', event);
    dispatch('change', event);
  }

  function commitTags(event: Event) {
    const additions = splitTags(tagDraft);
    if (additions.length) {
      tags = [...tags, ...additions];
      tagDraft = '';
      publishTags(event);
      return;
    }
    dispatch('change', event);
  }

  function removeTag(index: number) {
    if (disabled || readonly) return;
    tags = tags.filter((_, itemIndex) => itemIndex !== index);
    publishTags(new Event('input'));
  }

  function tagKind(tag: string) {
    if (tag.includes('=') || tag.includes(':')) return 'assignment';
    return 'key';
  }

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
  {#if tagged}
    <span class="text-input-tag-control" class:disabled class:readonly>
      {#each tags as tag, index}
        <span class={`text-input-tag ${tagKind(tag)}`}>
          <span>{tag}</span>
          <button type="button" aria-label={`移除 ${tag}`} disabled={disabled || readonly} on:click|stopPropagation={() => removeTag(index)}><X size={11} /></button>
        </span>
      {/each}
      <input
        bind:value={tagDraft}
        type="text"
        {placeholder}
        required={required && tags.length === 0}
        {disabled}
        {readonly}
        {autocomplete}
        {maxlength}
        aria-label={ariaLabel || undefined}
        aria-invalid={invalid || undefined}
        on:input={(event) => dispatch('input', event)}
        on:keydown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault();
            commitTags(event);
          }
        }}
        on:blur={commitTags}
      />
    </span>
  {:else}
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
  {/if}
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
  .text-input-tag-control {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
    width: 100%;
    min-height: 36px;
    padding: 4px 10px;
    color: var(--theme-fg);
    background: var(--theme-bg-input);
    border: 1px solid var(--theme-border-strong);
    border-radius: 4px;
    box-sizing: border-box;
    transition: border-color 120ms ease, box-shadow 120ms ease;
  }
  .text-input-tag-control:focus-within { border-color: var(--theme-border-focus); box-shadow: 0 0 0 3px var(--theme-focus); }
  .text-input-tag-control.disabled,
  .text-input-tag-control.readonly { color: var(--theme-fg-muted); background: var(--theme-bg-subtle); }
  .text-input-wrap.invalid:not(.bubble) .text-input-tag-control { border-color: var(--theme-danger); box-shadow: 0 0 0 1px var(--theme-danger); }
  .text-input-tag-control input {
    flex: 1 1 100px;
    width: auto;
    min-width: 80px;
    min-height: 24px;
    padding: 0;
    background: transparent;
    border: 0;
    box-shadow: none;
  }
  .text-input-tag-control input:focus-visible { border: 0; box-shadow: none; }
  .text-input-tag {
    position: relative;
    display: inline-flex;
    align-items: center;
    max-width: 100%;
    min-height: 22px;
    padding: 0 8px;
    border-radius: 3px;
    font-size: 11px;
    line-height: 1;
  }
  .text-input-tag > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .text-input-tag.key { color: var(--theme-fg-strong); background: var(--theme-tag-input-key-bg); }
  .text-input-tag.assignment { color: var(--theme-fg-strong); background: var(--theme-tag-input-value-bg); }
  .text-input-tag button {
    position: absolute;
    top: -6px;
    right: -6px;
    z-index: 1;
    display: grid;
    place-items: center;
    width: 13px;
    height: 13px;
    padding: 0;
    color: var(--theme-fg-muted);
    background: transparent;
    border: 0;
    border-radius: 50%;
    opacity: 0;
    pointer-events: none;
    transition: opacity 120ms ease, color 120ms ease;
  }
  .text-input-tag:hover button, .text-input-tag:focus-within button { opacity: 1; pointer-events: auto; }
  .text-input-tag button:hover, .text-input-tag button:focus-visible { color: var(--theme-action); background: transparent; outline: 0; }
  .text-input-wrap.bubble::after {
    position: absolute;
    z-index: 30;
    bottom: calc(100% + 7px);
    left: 0;
    max-width: min(280px, 90vw);
    padding: 6px 9px;
    color: var(--theme-danger);
    background: var(--theme-bg-danger-soft);
    border-radius: 4px;
    box-shadow: var(--theme-shadow-soft);
    content: attr(data-error);
    font-size: 11px;
    line-height: 1.35;
    white-space: normal;
  }
  .text-input-wrap.bubble::before {
    position: absolute;
    z-index: 30;
    bottom: calc(100% + 1px);
    left: 12px;
    width: 0;
    height: 0;
    border: 6px solid transparent;
    border-top-color: var(--theme-bg-danger-soft);
    content: '';
  }
</style>
