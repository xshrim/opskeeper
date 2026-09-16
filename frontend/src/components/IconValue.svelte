<script lang="ts">
  import { icons as lucideIcons } from 'lucide-svelte';
  export let value = '';
  export let size = 18;
  let Icon: any;
  $: Icon = lucideIcons[value as keyof typeof lucideIcons];
</script>
{#if value.startsWith('data:image/')}
  <img src={value} alt="" width={size} height={size} />
{:else if Icon}
  <svelte:component this={Icon} {size} strokeWidth={1.8} aria-hidden="true" />
{:else}<span aria-hidden="true">{value ? value.slice(0, 1) : '◇'}</span>{/if}
<style>img { display: block; object-fit: contain; border-radius: 3px; }</style>
