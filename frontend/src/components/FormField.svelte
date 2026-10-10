<script lang="ts">
  import FieldLabel from './FieldLabel.svelte';
  export let label = '';
  export let required = false;
  export let invalid = false;
  export let error = '';
  export let help = '';
  export let className = '';
  export let asLabel = true;
</script>

<svelte:element this={asLabel ? 'label' : 'div'} class={`form-field ${className}`.trim()} class:invalid={invalid || Boolean(error)}>
  <FieldLabel text={label} {required} invalid={invalid || Boolean(error)} className="form-field-label" />
  <slot />
  {#if error}<small class="form-field-message form-field-error">{error}</small>{:else if help}<small class="form-field-message">{help}</small>{/if}
</svelte:element>

<style>
  .form-field {
    display: grid;
    gap: 6px;
    min-width: 0;
    color: var(--theme-fg-muted);
    font-size: 12px;
    font-weight: 400;
  }
  .form-field-label { display: block; }
  .form-field-message {
    color: var(--theme-fg-muted);
    font-size: 11px;
    font-weight: 400;
    line-height: 1.35;
  }
  .form-field-error { color: var(--theme-danger); }
  .form-field.invalid :global(.textarea-wrap textarea),
  .form-field.invalid :global(.password-control input) {
    border-color: var(--theme-danger);
    box-shadow: 0 0 0 1px var(--theme-danger);
  }
</style>
