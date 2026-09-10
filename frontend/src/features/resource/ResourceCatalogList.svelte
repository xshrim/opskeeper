<script lang="ts">
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
  export let providerModelsForResource: (resource: Resource) => Array<Record<string, unknown>>;
  export let providerDefaultModelForResource: (resource: Resource) => Record<string, unknown> | undefined;
  export let providerModelCapabilities: (model: Record<string, unknown> | undefined) => string[];
  export let providerBindingsFor: (resource: Resource) => Array<{ tag: string }>;
  export let providerPurposeLabel: (tag: string) => string;
  export let mcpServerEndpointFor: (resource: Resource) => string = () => '';
  export let onSelect: (resource: Resource) => void = () => {};
  export let onLoadSnapshot: (resourceId: string) => void = () => {};
  export let onToggleEnabled: (resource: Resource, enabled: boolean) => void = () => {};
  export let onTestConnection: (resource: Resource) => void = () => {};
  export let onEdit: (resource: Resource) => void = () => {};
  export let onDelete: (resource: Resource) => void = () => {};

  let now = Date.now();

  onMount(() => {
    const timer = window.setInterval(() => {
      now = Date.now();
    }, 60_000);
    return () => window.clearInterval(timer);
  });

  function endpointLabel(resource: Resource) {
    if (String(resource.subtype ?? '').toLowerCase() === 'agent') {
      return mcpServerEndpointFor(resource) || '关联 MCPServer';
    }
    return resourceEndpointFor(resource);
  }

</script>

<div class="table-list resource-list">
  {#each resources as resource}
    {@const resourceCheck = resourceConnectionChecks[resource.id]}
    {@const connectionStatus = resourceCheck
      ? resourceCheck.status === 'succeeded' ? '正常' : '异常'
      : resourceHasConnector(resource) ? '未测试' : '不支持'}
    {@const connectionDetail = connectionDetailResourceId === resource.id && resourceCheck
      ? resourceCheck.status === 'succeeded'
        ? `时延 ${resourceCheck.latency_ms}ms`
        : resourceCheck.message
      : resourceCheck
        ? relativeConnectionTime(resourceCheck.checked_at, now)
        : resourceHasConnector(resource) ? '未测试' : '不支持'}
    <details
      class:selected={selectedResourceId === resource.id}
      class:provider-resource-row={resource.kind === 'AIProvider'}
      class:mcp-resource-row={resource.kind === 'MCPServer'}
      class:docker-resource-row={resource.kind === 'Docker'}
      class="resource-catalog-row"
      on:toggle={() => {
        onSelect(resource);
        if (resource.kind === 'MCPServer') onLoadSnapshot(resource.id);
      }}
    >
      <summary>
        <span class="entity-summary"><span class="entity-icon resource-icon"><ResourceBrandIcon resource={resource} fallback={resourceIcon(resource.kind)} /></span><span><strong>{resource.name}{#if resource.kind === 'AIProvider'}{#each providerBindingsFor(resource) as binding}<em class="provider-name-role-tag provider-role-{binding.tag}">{providerPurposeLabel(binding.tag)}</em>{/each}{/if}</strong><small>{endpointLabel(resource)}</small></span></span>
        <span class="resource-cell resource-category-cell">
          {#if resource.kind === 'AIProvider'}
            {@const models = providerModelsForResource(resource)}
            {@const currentModel = providerDefaultModelForResource(resource)}
            <strong>LLM · Provider</strong><small>{String(currentModel?.name ?? '未设置')}{#if models.length > 1}<em class="provider-model-count">+{models.length - 1}</em>{/if}</small>
          {:else if resource.kind === 'MCPServer'}
            <strong>MCPServer</strong><small>{resourceSubtypeFor(resource)}</small>
          {:else}
            <strong>{resourceCategoryFor(resource)}</strong><small>{resourceSubtypeFor(resource)}</small>
          {/if}
        </span>
        <span class="resource-cell resource-scope-cell"><strong class="scope-pill {scopeType(resource.scope_id)}">{resourceScopeLabel(resource)}</strong><small>{resourcePermissionLabel(resource)}</small></span>
        <span class="resource-tags-group">
          {#if resource.kind === 'AIProvider'}
            {@const labels = Object.entries(resource.labels ?? {})}
            {@const capabilities = providerModelCapabilities(providerDefaultModelForResource(resource))}
            <span class:resource-tags-empty-state={labels.length === 0 && capabilities.length === 0} class="resource-tags" aria-label="标签和模型能力">
              {#each labels as [key, value]}<span class="resource-tag">{key}{value ? `=${value}` : ''}</span>{/each}
              {#each capabilities as capability}<span class="resource-tag provider-capability-tag">{capability}</span>{/each}
              {#if labels.length === 0 && capabilities.length === 0}<small class="resource-tags-empty">未设置标签</small>{/if}
            </span>
          {:else}
            <span class:resource-tags-empty-state={Object.keys(resource.labels ?? {}).length === 0} class="resource-tags" aria-label="资源标签">
              {#each Object.entries(resource.labels ?? {}) as [key, value]}<span class="resource-tag">{key}{value ? `=${value}` : ''}</span>{:else}<small class="resource-tags-empty">未设置标签</small>{/each}
            </span>
          {/if}
        </span>
        <span class="resource-cell resource-connection-cell">
          <button
            class="status-label {resourceCheck ? resourceCheck.status === 'succeeded' ? 'active' : 'unknown' : 'unknown'}"
            type="button"
            disabled={busy || connectionBusyResourceIds.includes(resource.id) || !resourceHasConnector(resource)}
            title={resourceHasConnector(resource) ? connectionBusyResourceIds.includes(resource.id) ? '连接测试中' : '点击测试连接' : '此资源暂不支持连接测试'}
            aria-label={resourceHasConnector(resource) ? '点击测试连接' : '此资源暂不支持连接测试'}
            on:click|stopPropagation={() => onTestConnection(resource)}
          >{connectionStatus}</button>
          <small title={connectionDetail}>{connectionDetail}</small>
        </span>
        <span class="resource-row-actions" aria-label="资源操作">
          <span class="resource-enabled-control" title="是否启用"><span class="provider-toggle-control"><input type="checkbox" checked={resource.status === 'active'} disabled={busy || resourceActionBusy || !resourceCanManage(resource, 'resource:update')} aria-label={`是否启用 ${resource.name}`} on:click|stopPropagation on:change={(event) => onToggleEnabled(resource, (event.currentTarget as HTMLInputElement).checked)} /><i aria-hidden="true"></i></span></span>
          <button class="icon-button" type="button" on:click|stopPropagation={() => onEdit(resource)} disabled={busy || !resourceCanManage(resource, 'resource:update')} title={resourceCanManage(resource, 'resource:update') ? '编辑资源' : '无编辑权限'} aria-label="编辑资源"><Pencil size={15} aria-hidden="true" /></button>
          <button class="icon-button danger-action" type="button" on:click|stopPropagation={() => onDelete(resource)} disabled={busy || !resourceCanManage(resource, 'resource:delete')} title={resourceCanManage(resource, 'resource:delete') ? '删除资源' : '无删除权限'} aria-label="删除资源"><Trash2 size={15} aria-hidden="true" /></button>
        </span>
      </summary>
      <slot name="details" {resource} {resourceCheck}></slot>
    </details>
  {:else}
    <div class="empty-state">没有匹配的资源。</div>
  {/each}
</div>
