<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let checked: boolean | undefined = undefined;
  export let defaultOn = false;
  export let disabled = false;
  export let ariaLabel = '开关';
  export let className = '';
  export let tooltip = '';

  const dispatch = createEventDispatcher<{ change: boolean }>();
  let internalValue = defaultOn;
  $: isChecked = checked ?? internalValue;

  function toggle() {
    if (disabled) return;
    const next = !isChecked;
    if (checked === undefined) internalValue = next;
    else checked = next;
    dispatch('change', next);
  }
</script>

<button
  class={`switch-control ${className}`.trim()}
  class:active={isChecked}
  class:disabled
  type="button"
  role="switch"
  aria-checked={isChecked}
  aria-label={ariaLabel}
  data-tooltip={tooltip || undefined}
  title={tooltip || undefined}
  {disabled}
  on:click|stopPropagation={toggle}
>
  <span aria-hidden="true"></span>
</button>

<style>
  .switch-control {
    position: relative;
    display: inline-flex;
    align-items: center;
    width: 36px;
    height: 20px;
    flex: 0 0 36px;
    padding: 0;
    background: var(--theme-border-strong);
    border: 0;
    border-radius: 999px;
    transition: background 140ms ease;
  }
  .switch-control > span {
    width: 16px;
    height: 16px;
    margin: 2px;
    background: var(--theme-bg-raised);
    border-radius: 50%;
    box-shadow: var(--theme-shadow-soft);
    transition: transform 140ms ease;
  }
  .switch-control.active { background: var(--theme-action); }
  .switch-control.active > span { transform: translateX(16px); }
  .switch-control:focus-visible { outline: 3px solid var(--theme-focus); outline-offset: 2px; }
  .switch-control:disabled { opacity: .58; cursor: not-allowed; }
</style>
