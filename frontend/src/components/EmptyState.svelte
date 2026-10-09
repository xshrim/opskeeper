<script lang="ts">
  export let className = 'empty-state';
  export let title = '';
  export let description = '';
</script>

<div class={className}>
  <slot name="icon" />
  {#if title}<strong>{title}</strong>{/if}
  {#if description}<p>{description}</p>{/if}
  <slot />
</div>
