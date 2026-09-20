<script lang="ts">
  import IconValue from '../../components/IconValue.svelte';
  import { providerBrandIconValue } from '../../lib/iconifyIcons';

  export let providerType = '';
  export let label = '';
  export let size = 16;

  const monograms: Record<string, string> = {
    openai_compatible: 'AI',
    openai: 'AI',
    grok: 'X',
    glm: 'GLM',
    mimo: 'Mi',
    longcat: 'LC',
    doubao: 'DB',
    siliconflow: 'SF'
  };

  $: iconValue = providerBrandIconValue(providerType);
</script>

{#if iconValue}
  <IconValue value={iconValue} {size} />
{:else}
  <span class="provider-brand-monogram" style={`--brand-size: ${size}px`} aria-hidden="true">
    {monograms[providerType] ?? label.slice(0, 2)}
  </span>
{/if}

<style>
  .provider-brand-monogram {
    display: inline-grid;
    place-items: center;
    width: var(--brand-size);
    height: var(--brand-size);
    overflow: hidden;
    color: var(--theme-accent);
    background: var(--theme-bg-accent-soft);
    border-radius: 4px;
    font-size: 7px;
    font-weight: 800;
    line-height: 1;
  }
</style>
