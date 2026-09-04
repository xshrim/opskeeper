<script lang="ts">
  import { Eye, EyeOff, Pencil, PlugZap, Plus, Trash2 } from 'lucide-svelte';
  import BrandIcon from '../../lib/BrandIcon.svelte';
  import MessageBanner from '../../components/MessageBanner.svelte';
  import type {
    ConnectionCheck,
    ConnectorCapability,
    MCPSnapshot,
    Relation,
    Resource,
    ResourceSchema,
    TopologyNode
  } from '../../lib/api';

  type ProviderModel = {
    name: string;
    contextWindowTokens: number;
    maxOutputTokens: number;
    temperature: number;
    temperatureMutable: boolean;
    capabilities: string[];
    enabled: boolean;
    priority: number;
  };

  export let resourceCategoryOptions: Record<string, string[]> = {};
  export let visibleResources: Resource[] = [];
  export let resourceCategory = '全部';
  export let resourceSubtype = '全部';
  export let expandedResourceCategory = '';
  export let resourceCategoryIcon: (category: string) => string;
  export let resourceCategoryFor: (resource: any) => string;
  export let resourceSubtypeFor: (resource: any) => string;
  export let selectResourceCategory: (
    category: string,
    subtype?: string
  ) => void;
  export let resourceSearch = '';
  export let resourceStatusFilter = 'all';
  export let resourceLevelFilter = 'all';
  export let resourceAddMenuOpen = false;
  export let toggleResourceAddMenu: () => void;
  export let resourceCatalogItems: Resource[] = [];
  export let selectedResourceId = '';
  export let resourceConnectionChecks: Record<string, ConnectionCheck | null> =
    {};
  export let operationSnapshots: Record<string, MCPSnapshot[]> = {};
  export let connectionCheck: ConnectionCheck | null = null;
  export let busy = false;
  export let connectionBusy = false;
  export let loadResourceDetails: (id: string) => Promise<unknown>;
  export let loadMCPSnapshots: (id: string) => Promise<unknown>;
  export let testResourceRowConnection: (
    resource: Resource
  ) => Promise<unknown>;
  export let toggleResourceEnabled: (
    resource: Resource,
    enabled: boolean
  ) => Promise<unknown>;
  export let openResourceEditor: (resource: Resource) => void;
  export let deleteSelectedResource: () => Promise<unknown>;
  export let resourceCanManage: (
    resource: Resource,
    permission: string
  ) => boolean;
  export let resourceHasConnector: (resource: Resource) => boolean;
  export let brandNameFor: (resource: Resource) => string;
  export let resourceEndpointFor: (resource: Resource) => string;
  export let resourceScopeLabel: (resource: Resource) => string;
  export let scopeType: (id: string) => string;
  export let providerModelsForResource: (
    resource: Resource
  ) => Array<Record<string, unknown>>;
  export let providerDefaultModelForResource: (
    resource: Resource
  ) => Record<string, unknown> | undefined;
  export let providerModelCapabilities: (
    model: Record<string, unknown> | undefined
  ) => string[];
  export let providerBindingsFor: (
    resource: Resource
  ) => Array<{ tag: string }>;
  export let providerPurposeLabel: (tag: string) => string;
  export let resourceLabelsText: (resource: Resource) => string;
  export let providerTypeLabel: (type: unknown) => string;
  export let formatDate: (value: string) => string;
  export let resourceIcon: (kind: string) => string;
  export let selectedResource: Resource | null = null;
  export let selectedResourceCanDelete = false;
  export let selectedResourceHasConnector = false;
  export let selectedScopeId = '';
  export let testSelectedResourceConnection: () => Promise<unknown>;
  export let updateSelectedResource: () => Promise<unknown>;
  export let selectedResourceCanUpdate = false;
  export let scopeName: (id: string) => string;
  export let capabilityName: (capability: ConnectorCapability) => string;
  export let relations: Relation[] = [];
  export let topology: TopologyNode[] = [];
  export let relationTarget = '';
  export let relationType = 'depends_on';
  export let createRelation: () => Promise<unknown>;
  export let deleteRelation: (relation: Relation) => void;
  export let resources: Resource[] = [];
  export let selectedSchema: ResourceSchema | null = null;
  export let resourceSchemaFieldRequired: (key: string) => boolean;
  export let resourceSubtypeOptionsFor: (resource: Resource) => string[];
  export let editResourceName = '';
  export let editResourceStatus = 'active';
  export let editResourceLabels = '';
  export let editResourceConfig = '{}';
  export let editResourceSensitiveValues: Record<string, string> = {};
  export let resourceEditorOpen = false;
  export let resourceKind = '';
  export let resourceAddStep = 1;
  export let resourceAddCategory = '';
  export let resourceAddSubtype = '';
  export let resourceName = '';
  export let resourceStatus = 'active';
  export let resourceLabels = '';
  export let resourceConfig = '{}';
  export let resourceConfigValues: Record<string, string> = {};
  export let resourceSensitiveValues: Record<string, string> = {};
  export let editingProviderResourceId = '';
  export let editingResourceId = '';
  export let providerType = 'openai_compatible';
  export let providerProtocol = 'chat_completions';
  export let providerBaseURL = '';
  export let providerAPIKey = '';
  export let providerAPIKeyVisible = false;
  export let providerAPIKeyLoading = false;
  export let providerTimeoutSeconds = 60;
  export let providerMaxConcurrency = 5;
  export let providerRateLimitPerMinute = 0;
  export let providerModels: ProviderModel[] = [];
  export let providerModelDraft: ProviderModel;
  export let editingProviderModelName = '';
  export let providerDefaultModel = '';
  export let providerPurposeTags: string[] = [];
  export let mcpTransport = 'streamable_http';
  export let mcpURL = '';
  export let mcpToken = '';
  export let mcpRequestHeaders = '';
  export let mcpToolAllowlist = '';
  export let mcpTimeoutSeconds = 120;
  export let mcpMaxResponseBytes = 4 * 1024 * 1024;
  export let mcpDraftTest: any = null;
  export let mcpDraftTestBusy = false;
  export let mcpConfigurationAttempted = false;
  export let providerConfigurationAttempted = false;
  export let providerModelConfigurationAttempted = false;
  export let providerDraftTest: any = null;
  export let providerDraftTestBusy = false;
  export let providerDraftTestPassedState = false;
  export let resourceTypeSelectionAttempted = false;
  export let resourceBasicConfigurationAttempted = false;
  export let activeMessage = '';
  export let activeMessageTone: 'success' | 'error' = 'success';
  export let providerTypeOptions: Array<{ value: string; label: string }> = [];
  export let providerPurposeOptions: Array<{ value: string; label: string }> =
    [];
  export let providerCapabilityOptions: Array<{
    value: string;
    label: string;
  }> = [];
  export let createSchema: ResourceSchema | null = null;
  export let resourceAddSubtypeOptions: string[] = [];
  export let resourceBasicConfigurationComplete: () => boolean;
  export let mcpConfigurationValid: () => boolean;
  export let resourceAddStepTitle: (step: number, kind: string) => string;
  export let resourceAddStepValidationMessage: () => string;
  export let continueResourceAdd: () => void;
  export let continueProviderAdd: () => void;
  export let mcpHeaderCount: () => number;
  export let testMCPDraftConnection: () => void;
  export let providerBaseURLValid: () => boolean;
  export let selectProviderType: (type: string) => void;
  export let toggleProviderPurpose: (purpose: string) => void;
  export let toggleProviderModelCapability: (capability: string) => void;
  export let addProviderModel: () => void;
  export let setProviderDefaultModel: (name: string) => void;
  export let setProviderModelEnabled: (name: string, enabled: boolean) => void;
  export let editProviderModel: (model: any) => void;
  export let removeProviderModel: (name: string) => void;
  export let testProviderDraftConnection: () => void;
  export let submitProviderCreate: () => void;
  export let createResource: () => void;
  export let updateMCPFromWorkflow: () => void;
  export let parseLabels: (value: string) => Record<string, string>;
  export let resourceSchemaForSelection: (
    category: string,
    subtype: string
  ) => ResourceSchema | null;
  export let activeScopeSummary: () => string;
  export let selectResourceAddCategory: (category: string) => void;
</script>

