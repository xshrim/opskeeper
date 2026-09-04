<script lang="ts">
  import { Pencil, PlugZap, Trash2 } from 'lucide-svelte';
  import BrandIcon from '../../lib/BrandIcon.svelte';
  import type { ConnectionCheck, MCPSnapshot, Resource } from '../../lib/api';

  type ProviderBinding = { tag: string };

  export let resourceCatalogItems: Resource[] = [];
  export let selectedResourceId = '';
  export let resourceConnectionChecks: Record<string, ConnectionCheck | null> = {};
  export let operationSnapshots: Record<string, MCPSnapshot[]> = {};
  export let connectionCheck: ConnectionCheck | null = null;
  export let busy = false;
  export let connectionBusy = false;

  export let loadResourceDetails: (resourceId: string) => Promise<unknown>;
  export let loadMCPSnapshots: (resourceId: string) => Promise<unknown>;
  export let testResourceRowConnection: (resource: Resource) => Promise<unknown>;
  export let toggleResourceEnabled: (resource: Resource, enabled: boolean) => Promise<unknown>;
  export let openResourceEditor: (resource: Resource) => void;
  export let deleteSelectedResource: () => Promise<unknown>;
  export let resourceCanManage: (resource: Resource, permission: string) => boolean;
  export let resourceHasConnector: (resource: Resource) => boolean;
  export let brandNameFor: (resource: Resource) => string;
  export let resourceEndpointFor: (resource: Resource) => string;
  export let resourceCategoryFor: (resource: Resource) => string;
  export let resourceSubtypeFor: (resource: Resource) => string;
  export let resourceScopeLabel: (resource: Resource) => string;
  export let scopeType: (id: string) => string;
  export let providerModelsForResource: (resource: Resource) => Array<Record<string, unknown>>;
  export let providerDefaultModelForResource: (resource: Resource) => Record<string, unknown> | undefined;
  export let providerModelCapabilities: (model: Record<string, unknown> | undefined) => string[];
  export let providerBindingsFor: (resource: Resource) => ProviderBinding[];
  export let providerPurposeLabel: (tag: string) => string;
  export let resourceLabelsText: (resource: Resource) => string;
  export let providerTypeLabel: (type: unknown) => string;
  export let formatDate: (value: string) => string;
  export let resourceIcon: (kind: string) => string;
</script>

