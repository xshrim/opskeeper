<script lang="ts">
  import type { ConnectionCheck, MCPSnapshot, Resource } from '../../lib/api';
  import GenericResourceDetails from './GenericResourceDetails.svelte';
  import McpResourceDetails from './McpResourceDetails.svelte';
  import ProviderResourceDetails from './ProviderResourceDetails.svelte';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let operationSnapshots: Record<string, MCPSnapshot[]> = {};
  export let selectedResourceId = '';
  export let connectionCheck: ConnectionCheck | null = null;
  export let formatDate: (value: string) => string;
  export let resourceCanManage: (resource: Resource, permission: string) => boolean;
  export let providerModelsForResource: (resource: Resource) => Array<Record<string, unknown>>;
  export let providerModelCapabilities: (model: Record<string, unknown> | undefined) => string[];
  export let providerTypeLabel: (type: unknown) => string;
</script>

{#if resource.kind === 'AIProvider'}
  <ProviderResourceDetails
    {resource}
    {resourceCheck}
    {formatDate}
    modelsForResource={providerModelsForResource}
    modelCapabilities={providerModelCapabilities}
    {providerTypeLabel}
  />
{:else if resource.kind === 'MCPServer'}
  <McpResourceDetails {resource} snapshots={operationSnapshots[resource.id] ?? []} {formatDate} />
{:else}
  <GenericResourceDetails {resource} {selectedResourceId} {connectionCheck} {formatDate} {resourceCanManage} />
{/if}
