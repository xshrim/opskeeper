<script lang="ts">
  import type { ConnectionCheck, MCPSnapshot, Resource } from '../../lib/api';
  import GenericResourceDetails from './GenericResourceDetails.svelte';
  import DockerResourceDetails from './DockerResourceDetails.svelte';
  import KubernetesResourceDetails from './KubernetesResourceDetails.svelte';
  import HostResourceDetails from './HostResourceDetails.svelte';
  import McpResourceDetails from './McpResourceDetails.svelte';
  import ProviderResourceDetails from './ProviderResourceDetails.svelte';
  import PostgreSQLResourceDetails from './PostgreSQLResourceDetails.svelte';
  import RedisResourceDetails from './RedisResourceDetails.svelte';
  import NacosResourceDetails from './NacosResourceDetails.svelte';

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
  export let providerBindingsFor: (resource: Resource) => Array<{ tag: string }>;
  export let providerPurposeLabel: (tag: string) => string;
  export let mcpServerEndpointFor: (resource: Resource) => string = () => '';
</script>

{#if resource.kind === 'AIProvider'}
  <ProviderResourceDetails
    {resource}
    {resourceCheck}
    {formatDate}
    modelsForResource={providerModelsForResource}
    modelCapabilities={providerModelCapabilities}
    {providerTypeLabel}
    {providerBindingsFor}
    {providerPurposeLabel}
  />
{:else if resource.kind === 'MCPServer'}
  <McpResourceDetails {resource} snapshots={operationSnapshots[resource.id] ?? []} {formatDate} />
{:else if resource.kind === 'Docker'}
  <DockerResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
{:else if resource.kind === 'Kubernetes'}
  <KubernetesResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
  {:else if resource.kind === 'Host'}
    <HostResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
{:else if resource.kind === 'PostgreSQL'}
    <PostgreSQLResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
  {:else if resource.kind === 'Redis'}
    <RedisResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
  {:else if resource.kind === 'Nacos'}
    <NacosResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
{:else}
  <GenericResourceDetails {resource} {selectedResourceId} {connectionCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} {resourceCanManage} />
{/if}
