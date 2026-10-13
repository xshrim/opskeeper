<script lang="ts">
  import ReadonlyTable from '../../components/ReadonlyTable.svelte';
  import type { ApplicationReference, ConnectionCheck, MCPSnapshot, Resource } from '../../lib/api';
  import GenericResourceDetails from './GenericResourceDetails.svelte';
  import DockerResourceDetails from './DockerResourceDetails.svelte';
  import KubernetesResourceDetails from './KubernetesResourceDetails.svelte';
  import HostResourceDetails from './HostResourceDetails.svelte';
  import McpResourceDetails from './McpResourceDetails.svelte';
  import PostgreSQLResourceDetails from './PostgreSQLResourceDetails.svelte';
  import MySQLResourceDetails from './MySQLResourceDetails.svelte';
  import OracleResourceDetails from './OracleResourceDetails.svelte';
  import RabbitMQResourceDetails from './RabbitMQResourceDetails.svelte';
  import MinIOResourceDetails from './MinIOResourceDetails.svelte';
  import RedisResourceDetails from './RedisResourceDetails.svelte';
  import NacosResourceDetails from './NacosResourceDetails.svelte';
  import KafkaResourceDetails from './KafkaResourceDetails.svelte';
  import ElasticsearchResourceDetails from './ElasticsearchResourceDetails.svelte';
  import RepositoryResourceDetails from './RepositoryResourceDetails.svelte';
  import { resourceToolsFor } from './resourceTools';

  export let resource: Resource;
  export let resourceCheck: ConnectionCheck | null | undefined;
  export let operationSnapshots: Record<string, MCPSnapshot[]> = {};
  export let selectedResourceId = '';
  export let connectionCheck: ConnectionCheck | null = null;
  export let formatDate: (value: string) => string;
  export let resourceCanManage: (resource: Resource, permission: string) => boolean;
  export let mcpServerEndpointFor: (resource: Resource) => string = () => '';
  export let applicationReferences: ApplicationReference[] = [];

  $: toolRows = resource.kind === 'MCPServer'
    ? (operationSnapshots[resource.id]?.[0]?.tools ?? []).map(({ name, description }) => ({ name, description }))
    : resourceToolsFor(resource.kind);
</script>

<div class="resource-catalog-details-stack">
{#if resource.kind === 'MCPServer'}
  <McpResourceDetails {resource} snapshots={operationSnapshots[resource.id] ?? []} {formatDate} />
{:else if resource.kind === 'Docker'}
  <DockerResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
{:else if resource.kind === 'Kubernetes'}
  <KubernetesResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
  {:else if resource.kind === 'Host'}
    <HostResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
{:else if resource.kind === 'PostgreSQL'}
    <PostgreSQLResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
{:else if resource.kind === 'MySQL'}
    <MySQLResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
{:else if resource.kind === 'Oracle'}
    <OracleResourceDetails {resource} />
{:else if resource.kind === 'RabbitMQ'}
    <RabbitMQResourceDetails {resource} />
{:else if resource.kind === 'MinIO'}
    <MinIOResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
  {:else if resource.kind === 'Redis'}
    <RedisResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
{:else if resource.kind === 'Nacos'}
    <NacosResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
  {:else if resource.kind === 'Kafka'}
    <KafkaResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} />
  {:else if resource.kind === 'Elasticsearch' || resource.kind === 'ElasticSearch'}
    <ElasticsearchResourceDetails {resource} {resourceCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} />
{:else if resource.kind === 'Repository'}
    <RepositoryResourceDetails {resource} />
{:else}
  <GenericResourceDetails {resource} {selectedResourceId} {connectionCheck} mcpServerEndpoint={mcpServerEndpointFor(resource)} {formatDate} {resourceCanManage} />
{/if}
<section class="resource-application-references">
  <div class="provider-resource-models-heading"><strong>应用引用</strong><span>{applicationReferences.length} 条</span></div>
  <ReadonlyTable columns={[{ key: 'project', label: '项目', width: '28%' }, { key: 'application', label: '应用', width: '32%' }, { key: 'relation', label: '引用方式' }]} rows={applicationReferences} emptyText="当前资源未被应用引用">
    <svelte:fragment slot="cell" let:row let:column>
      {@const reference = row as ApplicationReference}
      {#if column.key === 'project'}{reference.project_name}{:else if column.key === 'application'}{reference.application_name}{:else}{reference.relation_type === 'instance' ? '实例' : '依赖'}{/if}
    </svelte:fragment>
  </ReadonlyTable>
</section>
<section class="resource-tool-table">
  <div class="provider-resource-models-heading"><strong>工具列表</strong><span>{toolRows.length} 个</span></div>
  <ReadonlyTable columns={[{ key: 'name', label: '工具名称', width: '30%' }, { key: 'description', label: '工具说明' }]} rows={toolRows} emptyText={resource.kind === 'MCPServer' ? '尚未发现工具，请先执行连接测试。' : '暂无工具'} />
</section>
</div>
