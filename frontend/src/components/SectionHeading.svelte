<script lang="ts">
  export let title = '';
  export let eyebrow = '';
  export let description = '';
  export let className = 'panel-heading';
  export let titleTag: 'h2' | 'h3' = 'h2';
  export let titleId = '';
</script>

<div class={className}>
  <div>
    {#if eyebrow}<p class="eyebrow">{eyebrow}</p>{/if}
    <svelte:element this={titleTag} id={titleId || undefined}>{title}</svelte:element>
    {#if description}<p>{description}</p>{/if}
  </div>
  <slot name="actions" />
</div>
