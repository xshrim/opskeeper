<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { Eye, EyeOff } from 'lucide-svelte';

  export let value = '';
  export let placeholder = '';
  export let autocomplete: 'off' | 'current-password' | 'new-password' = 'off';
  export let required = false;
  export let minlength: number | undefined = undefined;
  export let ariaLabel = '';
  export let secretLabel = '密码';
  export let disabled = false;

  let visible = false;
  const dispatch = createEventDispatcher<{ value: string; input: Event }>();

  function handleInput(event: Event) {
    value = (event.currentTarget as HTMLInputElement).value;
    dispatch('value', value);
    dispatch('input', event);
  }
</script>

<span class="password-control">
  <input
    {value}
    {placeholder}
    {autocomplete}
    {required}
    {minlength}
    {disabled}
    aria-label={ariaLabel || undefined}
    type={visible ? 'text' : 'password'}
    on:input={handleInput}
  />
  <button
    class="password-toggle"
    type="button"
    aria-label={visible ? `隐藏${secretLabel}` : `显示${secretLabel}`}
    aria-pressed={visible}
    data-tooltip={visible ? `隐藏${secretLabel}` : `显示${secretLabel}`}
    on:click={() => (visible = !visible)}
  >
    {#if visible}<EyeOff size={16} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={16} strokeWidth={1.8} aria-hidden="true" />{/if}
  </button>
</span>

<style>
  .password-control {
    position: relative;
    display: block;
    width: 100%;
  }
  .password-control input {
    width: 100%;
    padding-right: 42px;
  }
  .password-toggle {
    position: absolute;
    z-index: 1;
    top: 50%;
    right: 3px;
    display: grid;
    place-items: center;
    width: 32px;
    height: 30px;
    padding: 0;
    color: var(--theme-fg-muted);
    background: transparent;
    border: 0;
    border-radius: 4px;
    transform: translateY(-50%);
  }
  .password-toggle:hover,
  .password-toggle:focus-visible {
    color: var(--theme-accent);
    background: var(--theme-bg-hover);
    outline: none;
  }
</style>
