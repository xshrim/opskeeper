<script lang="ts">
  import { Server } from 'lucide-svelte';
  import { brandNameFor } from '../lib/resources';
  import BrandIcon from '../lib/BrandIcon.svelte';

  export let resource: {
    kind: string;
    subtype?: string;
    config?: Record<string, unknown>;
  };
  export let fallback = '◇';
  export let size = 18;

  $: brand = brandNameFor(resource);
</script>

{#if resource.kind === 'AIProvider'}
  <span class="resource-brand-monogram" aria-label="LLM">AI</span>
{:else if resource.kind === 'Host'}
  <Server size={size} strokeWidth={1.8} aria-hidden="true" />
{:else if brand === 'OpenAI'}
  <span class="resource-brand-monogram" aria-label="OpenAI">AI</span>
{:else if brand}
  <BrandIcon name={brand} {size} />
{:else}
  <span class="brand-icon-fallback" aria-hidden="true">{fallback}</span>
{/if}
