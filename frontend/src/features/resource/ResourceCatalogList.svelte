<script lang="ts">
  import ExpandableTable from '../../components/ExpandableTable.svelte';
  import type { ReadonlyTableColumn } from '../../components/ReadonlyTable.svelte';
  import StatusBadge from '../../components/StatusBadge.svelte';
  import Switch from '../../components/Switch.svelte';
  import { onMount } from 'svelte';
  import { Pencil, Trash2 } from 'lucide-svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import { resourceHasConnector } from '../../lib/resources';
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { relativeConnectionTime, resourceCategoryFor, resourceEndpointFor, resourceSubtypeFor } from './resourceCatalog';

  export let resources: Resource[] = [];
  export let selectedResourceId = '';
  export let resourceConnectionChecks: Record<string, ConnectionCheck | null> = {};
  export let busy = false;
  export let resourceActionBusy = false;
  export let connectionBusyResourceIds: string[] = [];
  export let connectionDetailResourceId = '';
  export let resourceCanManage: (resource: Resource, permission: string) => boolean;
  export let resourcePermissionLabel: (resource: Resource) => string = () => '可查看';
  export let scopeType: (id: string) => string;
  export let resourceScopeLabel: (resource: Resource) => string;
  export let resourceIcon: (kind: string) => string;
  export let mcpServerEndpointFor: (resource: Resource) => string = () => '';
  export let onSelect: (resource: Resource) => void = () => {};
  export let onLoadSnapshot: (resourceId: string) => void = () => {};
  export let onToggleEnabled: (resource: Resource, enabled: boolean) => void = () => {};
  export let onTestConnection: (resource: Resource) => void = () => {};
  export let onEdit: (resource: Resource) => void = () => {};
  export let onDelete: (resource: Resource) => void = () => {};

  let now = Date.now();
  const columns: ReadonlyTableColumn[] = [
    { key: 'resource', label: '资源', width: '300px' },
    { key: 'category', label: '类型', width: '150px' },
    { key: 'scope', label: '范围', width: '58px' },
    { key: 'labels', label: '标签' },
    { key: 'connection', label: '连接', width: '80px' },
    { key: 'actions', label: '操作', width: '140px' }
  ];

  onMount(() => {
    const timer = window.setInterval(() => { now = Date.now(); }, 60_000);
    return () => window.clearInterval(timer);
  });

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
  className="resource-expandable-table"
  rowClass={(resource) => `resource-catalog-row ${selectedResourceId === resource.id ? 'selected' : ''} ${resource.kind === 'MCPServer' ? 'mcp-resource-row' : ''} ${resource.kind === 'Docker' ? 'docker-resource-row' : ''}`}
  emptyText="没有匹配的资源。"
  onRowToggle={(resource) => { onSelect(resource); if (resource.kind === 'MCPServer') onLoadSnapshot(resource.id); }}
>
  <svelte:fragment slot="cell" let:row let:column>
    {@const resource = row as Resource}
    {@const resourceCheck = resourceConnectionChecks[resource.id]}
    {@const connectionStatus = resourceCheck ? resourceCheck.status === 'succeeded' ? '正常' : '异常' : resourceHasConnector(resource) ? '未测试' : '不支持'}
    {@const connectionDetail = connectionDetailResourceId === resource.id && resourceCheck ? resourceCheck.status === 'succeeded' ? `时延 ${resourceCheck.latency_ms}ms` : resourceCheck.message : resourceCheck ? relativeConnectionTime(resourceCheck.checked_at, now) : resourceHasConnector(resource) ? '未测试' : '不支持'}
    {#if column.key === 'resource'}
      <span class="entity-summary"><span class="entity-icon resource-icon"><ResourceBrandIcon resource={resource} fallback={resourceIcon(resource.kind)} /></span><span><strong>{resource.name}</strong><small>{endpointLabel(resource, resourceCheck)}</small></span></span>
    {:else if column.key === 'category'}
      <span class="resource-cell resource-category-cell">{#if resource.kind === 'MCPServer'}<strong>MCPServer</strong><small>{resourceSubtypeFor(resource)}</small>{:else}<strong>{resourceCategoryFor(resource)}</strong><small>{resourceSubtypeFor(resource)}</small>{/if}</span>
    {:else if column.key === 'scope'}
      <span class="resource-cell resource-scope-cell"><strong class="scope-pill {scopeType(resource.scope_id)}">{resourceScopeLabel(resource)}</strong><small>{resourcePermissionLabel(resource)}</small></span>
    {:else if column.key === 'labels'}
      <span class="resource-tags-group"><span class:resource-tags-empty-state={Object.keys(resource.labels ?? {}).length === 0} class="resource-tags" aria-label="资源标签">{#each Object.entries(resource.labels ?? {}) as [key, value]}<span class="resource-tag">{key}{value ? `=${value}` : ''}</span>{:else}<small class="resource-tags-empty">未设置标签</small>{/each}</span></span>
    {:else if column.key === 'connection'}
      <span class="resource-cell resource-connection-cell"><StatusBadge className="status-label" tone={resourceCheck ? resourceCheck.status === 'succeeded' ? 'active' : 'unknown' : 'unknown'} interactive ariaLabel={resourceHasConnector(resource) ? '点击测试连接' : '此资源暂不支持连接测试'} type="button" disabled={busy || connectionBusyResourceIds.includes(resource.id) || !resourceHasConnector(resource)} title={resourceHasConnector(resource) ? connectionBusyResourceIds.includes(resource.id) ? '连接测试中' : '点击测试连接' : '此资源暂不支持连接测试'} onClick={(event) => { event.stopPropagation(); onTestConnection(resource); }}>{connectionStatus}</StatusBadge><small title={connectionDetail}>{connectionDetail}</small></span>
    {:else if column.key === 'actions'}
      <span class="resource-row-actions" aria-label="资源操作"><span class="resource-enabled-control" title="是否启用"><Switch checked={resource.status === 'active'} disabled={busy || resourceActionBusy || !resourceCanManage(resource, 'resource:update')} ariaLabel={`是否启用 ${resource.name}`} on:change={(event) => onToggleEnabled(resource, event.detail)} /></span><button class="icon-button" type="button" on:click|stopPropagation={() => onEdit(resource)} disabled={busy || !resourceCanManage(resource, 'resource:update')} title={resourceCanManage(resource, 'resource:update') ? '编辑资源' : '无编辑权限'} aria-label="编辑资源"><Pencil size={15} aria-hidden="true" /></button><button class="icon-button danger-action" type="button" on:click|stopPropagation={() => onDelete(resource)} disabled={busy || !resourceCanManage(resource, 'resource:delete')} title={resourceCanManage(resource, 'resource:delete') ? '删除资源' : '无删除权限'} aria-label="删除资源"><Trash2 size={15} aria-hidden="true" /></button></span>
    {/if}
  </svelte:fragment>
  <svelte:fragment slot="details" let:row>
    {@const resource = row as Resource}
    {@const resourceCheck = resourceConnectionChecks[resource.id]}
    <slot name="details" {resource} {resourceCheck}></slot>
  </svelte:fragment>
</ExpandableTable>