<section class="resources-layout">
  <section class="panel resource-list-panel">
    <div class="resource-catalog-rail">
      <button
        class:active={resourceCategory === '全部'}
        class="catalog-root"
        type="button"
        on:click={() => selectResourceCategory('全部')}
        ><span class="catalog-icon">{resourceCategoryIcon('全部')}</span><span
          class="catalog-label">全部资源</span
        ><span>{visibleResources.length}</span></button
      >
      {#each Object.entries(resourceCategoryOptions).filter(([name]) => name !== '全部') as [category, subtypes]}
        <div class="catalog-category">
          <button
            class:active={resourceCategory === category &&
              resourceSubtype === '全部'}
            class="catalog-category-button"
            type="button"
            on:click={() => selectResourceCategory(category)}
            ><span class="catalog-name"
              ><span class="catalog-icon">{resourceCategoryIcon(category)}</span
              >{category}</span
            ><span
              >{visibleResources.filter(
                (item) => resourceCategoryFor(item) === category
              ).length}</span
            ></button
          >
          {#if expandedResourceCategory === category}
            <div class="catalog-subtypes">
              {#each subtypes as subtype}
                <button
                  class:active={resourceCategory === category &&
                    resourceSubtype === subtype}
                  type="button"
                  on:click={() => selectResourceCategory(category, subtype)}
                  ><span class="catalog-name"
                    ><span class="catalog-icon subtype-icon"
                      >{resourceCategoryIcon(subtype)}</span
                    >{subtype}</span
                  ><span
                    >{visibleResources.filter(
                      (item) =>
                        resourceCategoryFor(item) === category &&
                        resourceSubtypeFor(item) === subtype
                    ).length}</span
                  ></button
                >
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  </section>
  <section class="resource-workspace">
    <section class="panel resource-catalog-panel">
      <div class="resource-catalog-toolbar">
        <div class="resource-catalog-title">
          <h2>
            {resourceCategory === '全部'
              ? '全部'
              : resourceSubtype === '全部'
                ? resourceCategory
                : `${resourceCategory} · ${resourceSubtype}`}
          </h2>
          <small>{resourceCatalogItems.length} 个可见资源</small>
        </div>
        <div class="resource-catalog-filters">
          <input
            class="resource-search"
            bind:value={resourceSearch}
            placeholder="搜索名称、端点或标签"
            aria-label="搜索资源"
          />
          <select bind:value={resourceStatusFilter} aria-label="连接状态"
            ><option value="all">全部状态</option><option value="active"
              >正常</option
            ><option value="disabled">已停用</option><option value="unknown"
              >未知</option
            ></select
          >
          <select bind:value={resourceLevelFilter} aria-label="资源级别"
            ><option value="all">全部级别</option><option value="platform"
              >平台级</option
            ><option value="team">团队级</option><option value="project"
              >项目级</option
            ></select
          >
          <button
            class="primary resource-add-menu-trigger"
            type="button"
            on:click={toggleResourceAddMenu}
            aria-expanded={resourceAddMenuOpen}
            ><Plus
              size={15}
              strokeWidth={2}
              aria-hidden="true"
            />添加资源</button
          >
        </div>
      </div>
      <div class="table-list resource-list">
        {#each resourceCatalogItems as resource}
          {@const resourceCheck = resourceConnectionChecks[resource.id]}
          <details
            class:selected={selectedResourceId === resource.id}
            class:provider-resource-row={resource.kind === 'AIProvider'}
            class:mcp-resource-row={resource.kind === 'MCPServer'}
            class="resource-catalog-row"
            on:toggle={() => {
              void loadResourceDetails(resource.id);
              if (resource.kind === 'MCPServer')
                void loadMCPSnapshots(resource.id);
            }}
          >
            <summary>
              <span class="entity-summary"
                ><span class="entity-icon resource-icon"
                  >{#if brandNameFor(resource)}<BrandIcon
                      name={brandNameFor(resource)}
                      size={18}
                    />{:else}{resourceIcon(resource.kind)}{/if}</span
                ><span
                  ><strong>{resource.name}</strong><small
                    >{resourceEndpointFor(resource)}</small
                  ></span
                ></span
              >
              <span class="resource-cell resource-category-cell">
                {#if resource.kind === 'AIProvider'}
                  {@const models = providerModelsForResource(resource)}
                  {@const currentModel =
                    providerDefaultModelForResource(resource)}
                  <strong
                    >{providerTypeLabel(resource.config?.provider_type)} · {String(
                      currentModel?.name ?? '未设置'
                    )}</strong
                  ><small>模型 · 共 {models.length} 个</small>
                {:else if resource.kind === 'MCPServer'}<strong
                    >MCPServer</strong
                  ><small>{resourceSubtypeFor(resource)}</small>
                {:else}<strong>{resourceCategoryFor(resource)}</strong><small
                    >{resourceSubtypeFor(resource)}</small
                  >{/if}
              </span>
              <span class="resource-cell resource-scope-cell"
                ><strong class="scope-pill {scopeType(resource.scope_id)}"
                  >{resourceScopeLabel(resource)}</strong
                ><small>级别</small></span
              >
              {#if resource.kind === 'AIProvider'}<span
                  class="resource-cell provider-purpose-cell"
                  title="角色表示特定场景的调用优先级；同级别每个角色最多绑定一个 Provider。"
                  ><span class="provider-purpose-tags"
                    >{#each providerBindingsFor(resource) as binding}<span
                        class="resource-tag provider-purpose-tag"
                        >{providerPurposeLabel(binding.tag)}</span
                      >{:else}<small class="resource-tags-empty">未设置</small
                      >{/each}</span
                  ><small>角色</small></span
                >{/if}
              <span
                class="resource-tags"
                class:provider-capabilities-cell={resource.kind ===
                  'AIProvider'}
                aria-label={resource.kind === 'AIProvider'
                  ? '模型能力'
                  : '资源标签'}
                >{#if resource.kind === 'AIProvider'}{#each providerModelCapabilities(providerDefaultModelForResource(resource)) as capability}<span
                      class="resource-tag provider-capability-tag"
                      >{capability}</span
                    >{:else}<small class="resource-tags-empty">未声明能力</small
                    >{/each}{:else}{#each Object.entries(resource.labels ?? {}) as [key, value]}<span
                      class="resource-tag">{key}{value ? `=${value}` : ''}</span
                    >{:else}<small class="resource-tags-empty">未设置标签</small
                    >{/each}{/if}</span
              >
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
                <span class="resource-enabled-control" title="是否启用"
                  ><span class="provider-toggle-control"
                    ><input
                      type="checkbox"
                      checked={resource.status === 'active'}
                      disabled={busy ||
                        !resourceCanManage(resource, 'resource:update')}
                      aria-label={`是否启用 ${resource.name}`}
                      on:click|stopPropagation
                      on:change={(event) =>
                        void toggleResourceEnabled(
                          resource,
                          (event.currentTarget as HTMLInputElement).checked
                        )}
                    /><i aria-hidden="true"></i></span
                  ></span
                >
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
                    <span>Provider 类型</span><strong
                      >{providerTypeLabel(providerConfig.provider_type)}</strong
                    >
                  </div>
                  <div>
                    <span>协议</span><strong
                      >{String(
                        providerConfig.protocol ?? 'chat_completions'
                      )}</strong
                    >
                  </div>
                  <div class="provider-resource-address">
                    <span>服务地址</span><strong
                      >{resourceEndpointFor(resource)}</strong
                    >
                  </div>
                  <div>
                    <span>最新更新</span><strong
                      >{formatDate(resource.updated_at)}</strong
                    >
                  </div>
                  <div>
                    <span>请求超时</span><strong
                      >{Number(providerConfig.timeout_seconds ?? 60)} 秒</strong
                    >
                  </div>
                  <div>
                    <span>最大并发</span><strong
                      >{Number(providerConfig.max_concurrency ?? 5)}</strong
                    >
                  </div>
                  <div>
                    <span>限流</span><strong
                      >{Number(providerConfig.rate_limit_per_minute ?? 0) > 0
                        ? `${Number(providerConfig.rate_limit_per_minute)} 次/分钟`
                        : '不限流'}</strong
                    >
                  </div>
                  <div>
                    <span>默认 Model</span><strong
                      >{String(
                        providerConfig.default_model ??
                          providerModels[0]?.name ??
                          '未设置'
                      )}</strong
                    >
                  </div>
                  <div>
                    <span>API Key</span><strong
                      >{resource.credential_id
                        ? '已配置凭据'
                        : '未配置凭据'}</strong
                    >
                  </div>
                  <div>
                    <span>Provider 状态</span><strong
                      >{resource.status === 'active'
                        ? '已启用'
                        : resource.status === 'disabled'
                          ? '已停用'
                          : '未知'}</strong
                    >
                  </div>
                  <div
                    class="provider-resource-labels"
                    data-tooltip={resourceLabelText || undefined}
                  >
                    <span>标签</span><strong
                      >{resourceLabelText || '未设置标签'}</strong
                    >
                  </div>
                  <div class="provider-resource-connection">
                    <span>连接测试</span><strong
                      >{resourceCheck
                        ? resourceCheck.status === 'succeeded'
                          ? `正常 · ${resourceCheck.latency_ms} ms`
                          : '失败'
                        : '尚未测试'}</strong
                    >
                  </div>
                </div>
                <div class="provider-resource-models">
                  <div class="provider-resource-models-heading">
                    <strong>Model 列表</strong><span
                      >{providerModels.length} 个</span
                    >
                  </div>
                  {#each providerModels as model}{@const modelName = String(
                      model.name ?? '未命名 Model'
                    )}{@const modelIsDefault =
                      modelName ===
                        String(providerConfig.default_model ?? '').trim() ||
                      (!providerConfig.default_model &&
                        model === providerModels[0])}
                    <div class="provider-resource-model-row">
                      <div class="provider-resource-model-name">
                        <strong>{modelName}</strong><small
                          >{modelIsDefault ? '默认 Model' : '备用 Model'} · {model.enabled ===
                          false
                            ? '已停用'
                            : '已启用'}</small
                        >
                      </div>
                      <div>
                        <span>能力</span><strong
                          >{providerModelCapabilities(model).join('、') ||
                            '未声明'}</strong
                        >
                      </div>
                      <div>
                        <span>上下文</span><strong
                          >{Number(
                            model.context_window_tokens ??
                              model.context_window ??
                              128000
                          ).toLocaleString()} Token</strong
                        >
                      </div>
                      <div>
                        <span>最大输出</span><strong
                          >{Number(
                            model.max_output_tokens ?? 128000
                          ).toLocaleString()} Token</strong
                        >
                      </div>
                      <div>
                        <span>温度</span><strong
                          >{Number(model.temperature ?? 0.7)}</strong
                        >
                      </div>
                      <div>
                        <span>优先级</span><strong
                          >{Number(model.priority ?? 0)}</strong
                        >
                      </div>
                    </div>{:else}<div class="empty-state">
                      尚未配置 Model。
                    </div>{/each}
                </div>
              </div>
            {:else if resource.kind === 'MCPServer'}
              {@const mcpConfig = resource.config ?? {}}
              {@const mcpSnapshot = (operationSnapshots[resource.id] ?? [])[0]}
              <div class="provider-resource-details mcp-resource-details">
                <div class="provider-resource-meta">
                  <div>
                    <span>传输方式</span><strong
                      >{String(mcpConfig.transport ?? 'streamable_http') ===
                      'sse'
                        ? 'SSE'
                        : 'Streamable HTTP'}</strong
                    >
                  </div>
                  <div class="provider-resource-address">
                    <span>Server 地址</span><strong
                      >{resourceEndpointFor(resource)}</strong
                    >
                  </div>
                  <div>
                    <span>工具数量</span><strong
                      >{mcpSnapshot?.tools?.length ?? 0} 个</strong
                    >
                  </div>
                  <div>
                    <span>最新更新</span><strong
                      >{formatDate(resource.updated_at)}</strong
                    >
                  </div>
                  <div>
                    <span>超时时间</span><strong
                      >{Number(mcpConfig.timeout_seconds ?? 120)} 秒</strong
                    >
                  </div>
                  <div>
                    <span>响应体大小限制</span><strong
                      >{Math.round(
                        Number(
                          mcpConfig.max_response_bytes ?? 4 * 1024 * 1024
                        ) /
                          1024 /
                          1024
                      )} MiB</strong
                    >
                  </div>
                  <div>
                    <span>自定义 Header</span><strong
                      >{resource.credential_id ? '已配置' : '未配置'}</strong
                    >
                  </div>
                  <div>
                    <span>工具白名单</span><strong
                      >{Array.isArray(mcpConfig.tool_allowlist) &&
                      mcpConfig.tool_allowlist.length
                        ? `${mcpConfig.tool_allowlist.length} 条规则`
                        : '不限制'}</strong
                    >
                  </div>
                  <div>
                    <span>访问凭据</span><strong
                      >{resource.credential_id
                        ? '已配置 Token / Header'
                        : '未配置'}</strong
                    >
                  </div>
                  <div class="provider-resource-labels">
                    <span>标签</span><strong
                      >{resourceLabelsText(resource) || '未设置标签'}</strong
                    >
                  </div>
                  <div>
                    <span>MCPServer状态</span><strong
                      >{resource.status === 'active'
                        ? '已启用'
                        : resource.status === 'disabled'
                          ? '已停用'
                          : '未知'}</strong
                    >
                  </div>
                  <div>
                    <span>连接测试</span><strong
                      >{mcpSnapshot
                        ? mcpSnapshot.status === 'succeeded'
                          ? `正常 · ${mcpSnapshot.latency_ms ?? 0} ms`
                          : '失败'
                        : '尚未测试'}</strong
                    >
                  </div>
                </div>
                <div class="provider-resource-models mcp-resource-tools">
                  <div class="provider-resource-models-heading">
                    <strong>工具列表</strong><span
                      >{mcpSnapshot?.tools?.length ?? 0} 个</span
                    >
                  </div>
                  {#each mcpSnapshot?.tools ?? [] as tool}<div
                      class="provider-resource-model-row mcp-resource-tool-row"
                    >
                      <div>
                        <span>工具名称</span><strong>{tool.name}</strong>
                      </div>
                      <div class="mcp-tool-description">
                        <span>工具说明</span><strong
                          >{tool.description || '未提供说明'}</strong
                        >
                      </div>
                    </div>{:else}<div class="empty-state">
                      尚未发现工具，请先执行连接测试。
                    </div>{/each}
                </div>
              </div>
            {:else}<div class="resource-row-details">
                <div>
                  <span>资源地址</span><strong
                    >{resourceEndpointFor(resource)}</strong
                  >
                </div>
                <div>
                  <span>最新更新</span><strong
                    >{formatDate(resource.updated_at)}</strong
                  >
                </div>
                <div>
                  <span>连接测试</span><strong
                    >{selectedResourceId === resource.id && connectionCheck
                      ? connectionCheck.status === 'succeeded'
                        ? '连接正常'
                        : '连接失败'
                      : '展开后可测试'}</strong
                  >
                </div>
                <div>
                  <span>管理范围</span><strong
                    >{resourceCanManage(resource, 'resource:update')
                      ? '当前 Scope 可管理'
                      : '继承资源，仅限查看'}</strong
                  >
                </div>
              </div>{/if}
          </details>
        {:else}<div class="empty-state">没有匹配的资源。</div>{/each}
      </div>
    </section>
    {#if resourceAddMenuOpen}
      <section
        class="panel resource-add-workflow"
        aria-labelledby="resource-add-title"
      >
        <header class="resource-add-main-heading">
          <div>
            <p class="eyebrow">
              {editingProviderResourceId || editingResourceId
                ? 'EDIT RESOURCE'
                : 'ADD RESOURCE'}
            </p>
            <h2 id="resource-add-title">
              <span
                >{editingProviderResourceId || editingResourceId
                  ? '编辑资源'
                  : '添加资源'}</span
              >
              {#if resourceAddCategory && resourceAddSubtype}<small
                  >{resourceAddCategory} · {resourceAddSubtype}</small
                >{/if}
            </h2>
          </div>
          <button
            class="secondary"
            type="button"
            on:click={() => {
              resourceAddMenuOpen = false;
              resourceAddStep = 1;
              editingProviderResourceId = '';
              editingResourceId = '';
            }}>取消</button
          >
        </header>
        <aside class="resource-add-steps" aria-label="添加资源步骤">
          <button
            class:active={resourceAddStep === 1}
            class:done={resourceAddStep > 1}
            type="button"
            on:click={() => (resourceAddStep = 1)}
            ><b>1</b><span>基础配置</span></button
          >
          {#if resourceKind === 'AIProvider'}
            <button
              class:active={resourceAddStep === 2}
              class:done={resourceAddStep > 2}
              disabled={!resourceBasicConfigurationComplete()}
              type="button"
              on:click={() => {
                if (resourceBasicConfigurationComplete()) resourceAddStep = 2;
              }}><b>2</b><span>Provider 配置</span></button
            >
            <button
              class:active={resourceAddStep === 3}
              class:done={resourceAddStep > 3}
              type="button"
              on:click={() => (resourceAddStep = 3)}
              ><b>3</b><span>Model 配置</span></button
            >
            <button
              class:active={resourceAddStep === 4}
              disabled={providerModels.length === 0}
              type="button"
              on:click={() => {
                if (providerModels.length > 0) resourceAddStep = 4;
              }}><b>4</b><span>总结核验</span></button
            >
          {:else if resourceKind === 'MCPServer'}
            <button
              class:active={resourceAddStep === 2}
              class:done={resourceAddStep > 2}
              disabled={!resourceBasicConfigurationComplete()}
              type="button"
              on:click={() => {
                if (resourceBasicConfigurationComplete()) resourceAddStep = 2;
              }}><b>2</b><span>MCP 配置</span></button
            >
            <button
              class:active={resourceAddStep === 3}
              disabled={!mcpConfigurationValid()}
              type="button"
              on:click={() => {
                if (mcpConfigurationValid()) resourceAddStep = 3;
              }}><b>3</b><span>总结核验</span></button
            >
          {:else}
            <button
              class:active={resourceAddStep === 2}
              disabled={!resourceBasicConfigurationComplete()}
              type="button"
              on:click={() => {
                if (resourceBasicConfigurationComplete()) resourceAddStep = 2;
              }}><b>2</b><span>配置资源</span></button
            >
          {/if}
        </aside>
        <div
          class="resource-add-content"
          class:type-selection-content={resourceAddStep === 1}
        >
          {#if activeMessage}<MessageBanner
              message={activeMessage}
              tone={activeMessageTone}
            />{/if}
          <h3>{resourceAddStepTitle(resourceAddStep, resourceKind)}</h3>
          {#if resourceAddStepValidationMessage()}<p
              class="resource-add-step-validation"
              role="alert"
            >
              {resourceAddStepValidationMessage()}
            </p>{/if}
          <div class="resource-add-step-actions">
            {#if resourceAddStep === 1}<button
                class="primary"
                type="button"
                on:click={continueResourceAdd}>下一步</button
              >
            {:else if resourceKind === 'AIProvider' && resourceAddStep === 2}<button
                class="secondary"
                type="button"
                on:click={() => (resourceAddStep = 1)}>上一步</button
              ><button
                class="primary"
                type="button"
                on:click={continueProviderAdd}>下一步</button
              >
            {:else if resourceKind === 'AIProvider' && resourceAddStep === 3}<button
                class="secondary"
                type="button"
                on:click={() => (resourceAddStep = 2)}>上一步</button
              ><button
                class="primary"
                type="button"
                on:click={continueProviderAdd}>下一步</button
              >
            {:else if resourceKind === 'AIProvider' && resourceAddStep === 4}<button
                class="secondary"
                type="button"
                on:click={() => (resourceAddStep = 3)}>上一步</button
              ><button
                class="primary"
                type="submit"
                form="provider-create-form"
                disabled={busy || !selectedScopeId}
                >{editingProviderResourceId ? '保存' : '创建'}</button
              >
            {:else if resourceKind === 'MCPServer' && resourceAddStep === 2}<button
                class="secondary"
                type="button"
                on:click={() => (resourceAddStep = 1)}>上一步</button
              ><button
                class="primary"
                type="button"
                on:click={() => {
                  mcpConfigurationAttempted = true;
                  if (mcpConfigurationValid()) {
                    mcpConfigurationAttempted = false;
                    resourceAddStep = 3;
                  }
                }}>下一步</button
              >
            {:else if resourceKind === 'MCPServer' && resourceAddStep === 3}<button
                class="secondary"
                type="button"
                on:click={() => (resourceAddStep = 2)}>上一步</button
              ><button
                class="primary"
                type="button"
                on:click={() =>
                  void (editingResourceId
                    ? updateMCPFromWorkflow()
                    : createResource())}
                disabled={busy || !selectedScopeId}
                >{editingResourceId ? '保存' : '创建'}</button
              >
            {:else if resourceAddStep === 2}<button
                class="secondary"
                type="button"
                on:click={() => (resourceAddStep = 1)}>上一步</button
              ><button
                class="primary"
                type="submit"
                form="resource-create-form"
                disabled={busy || !selectedScopeId}>创建</button
              >{/if}
          </div>

          {#if resourceAddStep === 1}
            <p class="resource-add-description">
              配置资源的基础身份、归属和标签；资源类型、子类型与名称为必填项。
            </p>
            <div class="resource-type-selection">
              <div class="resource-basic-type-row">
                <label
                  class:invalid={resourceTypeSelectionAttempted &&
                    !resourceAddCategory}
                  ><span><i>*</i>资源类型</span><select
                    bind:value={resourceAddCategory}
                    disabled={Boolean(
                      editingProviderResourceId || editingResourceId
                    )}
                    on:change={(event) =>
                      selectResourceAddCategory(
                        (event.currentTarget as HTMLSelectElement).value
                      )}
                    ><option value="">请选择资源类型</option
                    >{#each Object.keys(resourceCategoryOptions).filter((category) => category !== '全部') as category}<option
                        value={category}>{category}</option
                      >{/each}</select
                  ></label
                >
                <label
                  class:invalid={resourceTypeSelectionAttempted &&
                    !resourceAddSubtype}
                  ><span><i>*</i>资源子类型</span><select
                    bind:value={resourceAddSubtype}
                    disabled={!resourceAddCategory ||
                      Boolean(editingProviderResourceId || editingResourceId)}
                    on:change={(event) => {
                      resourceAddSubtype = (
                        event.currentTarget as HTMLSelectElement
                      ).value;
                      const schema = resourceSchemaForSelection(
                        resourceAddCategory,
                        resourceAddSubtype
                      );
                      resourceKind =
                        resourceAddCategory === 'LLM' &&
                        resourceAddSubtype === 'Provider'
                          ? 'AIProvider'
                          : (schema?.kind ?? '');
                    }}
                    ><option value="">请选择资源子类型</option
                    >{#each resourceAddSubtypeOptions as subtype}<option
                        value={subtype}>{subtype}</option
                      >{/each}</select
                  ></label
                >
              </div>
              <div class="resource-basic-identity-row">
                <label
                  class="resource-basic-name"
                  class:invalid={resourceBasicConfigurationAttempted &&
                    !resourceName.trim()}
                  ><span><i>*</i>资源名称</span><input
                    bind:value={resourceName}
                    required
                    placeholder="例如 production-resource"
                    autocomplete="off"
                  /></label
                >
                <label class="resource-basic-level"
                  ><span>资源级别</span><input
                    value={activeScopeSummary()}
                    readonly
                    aria-readonly="true"
                  /></label
                >
                <label class="resource-basic-enabled"
                  ><span>是否启用</span><span class="provider-toggle-control"
                    ><input
                      type="checkbox"
                      checked={resourceStatus === 'active'}
                      on:change={(event) =>
                        (resourceStatus = (
                          event.currentTarget as HTMLInputElement
                        ).checked
                          ? 'active'
                          : 'disabled')}
                      aria-label="是否启用资源"
                    /><i aria-hidden="true"></i></span
                  ></label
                >
              </div>
              <label class="resource-basic-labels"
                ><span>资源标签</span><input
                  bind:value={resourceLabels}
                  placeholder="填写 key=value，多个标签用逗号分隔，例如 env=prod, owner=platform"
                  autocomplete="off"
                /></label
              >
            </div>
          {:else if resourceKind === 'MCPServer' && resourceAddStep === 2}
            <p class="resource-add-description">
              配置 MCP Server 的连接参数。增强安全模式下服务地址必须使用 HTTPS。
            </p>
            <form
              id="resource-create-form"
              class="stack-form resource-create-form"
              on:submit|preventDefault={createResource}
            >
              <div class="mcp-resource-form">
                <label class="mcp-url-field"
                  ><span><i>*</i>Server 地址</span><input
                    bind:value={mcpURL}
                    type="url"
                    placeholder="https://mcp.example.com/mcp"
                    autocomplete="off"
                  /></label
                ><label
                  ><span>Token</span><input
                    bind:value={mcpToken}
                    type="password"
                    placeholder="保存于加密凭据"
                    autocomplete="new-password"
                  /></label
                >
                <div class="mcp-number-grid">
                  <label
                    ><span>超时时间（秒）</span><input
                      bind:value={mcpTimeoutSeconds}
                      type="number"
                      min="1"
                      max="600"
                    /></label
                  ><label
                    ><span>响应体大小限制（字节）</span><input
                      bind:value={mcpMaxResponseBytes}
                      type="number"
                      min="1"
                      max="16777216"
                      step="1024"
                    /></label
                  >
                </div>
                <label
                  ><span>请求 Header</span><textarea
                    bind:value={mcpRequestHeaders}
                    rows="3"
                    placeholder="每行一个 Header，例如 X-Tenant: production"
                  ></textarea></label
                ><label class="mcp-tools-field"
                  ><span>工具白名单</span><textarea
                    bind:value={mcpToolAllowlist}
                    rows="4"
                    placeholder="支持通配符，例如 docker:*&#10;为空表示允许全部工具"
                    spellcheck="false"
                  ></textarea></label
                >
              </div>
            </form>
          {:else if resourceKind === 'MCPServer' && resourceAddStep === 3}
            <div class="provider-summary mcp-summary">
              <div>
                <span>传输方式</span><strong
                  >{mcpTransport === 'sse' ? 'SSE' : 'Streamable HTTP'}</strong
                >
              </div>
              <div>
                <span>Server 地址</span><strong>{mcpURL || '未设置'}</strong>
              </div>
              <div>
                <span>Token / Header</span><strong
                  >{mcpToken.trim() ? 'Token 已配置' : 'Token 未配置'}</strong
                ><small>{mcpHeaderCount()} 个请求 Header</small>
              </div>
              <div>
                <span>工具白名单</span><strong
                  >{mcpToolAllowlist.trim() || '不限制'}</strong
                ><small>支持通配符，空白表示允许全部工具</small>
              </div>
              <div>
                <span>超时时间</span><strong>{mcpTimeoutSeconds} 秒</strong>
              </div>
              <div>
                <span>响应体大小限制</span><strong
                  >{Math.round(Number(mcpMaxResponseBytes) / 1024 / 1024)} MiB</strong
                >
              </div>
              <div class="provider-test-summary">
                <span>连接核验</span
                >{#if mcpDraftTest?.result?.status === 'succeeded'}<strong
                    class="success"
                    >连接正常 · 发现 {mcpDraftTest.result.tools.length} 个工具{mcpDraftTest
                      .result.latency_ms
                      ? ` · ${mcpDraftTest.result.latency_ms} ms`
                      : ''}</strong
                  ><small>Server 初始化和工具发现已完成</small
                  >{:else if mcpDraftTest?.error}<strong class="failed"
                    >{mcpDraftTest.error}</strong
                  ><small>请修正配置后重新测试</small>{:else}<strong
                    >尚未核验</strong
                  ><small>创建前必须完成连接测试</small>{/if}<button
                  class="secondary provider-test-button"
                  type="button"
                  on:click={() => void testMCPDraftConnection()}
                  disabled={mcpDraftTestBusy}
                  >{mcpDraftTestBusy ? '连接中…' : '连接测试'}</button
                >
              </div>
            </div>
          {:else if resourceKind === 'AIProvider' && resourceAddStep === 2}
            <p class="resource-add-description">
              配置 Provider
              连接、运行边界和角色。凭据会作为独立加密对象保存，不会写入资源配置。
            </p>
            <div class="provider-config-form">
              <label
                class="provider-config-type"
                class:invalid={providerConfigurationAttempted && !providerType}
                ><span><i>*</i>Provider类型</span><select
                  bind:value={providerType}
                  on:change={(event) =>
                    selectProviderType(
                      (event.currentTarget as HTMLSelectElement).value
                    )}
                  >{#each providerTypeOptions as option}<option
                      value={option.value}>{option.label}</option
                    >{/each}</select
                ></label
              >
              <label class="provider-config-protocol"
                ><span>Provider协议</span><select bind:value={providerProtocol}
                  ><option value="chat_completions">Chat Completions</option
                  ></select
                ></label
              >
              <div class="provider-purpose-options provider-config-purpose">
                <span>Provider角色</span>
                <div>
                  {#each providerPurposeOptions as purpose}<button
                      class:active={providerPurposeTags.includes(purpose.value)}
                      type="button"
                      aria-pressed={providerPurposeTags.includes(purpose.value)}
                      on:click={() => toggleProviderPurpose(purpose.value)}
                      >{purpose.label}</button
                    >{/each}
                </div>
              </div>
              <label
                class="provider-config-url"
                class:invalid={providerConfigurationAttempted &&
                  !providerBaseURLValid()}
                ><span><i>*</i>服务地址</span><input
                  bind:value={providerBaseURL}
                  required
                  type="url"
                  placeholder="https://api.example.com/v1"
                  autocomplete="off"
                /></label
              >
              <label
                class="provider-config-api-key"
                class:invalid={providerConfigurationAttempted &&
                  !providerAPIKey.trim()}
                ><span>API Key</span><span class="provider-api-key-control"
                  ><input
                    bind:value={providerAPIKey}
                    required
                    type={providerAPIKeyVisible ? 'text' : 'password'}
                    placeholder={providerAPIKeyLoading
                      ? '正在读取 API Key…'
                      : '请输入 API Key'}
                    autocomplete="new-password"
                  /><button
                    class="provider-api-key-toggle"
                    type="button"
                    aria-label={providerAPIKeyVisible
                      ? '隐藏 API Key'
                      : '显示 API Key'}
                    aria-pressed={providerAPIKeyVisible}
                    data-tooltip={providerAPIKeyVisible
                      ? '隐藏 API Key'
                      : '显示 API Key'}
                    on:click={() =>
                      (providerAPIKeyVisible = !providerAPIKeyVisible)}
                    >{#if providerAPIKeyVisible}<EyeOff
                        size={16}
                        strokeWidth={1.8}
                        aria-hidden="true"
                      />{:else}<Eye
                        size={16}
                        strokeWidth={1.8}
                        aria-hidden="true"
                      />{/if}</button
                  ></span
                ></label
              >
              <label class="provider-config-timeout"
                ><span>请求超时（秒）</span><input
                  bind:value={providerTimeoutSeconds}
                  min="1"
                  max="300"
                  type="number"
                /></label
              ><label class="provider-config-concurrency"
                ><span>最大并发</span><input
                  bind:value={providerMaxConcurrency}
                  min="1"
                  type="number"
                /></label
              ><label class="provider-config-rate-limit"
                ><span>限流（请求/分钟）</span><input
                  bind:value={providerRateLimitPerMinute}
                  min="0"
                  type="number"
                /></label
              >
            </div>
          {:else if resourceKind === 'AIProvider' && resourceAddStep === 3}
            <p class="resource-add-description">
              添加此 Provider
              可用的模型；第一个添加的模型会自动设为默认模型，也可在下方调整。
            </p>
            <div class="provider-model-editor">
              <div class="provider-model-grid">
                <label
                  class:invalid={providerModelConfigurationAttempted &&
                    (!providerModelDraft.name.trim() ||
                      providerModels.some(
                        (model) =>
                          model.name === providerModelDraft.name.trim() &&
                          model.name !== editingProviderModelName
                      ))}
                  ><span><i>*</i>Model 名称</span><input
                    bind:value={providerModelDraft.name}
                    required
                    placeholder="例如 gpt-4.1"
                    autocomplete="off"
                  /></label
                ><label
                  class:invalid={providerModelConfigurationAttempted &&
                    providerModelDraft.contextWindowTokens <= 0}
                  ><span><i>*</i>上下文窗口</span><input
                    bind:value={providerModelDraft.contextWindowTokens}
                    min="1"
                    required
                    type="number"
                  /></label
                ><label
                  ><span>最大输出 Token</span><input
                    bind:value={providerModelDraft.maxOutputTokens}
                    min="1"
                    type="number"
                  /></label
                ><label
                  ><span>优先级</span><input
                    bind:value={providerModelDraft.priority}
                    min="0"
                    type="number"
                  /></label
                ><label
                  ><span>温度</span><span class="provider-temperature-control"
                    ><input
                      bind:value={providerModelDraft.temperature}
                      min="0"
                      max="2"
                      step="0.1"
                      type="number"
                    /><span
                      class="provider-temperature-toggle"
                      data-tooltip="允许调用时调整温度参数"
                      ><input
                        type="checkbox"
                        bind:checked={providerModelDraft.temperatureMutable}
                        aria-label="温度可调"
                      /><i aria-hidden="true"></i></span
                    ></span
                  ></label
                >
              </div>
              <div class="provider-model-capabilities-row">
                <div
                  class="provider-model-flags"
                  class:invalid={providerModelConfigurationAttempted &&
                    providerModelDraft.capabilities.length === 0}
                >
                  <span><i>*</i>支持能力</span
                  >{#each providerCapabilityOptions as capability}<button
                      class:active={providerModelDraft.capabilities.includes(
                        capability.value
                      )}
                      type="button"
                      on:click={() =>
                        toggleProviderModelCapability(capability.value)}
                      >{capability.label}</button
                    >{/each}
                </div>
                <button
                  class="secondary"
                  type="button"
                  on:click={addProviderModel}
                  >{editingProviderModelName ? '保存修改' : '添加模型'}</button
                >
              </div>
            </div>
            <div class="provider-model-list">
              <div class="provider-model-list-heading">
                <strong>已配置模型</strong><span
                  >{providerModels.length} 个</span
                >
              </div>
              {#each providerModels as model}<div class="provider-model-row">
                  <strong>{model.name}</strong><span
                    >{model.contextWindowTokens.toLocaleString()} Token · 温度 {model.temperature}</span
                  ><span
                    >{model.capabilities
                      .map(
                        (capability) =>
                          providerCapabilityOptions.find(
                            (item) => item.value === capability
                          )?.label ?? capability
                      )
                      .join('、')}</span
                  ><label class="provider-model-default"
                    ><input
                      type="radio"
                      name="provider-default-model"
                      value={model.name}
                      checked={providerDefaultModel === model.name}
                      on:change={() => setProviderDefaultModel(model.name)}
                    /> 默认</label
                  ><label
                    class="provider-model-enabled"
                    data-tooltip={model.name === providerDefaultModel
                      ? '默认模型必须保持启用'
                      : model.enabled
                        ? '模型已启用，可被 Provider 调用'
                        : '模型已停用，不会被 Provider 调用'}
                    ><span class="visually-hidden">启用 {model.name}</span><span
                      class="provider-toggle-control"
                      ><input
                        type="checkbox"
                        checked={model.enabled}
                        disabled={model.name === providerDefaultModel}
                        aria-label={'启用 ' + model.name}
                        on:change={(event) =>
                          setProviderModelEnabled(
                            model.name,
                            (event.currentTarget as HTMLInputElement).checked
                          )}
                      /><i aria-hidden="true"></i></span
                    ></label
                  ><button
                    class="icon-button"
                    type="button"
                    aria-label={`编辑 ${model.name}`}
                    title="编辑模型"
                    on:click={() => editProviderModel(model)}
                    ><Pencil size={14} aria-hidden="true" /></button
                  ><button
                    class="icon-button danger-action"
                    type="button"
                    aria-label={`删除 ${model.name}`}
                    title="删除模型"
                    on:click={() => removeProviderModel(model.name)}
                    ><Trash2 size={14} aria-hidden="true" /></button
                  >
                </div>{:else}<div class="empty-state">
                  尚未添加模型。
                </div>{/each}
            </div>
          {:else if resourceKind === 'AIProvider' && resourceAddStep === 4}
            <p class="resource-add-description">
              使用默认 Model 完成连接核验后，才可创建 Provider 并发布以下 Model
              列表。
            </p>
            <form
              id="provider-create-form"
              class="provider-summary"
              on:submit|preventDefault={submitProviderCreate}
            >
              <div>
                <span>Provider</span><strong>{resourceName}</strong><small
                  >{providerTypeOptions.find(
                    (item) => item.value === providerType
                  )?.label} · {resourceStatus === 'active'
                    ? '已启用'
                    : '未启用'}</small
                >
              </div>
              <div>
                <span>服务地址</span><strong>{providerBaseURL}</strong><small
                  >{providerProtocol} · 超时 {providerTimeoutSeconds} 秒 · 并发 {providerMaxConcurrency}</small
                >
              </div>
              <div>
                <span>默认 Model</span><strong>{providerDefaultModel}</strong
                ><small
                  >共 {providerModels.length} 个 Model，凭据将加密保存</small
                >
              </div>
              <div class="provider-test-summary">
                <span>连接核验</span>{#if providerDraftTestBusy}<strong
                    >正在测试默认 Model...</strong
                  ><small>请求正在发送至 {providerDefaultModel}。</small
                  >{:else if providerDraftTestPassedState}<strong
                    class="success"
                    >连接正常 · {providerDraftTest?.result?.latency_ms} ms</strong
                  ><small>{providerDraftTest?.result?.message}</small
                  >{:else if providerDraftTest?.error}<strong class="failed"
                    >连接失败</strong
                  ><small>{providerDraftTest.error}</small>{:else}<strong
                    >尚未核验</strong
                  ><small>需验证默认 Model 可成功响应后才能创建。</small
                  >{/if}<button
                  class="secondary provider-test-button"
                  type="button"
                  disabled={providerDraftTestBusy}
                  on:click={() => void testProviderDraftConnection()}
                  >{providerDraftTestBusy ? '测试中' : '连接测试'}</button
                >
              </div>
              <div>
                <span>资源属性</span><strong>{activeScopeSummary()}</strong
                ><small
                  >{resourceLabelsText({
                    labels: parseLabels(resourceLabels)
                  } as Resource)
                    ? '已配置的资源标签'
                    : '未配置资源标签'}</small
                >
              </div>
              <div>
                <span>Provider角色</span><strong
                  >{providerPurposeTags.length > 0
                    ? providerPurposeTags.map(providerPurposeLabel).join('、')
                    : '未设置'}</strong
                ><small>同级别同一角色会自动路由至此 Provider。</small>
              </div>
              <div class="provider-summary-models">
                <span>Model 列表</span>{#each providerModels as model}<div>
                    <strong>{model.name}</strong><small
                      >{model.contextWindowTokens.toLocaleString()} Token · {model.capabilities
                        .map(
                          (capability) =>
                            providerCapabilityOptions.find(
                              (item) => item.value === capability
                            )?.label ?? capability
                        )
                        .join('、')}</small
                    >
                  </div>{/each}
              </div>
            </form>
          {:else if selectedResource?.kind === 'MCPServer'}
            <div class="mcp-resource-form editor-mcp-form">
              <div class="form-row">
                <label
                  ><span>资源名称</span><input
                    bind:value={editResourceName}
                    required
                  /></label
                ><label
                  ><span>状态</span><select bind:value={editResourceStatus}
                    ><option value="active">正常</option><option
                      value="disabled">停用</option
                    ><option value="unknown">未知</option></select
                  ></label
                >
              </div>
              <label
                ><span>标签</span><input
                  bind:value={editResourceLabels}
                  placeholder="env=prod, owner=platform"
                /></label
              ><label
                ><span>Server 地址</span><input
                  bind:value={mcpURL}
                  type="url"
                  placeholder="https://mcp.example.com/mcp"
                /></label
              >
              <div class="mcp-number-grid">
                <label
                  ><span>超时时间（秒）</span><input
                    bind:value={mcpTimeoutSeconds}
                    type="number"
                    min="1"
                    max="600"
                  /></label
                ><label
                  ><span>响应体大小限制（字节）</span><input
                    bind:value={mcpMaxResponseBytes}
                    type="number"
                    min="1"
                    max="16777216"
                    step="1024"
                  /></label
                >
              </div>
              <label
                ><span>Token</span><input
                  bind:value={mcpToken}
                  type="password"
                  placeholder="留空保持原凭据"
                /></label
              ><label
                ><span>请求 Header</span><textarea
                  bind:value={mcpRequestHeaders}
                  rows="3"
                  placeholder="每行一个 Header，例如 X-Tenant: production"
                ></textarea></label
              ><label
                ><span>工具白名单</span><textarea
                  bind:value={mcpToolAllowlist}
                  rows="5"
                  placeholder="支持通配符，例如 docker:*&#10;为空表示允许全部工具"
                ></textarea></label
              >
            </div>
          {:else}
            <p class="resource-add-description">
              配置将按 {resourceAddCategory} · {resourceAddSubtype} 的资源契约保存；敏感字段会单独加密存储。
            </p>
            <form
              id="resource-create-form"
              class="stack-form resource-create-form"
              on:submit|preventDefault={createResource}
            >
              {#if createSchema?.schema.properties}<div class="schema-inputs">
                  <p class="eyebrow">SCHEMA FIELDS</p>
                  {#each Object.entries(createSchema.schema.properties) as [key, field]}<label
                      >{field.title || key}{#if field.sensitive}<input
                          type="password"
                          bind:value={resourceSensitiveValues[key]}
                          placeholder="敏感信息将加密保存"
                          autocomplete="new-password"
                        />{:else if field.enum}<select
                          bind:value={resourceConfigValues[key]}
                          ><option value="">未设置</option
                          >{#each field.enum as option}<option value={option}
                              >{option}</option
                            >{/each}</select
                        >{:else if field.type === 'array'}<textarea
                          bind:value={resourceConfigValues[key]}
                          rows="4"
                          placeholder={'JSON 数组，例如 [{"name":"model","context_window":8192}]'}
                          spellcheck="false"
                        ></textarea>{:else}<input
                          bind:value={resourceConfigValues[key]}
                          type={field.type === 'number' ||
                          field.type === 'integer'
                            ? 'number'
                            : field.type === 'url' || field.format === 'uri'
                              ? 'url'
                              : 'text'}
                          placeholder={field.description || key}
                          autocomplete="off"
                        />{/if}</label
                    >{/each}
                </div>{:else}<label
                  >配置 JSON<textarea
                    bind:value={resourceConfig}
                    rows="4"
                    spellcheck="false"
                  ></textarea></label
                >{/if}
            </form>
          {/if}
        </div>
      </section>
    {/if}
    {#if selectedResource}<section
        class="panel detail-panel"
        class:open={resourceEditorOpen}
      >
        <div class="panel-heading">
          <div>
            <p class="eyebrow">RESOURCE DETAIL</p>
            <h2 class="resource-editor-title">
              <span>编辑资源</span>
              {#if resourceCategoryFor(selectedResource) && resourceSubtypeFor(selectedResource)}
                <small
                  >{resourceCategoryFor(selectedResource)} · {resourceSubtypeFor(
                    selectedResource
                  )}</small
                >
              {/if}
            </h2>
            <p class="resource-editor-name">{selectedResource.name}</p>
            <p class="muted">
              {selectedResource.kind} · {scopeName(selectedResource.scope_id)}
            </p>
          </div>
          <button
            class="danger-button"
            on:click={deleteSelectedResource}
            disabled={busy || !selectedResourceCanDelete}
            title={selectedResourceCanDelete ? '停用资源' : '继承资源仅可查看'}
            >停用</button
          >
        </div>
        <div class="detail-meta">
          <span>状态 <strong>{selectedResource.status}</strong></span><span
            >Schema v{selectedResource.schema_version}</span
          ><span>更新于 {formatDate(selectedResource.updated_at)}</span>
        </div>
        {#if selectedResourceHasConnector && selectedResource.kind !== 'AIProvider'}
          <div class="connection-status">
            <div class="connection-summary">
              <span
                class:success={connectionCheck?.status === 'succeeded'}
                class:failed={connectionCheck?.status === 'failed'}
                class="connection-indicator"
                aria-hidden="true"
              ></span><span
                ><strong
                  >{connectionCheck?.status === 'succeeded'
                    ? '连接正常'
                    : connectionCheck?.status === 'failed'
                      ? '连接失败'
                      : '尚未测试'}</strong
                ><small
                  >{connectionCheck
                    ? `${connectionCheck.message} · ${connectionCheck.latency_ms} ms · ${formatDate(connectionCheck.checked_at)}`
                    : '当前资源还没有连接测试记录'}</small
                ></span
              >
            </div>
            {#if connectionCheck?.capabilities.length}<div
                class="capability-list"
                aria-label="连接器能力"
              >
                {#each connectionCheck.capabilities as capability}<span
                    >{capabilityName(capability)}</span
                  >{/each}
              </div>{/if}<button
              class="secondary connection-test-button"
              on:click={testSelectedResourceConnection}
              disabled={busy || connectionBusy}
              ><span aria-hidden="true">↻</span>{connectionBusy
                ? '测试中'
                : '测试连接'}</button
            >
          </div>
        {/if}
        <form
          class="stack-form editor-form"
          on:submit|preventDefault={updateSelectedResource}
        >
          {#if selectedResource.kind === 'AIProvider'}
            <div class="provider-edit-form">
              <div class="provider-config-form">
                <label
                  ><span>Provider类型</span><select bind:value={providerType}
                    >{#each providerTypeOptions as option}<option
                        value={option.value}>{option.label}</option
                      >{/each}</select
                  ></label
                >
                <label
                  ><span>Provider协议</span><select
                    bind:value={providerProtocol}
                    ><option value="chat_completions">Chat Completions</option
                    ></select
                  ></label
                >
                <div class="provider-purpose-options provider-config-purpose">
                  <span>Provider角色</span>
                  <div>
                    {#each providerPurposeOptions as purpose}<button
                        class:active={providerPurposeTags.includes(
                          purpose.value
                        )}
                        type="button"
                        aria-pressed={providerPurposeTags.includes(
                          purpose.value
                        )}
                        on:click={() => toggleProviderPurpose(purpose.value)}
                        >{purpose.label}</button
                      >{/each}
                  </div>
                </div>
                <label
                  ><span>服务地址</span><input
                    bind:value={providerBaseURL}
                    type="url"
                    required
                  /></label
                >
                <label
                  ><span>请求超时（秒）</span><input
                    bind:value={providerTimeoutSeconds}
                    type="number"
                    min="1"
                  /></label
                >
                <label
                  ><span>最大并发</span><input
                    bind:value={providerMaxConcurrency}
                    type="number"
                    min="1"
                  /></label
                >
                <label
                  ><span>限流（请求/分钟）</span><input
                    bind:value={providerRateLimitPerMinute}
                    type="number"
                    min="0"
                  /></label
                >
              </div>
              <div class="provider-model-editor">
                <div class="provider-model-grid">
                  <label
                    ><span>Model 名称</span><input
                      bind:value={providerModelDraft.name}
                    /></label
                  >
                  <label
                    ><span>上下文窗口</span><input
                      bind:value={providerModelDraft.contextWindowTokens}
                      type="number"
                      min="1"
                    /></label
                  >
                  <label
                    ><span>最大输出 Token</span><input
                      bind:value={providerModelDraft.maxOutputTokens}
                      type="number"
                      min="1"
                    /></label
                  >
                  <label
                    ><span>优先级</span><input
                      bind:value={providerModelDraft.priority}
                      type="number"
                      min="0"
                    /></label
                  >
                  <label
                    ><span>温度</span><span class="provider-temperature-control"
                      ><input
                        bind:value={providerModelDraft.temperature}
                        type="number"
                        min="0"
                        max="2"
                        step="0.1"
                      /><span
                        class="provider-temperature-toggle"
                        data-tooltip="允许调用时调整温度参数"
                        ><input
                          type="checkbox"
                          bind:checked={providerModelDraft.temperatureMutable}
                          aria-label="温度可调"
                        /><i aria-hidden="true"></i></span
                      ></span
                    ></label
                  >
                </div>
                <div class="provider-model-capabilities-row">
                  <div class="provider-model-flags">
                    <span>支持能力</span
                    >{#each providerCapabilityOptions as capability}<button
                        class:active={providerModelDraft.capabilities.includes(
                          capability.value
                        )}
                        type="button"
                        on:click={() =>
                          toggleProviderModelCapability(capability.value)}
                        >{capability.label}</button
                      >{/each}
                  </div>
                  <button
                    class="secondary"
                    type="button"
                    on:click={addProviderModel}
                    >{editingProviderModelName
                      ? '保存修改'
                      : '添加模型'}</button
                  >
                </div>
                <div class="provider-model-list">
                  {#each providerModels as model}<div
                      class="provider-model-row"
                    >
                      <strong>{model.name}</strong><span
                        >{model.contextWindowTokens.toLocaleString()} Token · 温度
                        {model.temperature}</span
                      ><span>{providerModelCapabilities(model).join('、')}</span
                      ><label class="provider-model-default"
                        ><input
                          type="radio"
                          name="provider-default-model-edit"
                          value={model.name}
                          checked={providerDefaultModel === model.name}
                          on:change={() => setProviderDefaultModel(model.name)}
                        /> 默认</label
                      ><button
                        class="icon-button"
                        type="button"
                        aria-label={`编辑 ${model.name}`}
                        on:click={() => editProviderModel(model)}
                        ><Pencil size={14} /></button
                      ><button
                        class="icon-button danger-action"
                        type="button"
                        aria-label={`删除 ${model.name}`}
                        on:click={() => removeProviderModel(model.name)}
                        ><Trash2 size={14} /></button
                      >
                    </div>{:else}<div class="empty-state">
                      尚未添加模型。
                    </div>{/each}
                </div>
              </div>
            </div>
          {:else}
            <h3 class="editor-section-title">基础配置</h3>
            <p class="editor-section-description">
              资源类型和子类型只读，资源名称、启用状态与资源标签可在此调整。
            </p>
            <div class="resource-basic-edit-grid">
              <div class="resource-basic-type-row">
                <label
                  ><span>资源类型</span><select
                    value={resourceCategoryFor(selectedResource)}
                    disabled
                    aria-label="资源类型"
                  >
                    {#each Object.keys(resourceCategoryOptions).filter((category) => category !== '全部') as category}<option
                        value={category}>{category}</option
                      >{/each}
                  </select></label
                >
                <label
                  ><span>资源子类型</span><select
                    value={resourceSubtypeFor(selectedResource)}
                    disabled
                    aria-label="资源子类型"
                  >
                    {#each resourceSubtypeOptionsFor(selectedResource) as subtype}<option
                        value={subtype}>{subtype}</option
                      >{/each}
                  </select></label
                >
              </div>
              <div class="resource-basic-identity-row">
                <label
                  ><span><i>*</i>资源名称</span><input
                    bind:value={editResourceName}
                    required
                  /></label
                >
                <label
                  ><span>资源级别</span><input
                    value={scopeName(selectedResource.scope_id)}
                    readonly
                  /></label
                >
                <label class="resource-basic-enabled"
                  ><span>是否启用</span><span class="provider-toggle-control"
                    ><input
                      type="checkbox"
                      checked={editResourceStatus === 'active'}
                      on:change={(event) =>
                        (editResourceStatus = (
                          event.currentTarget as HTMLInputElement
                        ).checked
                          ? 'active'
                          : 'disabled')}
                      aria-label="是否启用资源"
                    /><i aria-hidden="true"></i></span
                  ></label
                >
              </div>
              <label class="resource-basic-labels"
                ><span>资源标签</span><input
                  bind:value={editResourceLabels}
                  placeholder="填写 key=value，多个标签用逗号分隔，例如 env=prod, owner=platform"
                  autocomplete="off"
                /></label
              >
            </div>
            {#if selectedResource.kind === 'MCPServer'}
              <div class="mcp-resource-form editor-mcp-form">
                <label class="mcp-url-field"
                  ><span><i>*</i>Server 地址</span><input
                    bind:value={mcpURL}
                    type="url"
                    placeholder="https://mcp.example.com/mcp"
                  /></label
                >
                <div class="mcp-number-grid">
                  <label
                    ><span>超时时间（秒）</span><input
                      bind:value={mcpTimeoutSeconds}
                      type="number"
                      min="1"
                      max="600"
                    /></label
                  ><label
                    ><span>响应体大小限制（字节）</span><input
                      bind:value={mcpMaxResponseBytes}
                      type="number"
                      min="1"
                      max="16777216"
                      step="1024"
                    /></label
                  >
                </div>
                <label
                  ><span>Token</span><input
                    bind:value={mcpToken}
                    type="password"
                    placeholder="留空保持原凭据"
                  /></label
                >
                <label
                  ><span>请求 Header</span><textarea
                    bind:value={mcpRequestHeaders}
                    rows="3"
                    placeholder="每行一个 Header，例如 X-Tenant: production"
                  ></textarea></label
                >
                <label class="mcp-tools-field"
                  ><span>工具白名单</span><textarea
                    bind:value={mcpToolAllowlist}
                    rows="6"
                    placeholder="支持通配符，例如 docker:*&#10;为空表示允许全部工具"
                  ></textarea></label
                >
              </div>
            {:else if selectedSchema?.schema.properties}
              <div class="schema-inputs">
                <p class="eyebrow">SCHEMA FIELDS</p>
                {#each Object.entries(selectedSchema.schema.properties) as [key, field]}
                  <label
                    ><span
                      >{#if resourceSchemaFieldRequired(key)}<i>*</i
                        >{/if}{field.title || key}</span
                    >{#if field.sensitive}<input
                        type="password"
                        bind:value={editResourceSensitiveValues[key]}
                        placeholder={selectedResource.credential_id
                          ? '已有关联凭据，留空保持不变'
                          : '敏感信息将加密保存'}
                        autocomplete="new-password"
                      />{:else if field.enum}<select
                        bind:value={resourceConfigValues[key]}
                        ><option value="">未设置</option
                        >{#each field.enum as option}<option value={option}
                            >{option}</option
                          >{/each}</select
                      >{:else if field.type === 'array'}<textarea
                        bind:value={resourceConfigValues[key]}
                        rows="4"
                        placeholder={'JSON 数组，例如 [{"name":"model","context_window":8192}]'}
                        spellcheck="false"
                      ></textarea>{:else}<input
                        bind:value={resourceConfigValues[key]}
                        type={field.type === 'number' ||
                        field.type === 'integer'
                          ? 'number'
                          : field.type === 'url' || field.format === 'uri'
                            ? 'url'
                            : 'text'}
                        autocomplete="off"
                      />{/if}</label
                  >
                {/each}
              </div>
            {:else}
              <label
                >配置 JSON<textarea
                  bind:value={editResourceConfig}
                  rows="4"
                  spellcheck="false"
                ></textarea></label
              >
            {/if}
          {/if}
          <button
            class="secondary"
            disabled={busy || !selectedResourceCanUpdate}
            title={selectedResourceCanUpdate
              ? '保存资源修改'
              : '继承资源仅可查看'}>保存</button
          >
        </form>
        {#if selectedSchema?.schema.properties}<div class="schema-fields">
            <p class="eyebrow">SCHEMA FIELDS</p>
            {#each Object.entries(selectedSchema.schema.properties) as [key, field]}<div
              >
                <span>{field.title || key}</span><code
                  >{field.sensitive
                    ? selectedResource.credential_id
                      ? '已由加密凭据保存'
                      : '未设置'
                    : String(selectedResource.config[key] ?? '未设置')}</code
                >
              </div>{/each}
          </div>{:else}<pre class="config-preview">{JSON.stringify(
              selectedResource.config,
              null,
              2
            )}</pre>{/if}
        <div class="relation-section">
          <div class="subheading">
            <h3>关系与拓扑</h3>
            <span>{relations.length} 条关系 · {topology.length} 个节点</span>
          </div>
          <form class="relation-form" on:submit|preventDefault={createRelation}>
            <select bind:value={relationTarget} required
              ><option value="" disabled>选择目标资源</option
              >{#each resources.filter((item) => item.id !== selectedResource.id) as resource}<option
                  value={resource.id}>{resource.name} · {resource.kind}</option
                >{/each}</select
            ><select bind:value={relationType}
              ><option value="depends_on">depends_on</option><option
                value="contains">contains</option
              ><option value="deployed_on">deployed_on</option><option
                value="exposes">exposes</option
              ><option value="uses_provider">uses_provider</option></select
            ><button class="secondary" disabled={busy}>建立关系</button>
          </form>
          {#if relations.length}<div class="relation-list">
              {#each relations as relation}<div class="relation-row">
                  <span
                    ><strong>{relation.relation_type}</strong><small
                      >{resources.find(
                        (item) =>
                          item.id ===
                          (relation.source_resource_id === selectedResource.id
                            ? relation.target_resource_id
                            : relation.source_resource_id)
                      )?.name ?? '关联资源'}</small
                    ></span
                  ><button
                    class="icon-button"
                    data-tooltip="删除关系"
                    aria-label="删除关系"
                    on:click={() => deleteRelation(relation)}>×</button
                  >
                </div>{/each}
            </div>{/if}{#if topology.length}<div class="topology-list">
              {#each topology as node}<span
                  class="topology-node"
                  style={`--depth: ${node.depth}`}
                  >{node.depth} · {node.resource.name}</span
                >{/each}
            </div>{/if}
        </div>
      </section>{:else}<section class="panel empty-detail">
        <div class="empty-state">
          <span class="empty-icon">◇</span>
          <h2>选择一个资源</h2>
          <p>从左侧目录选择资源查看作用域、配置和关系。</p>
        </div>
      </section>{/if}
  </section>
</section>
