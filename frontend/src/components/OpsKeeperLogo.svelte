<script lang="ts">
  export let size = 30;
  export let favicon = false;
  export let label = '';
  export let className = '';
</script>

<svg
  class={`opskeeper-logo ${className}`}
  width={size}
  height={size}
  viewBox="0 0 48 48"
  role={label ? 'img' : 'presentation'}
  aria-label={label || undefined}
  aria-hidden={label ? undefined : 'true'}
>
  <path
    d="M6 24c6-10 13-14 18-14s12 4 18 14c-6 10-13 14-18 14S12 34 6 24Z"
    fill="none"
    stroke="currentColor"
    stroke-width="4.2"
    stroke-linecap="round"
    stroke-linejoin="round"
  />
  <path
    d="M12 24h7l3-7 4 14 3-7h7"
    fill="none"
    stroke="currentColor"
    stroke-width="4.2"
    stroke-linecap="round"
    stroke-linejoin="round"
  />
  {#if !favicon}
    <circle cx="24" cy="24" r="3.2" fill="currentColor" />
  {/if}
</svg>

<style>
  .opskeeper-logo {
    display: block;
    flex: none;
    color: var(--theme-logo-fg, var(--theme-accent));
  }
</style>
