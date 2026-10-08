<script lang="ts">
  import FieldLabel from './FieldLabel.svelte';
  export let label = '';
  export let required = false;
  export let invalid = false;
  export let error = '';
  export let help = '';
  export let className = '';
</script>

<label class={`form-field ${className}`.trim()} class:invalid={invalid || Boolean(error)}>
  <FieldLabel text={label} {required} invalid={invalid || Boolean(error)} className="form-field-label" />
  <slot />
  {#if error}<small class="form-field-message form-field-error">{error}</small>{:else if help}<small class="form-field-message">{help}</small>{/if}
</label>

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
</style>