<div class="table-list resource-list">
  {#each resourceCatalogItems as resource}
    {@const resourceCheck = resourceConnectionChecks[resource.id]}
    <details
      class:selected={selectedResourceId === resource.id}
      class:provider-resource-row={resource.kind === 'AIProvider'}
      class:mcp-resource-row={resource.kind === 'MCPServer'}
      class="resource-catalog-row"
      on:toggle={() => { void loadResourceDetails(resource.id); if (resource.kind === 'MCPServer') void loadMCPSnapshots(resource.id); }}
    >
      <summary>
        <span class="entity-summary"
          ><span class="entity-icon resource-icon"
            >{#if brandNameFor(resource)}<BrandIcon
                name={brandNameFor(resource)}
                size={18}
              />{:else}{resourceIcon(resource.kind)}{/if}</span
          ><span
            ><strong>{resource.name}</strong><small>{resourceEndpointFor(resource)}</small></span
          ></span
        >
        <span class="resource-cell resource-category-cell">
          {#if resource.kind === 'AIProvider'}
            {@const models = providerModelsForResource(resource)}
            {@const currentModel = providerDefaultModelForResource(resource)}
            <strong>{providerTypeLabel(resource.config?.provider_type)} · {String(currentModel?.name ?? '未设置')}</strong><small>模型 · 共 {models.length} 个</small>
          {:else if resource.kind === 'MCPServer'}
            <strong>MCPServer</strong><small>{resourceSubtypeFor(resource)}</small>
          {:else}
            <strong>{resourceCategoryFor(resource)}</strong><small>{resourceSubtypeFor(resource)}</small>
          {/if}
        </span>
        <span class="resource-cell resource-scope-cell"
          ><strong
            class="scope-pill {scopeType(resource.scope_id)}"
            >{resourceScopeLabel(resource)}</strong
          ><small>级别</small></span
        >
        {#if resource.kind === 'AIProvider'}
          <span
            class="resource-cell provider-purpose-cell"
            title="角色表示特定场景的调用优先级；同级别每个角色最多绑定一个 Provider。"
            ><span class="provider-purpose-tags">
              {#each providerBindingsFor(resource) as binding}
                <span class="resource-tag provider-purpose-tag"
                  >{providerPurposeLabel(binding.tag)}</span
                >
              {:else}
                <small class="resource-tags-empty">未设置</small>
              {/each}
            </span><small>角色</small></span
          >
        {/if}
        <span class="resource-tags" class:provider-capabilities-cell={resource.kind === 'AIProvider'} aria-label={resource.kind === 'AIProvider' ? '模型能力' : '资源标签'}>
          {#if resource.kind === 'AIProvider'}
            {#each providerModelCapabilities(providerDefaultModelForResource(resource)) as capability}<span class="resource-tag provider-capability-tag">{capability}</span>{:else}<small class="resource-tags-empty">未声明能力</small>{/each}
          {:else}
            {#each Object.entries(resource.labels ?? {}) as [key, value]}<span class="resource-tag">{key}{value ? `=${value}` : ''}</span>{:else}<small class="resource-tags-empty">未设置标签</small>{/each}
          {/if}
        </span>
        <span class="resource-cell resource-connection-cell"
          ><span
            class="status-label {resourceCheck
              ? resourceCheck.status === 'succeeded'
                ? 'active'
                : 'unknown'
              : resource.status}"
            >{resourceCheck
              ? `${resourceCheck.status === 'succeeded' ? '正常' : '失败'}·${resourceCheck.latency_ms}ms`
              : resource.status === 'active'
                ? '正常'
                : resource.status === 'disabled'
                  ? '已停用'
                  : '未知'}</span
          ><small>连接状态</small></span
        >
        <span class="resource-row-actions" aria-label="资源操作">
          <span class="resource-enabled-control" title="是否启用">
            <span class="provider-toggle-control"><input
              type="checkbox"
              checked={resource.status === 'active'}
              disabled={busy || !resourceCanManage(resource, 'resource:update')}
              aria-label={`是否启用 ${resource.name}`}
              on:click|stopPropagation
              on:change={(event) => void toggleResourceEnabled(resource, (event.currentTarget as HTMLInputElement).checked)}
            /><i aria-hidden="true"></i></span>
          </span>
          <button
            class="icon-button"
            type="button"
            on:click|stopPropagation={() =>
              void testResourceRowConnection(resource)}
            disabled={busy ||
              connectionBusy ||
              !resourceHasConnector(resource)}
            title={resourceHasConnector(resource)
              ? '连接测试'
              : '此资源暂不支持连接测试'}
            aria-label="连接测试"
            ><PlugZap size={15} aria-hidden="true" /></button
          >
          <button
            class="icon-button"
            type="button"
            on:click|stopPropagation={() =>
              void openResourceEditor(resource)}
            disabled={busy ||
              !resourceCanManage(resource, 'resource:update')}
            title={resourceCanManage(resource, 'resource:update')
              ? '编辑资源'
              : '无编辑权限'}
            aria-label="编辑资源"
            ><Pencil size={15} aria-hidden="true" /></button
          >
          <button
            class="icon-button danger-action"
            type="button"
            on:click|stopPropagation={() =>
              void loadResourceDetails(resource.id).then(
                deleteSelectedResource
              )}
            disabled={busy ||
              !resourceCanManage(resource, 'resource:delete')}
            title={resourceCanManage(resource, 'resource:delete')
              ? '删除资源'
              : '无删除权限'}
            aria-label="删除资源"
            ><Trash2 size={15} aria-hidden="true" /></button
          >
        </span>
      </summary>
      {#if resource.kind === 'AIProvider'}
        {@const providerConfig = resource.config ?? {}}
        {@const providerModels = providerModelsForResource(resource)}
        {@const resourceLabelText = resourceLabelsText(resource)}
        <div class="provider-resource-details">
          <div class="provider-resource-meta">
            <div>
              <span>Provider 类型</span><strong>{providerTypeLabel(providerConfig.provider_type)}</strong>
            </div>
            <div>
              <span>协议</span><strong>{String(providerConfig.protocol ?? 'chat_completions')}</strong>
            </div>
            <div class="provider-resource-address">
              <span>服务地址</span><strong>{resourceEndpointFor(resource)}</strong>
            </div>
            <div>
              <span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong>
            </div>
            <div>
              <span>请求超时</span><strong>{Number(providerConfig.timeout_seconds ?? 60)} 秒</strong>
            </div>
            <div>
              <span>最大并发</span><strong>{Number(providerConfig.max_concurrency ?? 5)}</strong>
            </div>
            <div>
              <span>限流</span><strong>{Number(providerConfig.rate_limit_per_minute ?? 0) > 0 ? `${Number(providerConfig.rate_limit_per_minute)} 次/分钟` : '不限流'}</strong>
            </div>
            <div>
              <span>默认 Model</span><strong>{String(providerConfig.default_model ?? providerModels[0]?.name ?? '未设置')}</strong>
            </div>
            <div>
              <span>API Key</span><strong>{resource.credential_id ? '已配置凭据' : '未配置凭据'}</strong>
            </div>
            <div>
              <span>Provider 状态</span><strong>{resource.status === 'active' ? '已启用' : resource.status === 'disabled' ? '已停用' : '未知'}</strong>
            </div>
            <div
              class="provider-resource-labels"
              data-tooltip={resourceLabelText || undefined}
            >
              <span>标签</span><strong>{resourceLabelText || '未设置标签'}</strong>
            </div>
            <div class="provider-resource-connection">
              <span>连接测试</span><strong>{resourceCheck ? (resourceCheck.status === 'succeeded' ? `正常 · ${resourceCheck.latency_ms} ms` : '失败') : '尚未测试'}</strong>
            </div>
          </div>
          <div class="provider-resource-models">
            <div class="provider-resource-models-heading">
              <strong>Model 列表</strong><span>{providerModels.length} 个</span>
            </div>
            {#each providerModels as model}
              {@const modelName = String(model.name ?? '未命名 Model')}
              {@const modelIsDefault = modelName === String(providerConfig.default_model ?? '').trim() || (!providerConfig.default_model && model === providerModels[0])}
              <div class="provider-resource-model-row">
                <div class="provider-resource-model-name">
                  <strong>{modelName}</strong>
                  <small>{modelIsDefault ? '默认 Model' : '备用 Model'} · {model.enabled === false ? '已停用' : '已启用'}</small>
                </div>
                <div><span>能力</span><strong>{providerModelCapabilities(model).join('、') || '未声明'}</strong></div>
                <div><span>上下文</span><strong>{Number(model.context_window_tokens ?? model.context_window ?? 128000).toLocaleString()} Token</strong></div>
                <div><span>最大输出</span><strong>{Number(model.max_output_tokens ?? 128000).toLocaleString()} Token</strong></div>
                <div><span>温度</span><strong>{Number(model.temperature ?? 0.7)}</strong></div>
                <div><span>优先级</span><strong>{Number(model.priority ?? 0)}</strong></div>
              </div>
            {:else}
              <div class="empty-state">尚未配置 Model。</div>
            {/each}
          </div>
        </div>
      {:else if resource.kind === 'MCPServer'}
        {@const mcpConfig = resource.config ?? {}}
        {@const mcpSnapshot = (operationSnapshots[resource.id] ?? [])[0]}
        <div class="provider-resource-details mcp-resource-details">
          <div class="provider-resource-meta">
            <div><span>传输方式</span><strong>{String(mcpConfig.transport ?? 'streamable_http') === 'sse' ? 'SSE' : 'Streamable HTTP'}</strong></div>
            <div class="provider-resource-address"><span>Server 地址</span><strong>{resourceEndpointFor(resource)}</strong></div>
            <div><span>工具数量</span><strong>{mcpSnapshot?.tools?.length ?? 0} 个</strong></div>
            <div><span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong></div>
            <div><span>超时时间</span><strong>{Number(mcpConfig.timeout_seconds ?? 120)} 秒</strong></div>
            <div><span>响应体大小限制</span><strong>{Math.round(Number(mcpConfig.max_response_bytes ?? 4 * 1024 * 1024) / 1024 / 1024)} MiB</strong></div>
            <div><span>自定义 Header</span><strong>{resource.credential_id ? '已配置' : '未配置'}</strong></div>
            <div><span>工具白名单</span><strong>{Array.isArray(mcpConfig.tool_allowlist) && mcpConfig.tool_allowlist.length ? `${mcpConfig.tool_allowlist.length} 条规则` : '不限制'}</strong></div>
            <div><span>访问凭据</span><strong>{resource.credential_id ? '已配置 Token / Header' : '未配置'}</strong></div>
            <div class="provider-resource-labels"><span>标签</span><strong>{resourceLabelsText(resource) || '未设置标签'}</strong></div>
            <div><span>MCPServer状态</span><strong>{resource.status === 'active' ? '已启用' : resource.status === 'disabled' ? '已停用' : '未知'}</strong></div>
            <div><span>连接测试</span><strong>{mcpSnapshot ? (mcpSnapshot.status === 'succeeded' ? `正常 · ${mcpSnapshot.latency_ms ?? 0} ms` : '失败') : '尚未测试'}</strong></div>
          </div>
          <div class="provider-resource-models mcp-resource-tools">
            <div class="provider-resource-models-heading"><strong>工具列表</strong><span>{mcpSnapshot?.tools?.length ?? 0} 个</span></div>
            {#each mcpSnapshot?.tools ?? [] as tool}
              <div class="provider-resource-model-row mcp-resource-tool-row"><div><span>工具名称</span><strong>{tool.name}</strong></div><div class="mcp-tool-description"><span>工具说明</span><strong>{tool.description || '未提供说明'}</strong></div></div>
            {:else}<div class="empty-state">尚未发现工具，请先执行连接测试。</div>{/each}
          </div>
        </div>
      {:else}
        <div class="resource-row-details">
          <div>
            <span>资源地址</span><strong>{resourceEndpointFor(resource)}</strong>
          </div>
          <div>
            <span>最新更新</span><strong>{formatDate(resource.updated_at)}</strong>
          </div>
          <div>
            <span>连接测试</span><strong>{selectedResourceId === resource.id && connectionCheck ? connectionCheck.status === 'succeeded' ? '连接正常' : '连接失败' : '展开后可测试'}</strong>
          </div>
          <div>
            <span>管理范围</span><strong>{resourceCanManage(resource, 'resource:update') ? '当前 Scope 可管理' : '继承资源，仅限查看'}</strong>
          </div>
        </div>
      {/if}
    </details>
  {:else}<div class="empty-state">没有匹配的资源。</div>{/each}
</div>
