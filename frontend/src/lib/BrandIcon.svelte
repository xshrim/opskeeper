<script lang="ts">
  import IconValue from '../components/IconValue.svelte';
  import { iconifyValueForName } from './iconifyIcons';

  export let name = '';
  export let size = 16;
  export let fallback = '◇';

  $: iconValue = iconifyValueForName(name);
</script>

{#if iconValue}
  <IconValue value={iconValue} {size} />
{:else}
  <span class="brand-icon-fallback" aria-hidden="true">{fallback}</span>
{/if}
