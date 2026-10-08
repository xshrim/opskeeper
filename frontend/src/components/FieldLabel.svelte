<script lang="ts">
  export let text = '';
  export let required = false;
  export let invalid = false;
  export let className = '';
</script>

<span class={`field-label ${className}`.trim()} class:invalid>
  {text}{#if required}<i class="required-mark" aria-hidden="true">*</i>{/if}
</span>

<style>
  .field-label {
    color: var(--theme-fg-muted);
    font-size: 12px;
    font-weight: 400;
    line-height: 1.4;
  }
  .field-label.invalid { color: var(--theme-danger); }
</style>
