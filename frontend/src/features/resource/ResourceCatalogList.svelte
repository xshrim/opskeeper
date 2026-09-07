<script lang="ts">
  import { Pencil, PlugZap, Trash2 } from 'lucide-svelte';
  import ResourceBrandIcon from '../../components/ResourceBrandIcon.svelte';
  import { resourceHasConnector } from '../../lib/resources';
  import type { ConnectionCheck, Resource } from '../../lib/api';
  import { resourceCategoryFor, resourceEndpointFor, resourceSubtypeFor } from './resourceCatalog';
  import { dockerAccessModeLabel } from './resourceWorkflow';

  export let resources: Resource[] = [];
  export let selectedResourceId = '';
  export let resourceConnectionChecks: Record<string, ConnectionCheck | null> = {};
  export let busy = false;
  export let resourceActionBusy = false;
  export let connectionBusy = false;
  export let resourceCanManage: (resource: Resource, permission: string) => boolean;
  export let scopeType: (id: string) => string;
  export let resourceScopeLabel: (resource: Resource) => string;
  export let resourceIcon: (kind: string) => string;
  export let providerModelsForResource: (resource: Resource) => Array<Record<string, unknown>>;
  export let providerDefaultModelForResource: (resource: Resource) => Record<string, unknown> | undefined;
  export let providerModelCapabilities: (model: Record<string, unknown> | undefined) => string[];
  export let providerTypeLabel: (type: unknown) => string;
  export let providerBindingsFor: (resource: Resource) => Array<{ tag: string }>;
  export let providerPurposeLabel: (tag: string) => string;
  export let mcpServerNameFor: (resource: Resource) => string = () => '';
  export let onSelect: (resource: Resource) => void = () => {};
  export let onLoadSnapshot: (resourceId: string) => void = () => {};
  export let onToggleEnabled: (resource: Resource, enabled: boolean) => void = () => {};
  export let onTestConnection: (resource: Resource) => void = () => {};
  export let onEdit: (resource: Resource) => void = () => {};
  export let onDelete: (resource: Resource) => void = () => {};

  function endpointLabel(resource: Resource) {
    if (resource.kind === 'Docker' && String(resource.access_mode ?? resource.subtype ?? '').toLowerCase() === 'agent') {
      return mcpServerNameFor(resource) || '关联 MCPServer';
    }
    return resourceEndpointFor(resource);
  }
</script>

<div class="table-list resource-list">
  {#each resources as resource}
    {@const resourceCheck = resourceConnectionChecks[resource.id]}
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
        <span class="entity-summary"><span class="entity-icon resource-icon"><ResourceBrandIcon resource={resource} fallback={resourceIcon(resource.kind)} /></span><span><strong>{resource.name}</strong><small>{endpointLabel(resource)}</small></span></span>
        <span class="resource-cell resource-category-cell">
          {#if resource.kind === 'AIProvider'}
            {@const models = providerModelsForResource(resource)}
            {@const currentModel = providerDefaultModelForResource(resource)}
            <strong>{providerTypeLabel(resource.config?.provider_type)} · {String(currentModel?.name ?? '未设置')}</strong><small>模型 · 共 {models.length} 个</small>
          {:else if resource.kind === 'MCPServer'}
            <strong>MCPServer</strong><small>{resourceSubtypeFor(resource)}</small>
          {:else if resource.kind === 'Docker'}
            <strong>{dockerAccessModeLabel(String(resource.access_mode ?? resource.subtype ?? 'direct').toLowerCase())}</strong><small>只读工具集 · 6 项</small>
          {:else}
            <strong>{resourceCategoryFor(resource)}</strong><small>{resourceSubtypeFor(resource)}</small>
          {/if}
        </span>
        <span class="resource-cell resource-scope-cell"><strong class="scope-pill {scopeType(resource.scope_id)}">{resourceScopeLabel(resource)}</strong><small>级别</small></span>
        {#if resource.kind === 'AIProvider'}
          <span class="resource-cell provider-purpose-cell" title="角色表示特定场景的调用优先级；同级别每个角色最多绑定一个 Provider。"><span class="provider-purpose-tags">{#each providerBindingsFor(resource) as binding}<span class="resource-tag provider-purpose-tag">{providerPurposeLabel(binding.tag)}</span>{:else}<small class="resource-tags-empty">未设置</small>{/each}</span><small>角色</small></span>
        {/if}
        <span class="resource-tags" class:provider-capabilities-cell={resource.kind === 'AIProvider'} aria-label={resource.kind === 'AIProvider' ? '模型能力' : '资源标签'}>
          {#if resource.kind === 'AIProvider'}
            {#each providerModelCapabilities(providerDefaultModelForResource(resource)) as capability}<span class="resource-tag provider-capability-tag">{capability}</span>{:else}<small class="resource-tags-empty">未声明能力</small>{/each}
          {:else}
            {#each Object.entries(resource.labels ?? {}) as [key, value]}<span class="resource-tag">{key}{value ? `=${value}` : ''}</span>{:else}<small class="resource-tags-empty">未设置标签</small>{/each}
          {/if}
        </span>
        <span class="resource-cell resource-connection-cell"><span class="status-label {resourceCheck ? resourceCheck.status === 'succeeded' ? 'active' : 'unknown' : resource.status}">{resourceCheck ? `${resourceCheck.status === 'succeeded' ? '正常' : '失败'}·${resourceCheck.latency_ms}ms` : resource.status === 'active' ? '正常' : resource.status === 'disabled' ? '已停用' : '未知'}</span><small>连接状态</small></span>
        <span class="resource-row-actions" aria-label="资源操作">
          <span class="resource-enabled-control" title="是否启用"><span class="provider-toggle-control"><input type="checkbox" checked={resource.status === 'active'} disabled={busy || resourceActionBusy || !resourceCanManage(resource, 'resource:update')} aria-label={`是否启用 ${resource.name}`} on:click|stopPropagation on:change={(event) => onToggleEnabled(resource, (event.currentTarget as HTMLInputElement).checked)} /><i aria-hidden="true"></i></span></span>
          <button class="icon-button" type="button" on:click|stopPropagation={() => onTestConnection(resource)} disabled={busy || connectionBusy || !resourceHasConnector(resource)} title={resourceHasConnector(resource) ? '连接测试' : '此资源暂不支持连接测试'} aria-label="连接测试"><PlugZap size={15} aria-hidden="true" /></button>
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
