<script lang="ts">
  import {
    Activity,
    Database,
    FolderGit2,
    Globe2,
    Network,
    Package,
    Server,
    Waypoints
  } from 'lucide-svelte';
  import IconValue from './IconValue.svelte';
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
$: kindIcon = {
  Artifact: Package,
  Repository: FolderGit2,
  Nacos: Network,
  Nginx: Globe2,
  TongHttpServer: Globe2,
  Oracle: Database,
  OceanBase: Database,
  TongRDS: Database,
  MCPServer: Waypoints,
  Monitor: Activity,
  Loki: Activity,
  Tempo: Activity,
  Alertmanager: Activity
}[resource.kind];
</script>

{#if kindIcon}
  <svelte:component this={kindIcon} size={size} strokeWidth={1.8} aria-hidden="true" />
{:else if resource.kind === 'Host'}
  <Server size={size} strokeWidth={1.8} aria-hidden="true" />
{:else if brand}
  <BrandIcon name={brand} {size} />
{:else if fallback.startsWith('lucide:') || fallback.startsWith('iconify:') || fallback.startsWith('data:image/')}
  <IconValue value={fallback} {size} />
{:else}
  <span class="brand-icon-fallback" aria-hidden="true">{fallback}</span>
{/if}
