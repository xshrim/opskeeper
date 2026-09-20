<script lang="ts">
  import Icon from '@iconify/svelte';
  import { icons as lucideIcons } from 'lucide-svelte';
  import { iconifyIconForValue } from '../lib/iconifyIcons';

  export let value = '';
  export let size = 18;

  let LucideIcon: any;
  $: iconifyIcon = iconifyIconForValue(value);
  $: lucideName = value.startsWith('lucide:') ? value.slice('lucide:'.length) : '';
  $: LucideIcon = lucideIcons[lucideName as keyof typeof lucideIcons] as any;
</script>

{#if value.startsWith('data:image/')}
  <img src={value} alt="" width={size} height={size} />
{:else if iconifyIcon}
  <span class="iconify-icon" aria-hidden="true">
    {#if iconifyIcon.source === 'simple-icons'}
      <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor">
        <path d={iconifyIcon.icon.path} />
      </svg>
    {:else}
      <Icon icon={iconifyIcon.icon} width={size} height={size} />
    {/if}
  </span>
{:else if LucideIcon}
  <svelte:component this={LucideIcon} {size} strokeWidth={1.8} aria-hidden="true" />
{:else}
  <span aria-hidden="true">{value ? value.slice(0, 1) : '◇'}</span>
{/if}

<style>
  img,
  .iconify-icon {
    display: inline-flex;
    line-height: 0;
  }

  img {
    object-fit: contain;
    border-radius: 3px;
  }

</style>
