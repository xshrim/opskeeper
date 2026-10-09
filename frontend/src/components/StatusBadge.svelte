<script lang="ts">
  export let label = '';
  export let tone = '';
  export let className = 'status-label';
  export let title = '';
  export let interactive = false;
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let disabled = false;
  export let ariaLabel = '';
  export let onClick: (event: MouseEvent) => void = () => {};
</script>

{#if interactive}
  <button class={`${className}${tone ? ` ${tone}` : ''}`} {title} {type} {disabled} aria-label={ariaLabel || undefined} on:click={onClick}><slot>{label}</slot></button>
{:else}
  <span class={`${className}${tone ? ` ${tone}` : ''}`} {title}><slot>{label}</slot></span>
{/if}
