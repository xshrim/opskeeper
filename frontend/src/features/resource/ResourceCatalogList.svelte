<script lang="ts">
  import ExpandableTable from '../../components/ExpandableTable.svelte';
  import type { ReadonlyTableColumn } from '../../components/ReadonlyTable.svelte';
  import ConnectionTest from '../../components/ConnectionTest.svelte';
  import { Pencil, Trash2 } from 'lucide-svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import ResourceTagList from '../../components/ResourceTagList.svelte';
  import ScopeLevelBadge from '../../components/ScopeLevelBadge.svelte';
  import { labelTags } from '../../lib/labels';
  import { resourceHasConnector } from '../../lib/resources';
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceCategoryFor, resourceEndpointFor, resourceSubtypeFor } from './resourceCatalog';

  export let resources: Resource[] = [];
  export let selectedResourceId = '';
  export let selectedResourceIds: string[] = [];
  export let resourceConnectionChecks: Record<string, ConnectionCheck | null> = {};
  export let busy = false;
  export let connectionBusyResourceIds: string[] = [];
  export let resourceCanManage: (resource: Resource, permission: string) => boolean;
  export let resourcePermissionLabel: (resource: Resource) => string = () => '可查看';
  export let scopeType: (id: string) => string;
  export let resourceScopeLabel: (resource: Resource) => string;
  export let resourceIcon: (kind: string) => string;
  export let mcpServerEndpointFor: (resource: Resource) => string = () => '';
  export let onSelect: (resource: Resource) => void = () => {};
  export let onLoadSnapshot: (resourceId: string) => void = () => {};
  export let onTestConnection: (resource: Resource) => void = () => {};
  export let onEdit: (resource: Resource) => void = () => {};
  export let onDelete: (resource: Resource) => void = () => {};

  const columns: ReadonlyTableColumn[] = [
    { key: 'resource', label: '资源', width: '300px' },
    { key: 'category', label: '类型', width: '75px' },
    { key: 'scope', label: '级别', width: '58px' },
    { key: 'labels', label: '标签' },
    { key: 'application', label: '引用', width: '56px' },
    { key: 'connection', label: '连接', width: '80px', className: 'resource-centered-column' },
    { key: 'actions', label: '操作', width: '64px', className: 'resource-centered-column' }
  ];

  function endpointLabel(resource: Resource, resourceCheck: ConnectionCheck | null | undefined) {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent') return mcpServerEndpointFor(resource) || '关联 MCPServer';
    if (resource.kind === 'Kubernetes') {
      const mode = String(resource.config?.connection_mode ?? (resource.config?.server ? 'endpoint' : 'kubeconfig')).toLowerCase();
      if (mode === 'kubeconfig') return String(resourceCheck?.endpoint ?? '').trim() || '尚未获取 Kubernetes API Server';
    }
    return resourceEndpointFor(resource);
  }
</script>

  <ExpandableTable
  {columns}
  rows={resources}
  rowKey={(resource) => resource.id}
    expandable

    selectable
    selectedIds={selectedResourceIds}
    canSelectRow={(resource) => resourceCanManage(resource, 'resource:delete')}
    onSelectionChange={(ids) => (selectedResourceIds = ids)}
  className="resource-expandable-table"
  rowClass={(resource) => `resource-catalog-row ${selectedResourceId === resource.id ? 'selected' : ''} ${resource.kind === 'MCPServer' ? 'mcp-resource-row' : ''} ${resource.kind === 'Docker' ? 'docker-resource-row' : ''}`}
  emptyText="没有匹配的资源。"
  onRowToggle={(resource) => { onSelect(resource); if (resource.kind === 'MCPServer') onLoadSnapshot(resource.id); }}
>
  <svelte:fragment slot="cell" let:row let:column>
    {@const resource = row as Resource}
    {@const resourceCheck = resourceConnectionChecks[resource.id]}
    {#if column.key === 'resource'}
      <span class="entity-summary"><span class="entity-icon resource-icon"><ResourceBrandIcon resource={resource} fallback={resourceIcon(resource.kind)} /></span><span><strong>{resource.name}</strong><small>{endpointLabel(resource, resourceCheck)}</small></span></span>
    {:else if column.key === 'category'}
      <span class="resource-cell resource-category-cell">{#if resource.kind === 'MCPServer'}<strong>MCPServer</strong><small>{resourceSubtypeFor(resource)}</small>{:else}<strong>{resourceCategoryFor(resource)}</strong><small>{resourceSubtypeFor(resource)}</small>{/if}</span>
    {:else if column.key === 'scope'}
      <span class="resource-cell resource-scope-cell"><ScopeLevelBadge className="resource-scope-badge" type={scopeType(resource.scope_id) as 'platform' | 'team' | 'project'} label={resourceScopeLabel(resource)} /><small>{resourcePermissionLabel(resource)}</small></span>
    {:else if column.key === 'labels'}
      <ResourceTagList tags={labelTags(resource.labels)} />
    {:else if column.key === 'application'}
      <span class="resource-cell resource-application-cell">{resource.application_count ?? 0}</span>
    {:else if column.key === 'connection'}
      <span class="resource-cell resource-connection-cell"><ConnectionTest status={resourceCheck?.status ?? ''} latencyMs={resourceCheck?.latency_ms ?? 0} message={resourceCheck?.message ?? ''} checkedAt={resourceCheck?.checked_at ?? ''} busy={connectionBusyResourceIds.includes(resource.id)} disabled={busy || !resourceHasConnector(resource)} tooltipText={resourceHasConnector(resource) ? '' : '此资源暂不支持连接测试'} onTest={() => onTestConnection(resource)} /></span>
    {:else if column.key === 'actions'}
      <span class="resource-row-actions" aria-label="资源操作"><button class="icon-button edit-action" type="button" on:click|stopPropagation={() => onEdit(resource)} disabled={busy || !resourceCanManage(resource, 'resource:update')} data-tooltip={resourceCanManage(resource, 'resource:update') ? '编辑资源' : '无编辑权限'} aria-label="编辑资源"><Pencil size={15} aria-hidden="true" /></button><button class="icon-button danger-action delete-action" type="button" on:click|stopPropagation={() => onDelete(resource)} disabled={busy || !resourceCanManage(resource, 'resource:delete')} data-tooltip={resourceCanManage(resource, 'resource:delete') ? '删除资源' : '无删除权限'} aria-label="删除资源"><Trash2 size={15} aria-hidden="true" /></button></span>
    {/if}
  </svelte:fragment>
  <svelte:fragment slot="details" let:row>
    {@const resource = row as Resource}
    {@const resourceCheck = resourceConnectionChecks[resource.id]}
    <slot name="details" {resource} {resourceCheck}></slot>
  </svelte:fragment>
</ExpandableTable>
