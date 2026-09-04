<script lang="ts">
  import { Pencil, PlugZap, Trash2 } from 'lucide-svelte';
  import type { ConnectionCheck, ConnectorCapability, MCPSnapshot, Relation, Resource, ResourceSchema, TopologyNode } from '../../lib/api';
  import ResourceCatalogRail from './ResourceCatalogRail.svelte';
  import ResourceCatalogToolbar from './ResourceCatalogToolbar.svelte';
  import ResourceCatalogList from './ResourceCatalogList.svelte';
  import ResourceAddStepNavigation from './ResourceAddStepNavigation.svelte';
  import ResourceAddWorkflowHeader from './ResourceAddWorkflowHeader.svelte';
  import ResourceBasicConfiguration from './ResourceBasicConfiguration.svelte';
  import MCPResourceConfiguration from './MCPResourceConfiguration.svelte';
  import MCPResourceSummary from './MCPResourceSummary.svelte';
  import ProviderResourceConfiguration from './ProviderResourceConfiguration.svelte';
  import ProviderModelConfiguration from './ProviderModelConfiguration.svelte';
  import ProviderResourceSummary from './ProviderResourceSummary.svelte';
  import MCPEditor from './MCPEditor.svelte';
  import ResourceSchemaConfiguration from './ResourceSchemaConfiguration.svelte';
  import ResourceConnectionStatus from './ResourceConnectionStatus.svelte';

  export let resourceCategoryOptions: Record<string, string[]> = {};
  export let visibleResources: Resource[] = [];
  export let resourceCategory = '全部';
  export let resourceSubtype = '全部';
  export let expandedResourceCategory = '';
  export let resourceCategoryIcon: (category: string) => string;
  export let resourceCategoryFor: (resource: any) => string;
  export let resourceSubtypeFor: (resource: any) => string;
  export let selectResourceCategory: (category: string, subtype?: string) => void;
  export let resourceSearch = '';
  export let resourceStatusFilter = 'all';
  export let resourceLevelFilter = 'all';
  export let resourceAddMenuOpen = false;
  export let toggleResourceAddMenu: () => void;
  export let resourceCatalogItems: Resource[] = [];
  export let selectedResourceId = '';
  export let resourceConnectionChecks: Record<string, ConnectionCheck | null> = {};
  export let operationSnapshots: Record<string, MCPSnapshot[]> = {};
  export let connectionCheck: ConnectionCheck | null = null;
  export let busy = false;
  export let connectionBusy = false;
  export let loadResourceDetails: (id: string) => Promise<unknown>;
  export let loadMCPSnapshots: (id: string) => Promise<unknown>;
  export let testResourceRowConnection: (resource: Resource) => Promise<unknown>;
  export let toggleResourceEnabled: (resource: Resource, enabled: boolean) => Promise<unknown>;
  export let openResourceEditor: (resource: Resource) => void;
  export let deleteSelectedResource: () => Promise<unknown>;
  export let resourceCanManage: (resource: Resource, permission: string) => boolean;
  export let resourceHasConnector: (resource: Resource) => boolean;
  export let brandNameFor: (resource: Resource) => string;
  export let resourceEndpointFor: (resource: Resource) => string;
  export let resourceScopeLabel: (resource: Resource) => string;
  export let scopeType: (id: string) => string;
  export let providerModelsForResource: (resource: Resource) => Array<Record<string, unknown>>;
  export let providerDefaultModelForResource: (resource: Resource) => Record<string, unknown> | undefined;
  export let providerModelCapabilities: (model: Record<string, unknown> | undefined) => string[];
  export let providerBindingsFor: (resource: Resource) => Array<{ tag: string }>;
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
  export let providerModels: any[] = [];
  export let providerModelDraft: any;
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
  export let providerPurposeOptions: Array<{ value: string; label: string }> = [];
  export let providerCapabilityOptions: Array<{ value: string; label: string }> = [];
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
  export let resourceSchemaForSelection: (category: string, subtype: string) => ResourceSchema | null;
  export let activeScopeSummary: () => string;
  export let selectResourceAddCategory: (category: string) => void;
</script>

+        <section class="resources-layout">
          <section class="panel resource-list-panel">
            <ResourceCatalogRail
              resourceCategoryOptions={resourceCategoryOptions}
              visibleResources={visibleResources}
              bind:resourceCategory={resourceCategory}
              bind:resourceSubtype={resourceSubtype}
              bind:expandedResourceCategory={expandedResourceCategory}
              resourceCategoryIcon={resourceCategoryIcon}
              resourceCategoryFor={resourceCategoryFor}
              resourceSubtypeFor={resourceSubtypeFor}
              onSelectCategory={selectResourceCategory}
            />
          </section>
          <section class="resource-workspace">
            <section class="panel resource-catalog-panel">
              <ResourceCatalogToolbar
                resourceCategory={resourceCategory}
                resourceSubtype={resourceSubtype}
                resourceCount={resourceCatalogItems.length}
                bind:resourceSearch={resourceSearch}
                bind:resourceStatusFilter={resourceStatusFilter}
                bind:resourceLevelFilter={resourceLevelFilter}
                resourceAddMenuOpen={resourceAddMenuOpen}
                onToggleAddMenu={toggleResourceAddMenu}
              />
              <ResourceCatalogList
                resourceCatalogItems={resourceCatalogItems}
                selectedResourceId={selectedResourceId}
                resourceConnectionChecks={resourceConnectionChecks}
                operationSnapshots={operationSnapshots}
                connectionCheck={connectionCheck}
                busy={busy}
                connectionBusy={connectionBusy}
                loadResourceDetails={loadResourceDetails}
                loadMCPSnapshots={loadMCPSnapshots}
                testResourceRowConnection={testResourceRowConnection}
                toggleResourceEnabled={toggleResourceEnabled}
                openResourceEditor={openResourceEditor}
                deleteSelectedResource={deleteSelectedResource}
                resourceCanManage={resourceCanManage}
                resourceHasConnector={resourceHasConnector}
                brandNameFor={brandNameFor}
                resourceEndpointFor={resourceEndpointFor}
                resourceCategoryFor={resourceCategoryFor}
                resourceSubtypeFor={resourceSubtypeFor}
                resourceScopeLabel={resourceScopeLabel}
                scopeType={scopeType}
                providerModelsForResource={providerModelsForResource}
                providerDefaultModelForResource={providerDefaultModelForResource}
                providerModelCapabilities={providerModelCapabilities}
                providerBindingsFor={providerBindingsFor}
                providerPurposeLabel={providerPurposeLabel}
                resourceLabelsText={resourceLabelsText}
                providerTypeLabel={providerTypeLabel}
                formatDate={formatDate}
                resourceIcon={resourceIcon}
              />
            </section>
            {#if resourceAddMenuOpen}
              <section
                class="panel resource-add-workflow"
                aria-labelledby="resource-add-title"
              >
                <header class="resource-add-main-heading">
                  <div>
                    <p class="eyebrow">{editingProviderResourceId || editingResourceId ? 'EDIT RESOURCE' : 'ADD RESOURCE'}</p>
                    <h2 id="resource-add-title">
                      <span>{editingProviderResourceId || editingResourceId ? '编辑资源' : '添加资源'}</span>
                      {#if resourceAddCategory && resourceAddSubtype}<small>{resourceAddCategory} · {resourceAddSubtype}</small>{/if}
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
                <ResourceAddStepNavigation
                  bind:resourceAddStep={resourceAddStep}
                  resourceKind={resourceKind}
                  providerModels={providerModels}
                  resourceBasicConfigurationComplete={resourceBasicConfigurationComplete}
                  mcpConfigurationValid={mcpConfigurationValid}
                />
                <div
                  class="resource-add-content"
                  class:type-selection-content={resourceAddStep === 1}
                >
                  <ResourceAddWorkflowHeader
                    activeMessage={activeMessage}
                    activeMessageTone={activeMessageTone}
                    editingProviderResourceId={editingProviderResourceId}
                    editingResourceId={editingResourceId}
                    bind:resourceAddStep={resourceAddStep}
                    resourceKind={resourceKind}
                    busy={busy}
                    selectedScopeId={selectedScopeId}
                    resourceAddStepTitle={resourceAddStepTitle}
                    resourceAddStepValidationMessage={resourceAddStepValidationMessage}
                    onContinueResourceAdd={continueResourceAdd}
                    onContinueProviderAdd={continueProviderAdd}
                    onPrevious={(step) => (resourceAddStep = step)}
                    onContinueMCP={() => {
                      mcpConfigurationAttempted = true;
                      if (mcpConfigurationValid()) {
                        mcpConfigurationAttempted = false;
                        resourceAddStep = 3;
                      }
                    }}
                    onSubmitMCP={() => void (editingResourceId ? updateMCPFromWorkflow() : createResource())}
                  />
                  {#if resourceAddStep === 1}
                    <ResourceBasicConfiguration
                      resourceCategoryOptions={resourceCategoryOptions}
                      resourceAddSubtypeOptions={resourceAddSubtypeOptions}
                      resourceTypeSelectionAttempted={resourceTypeSelectionAttempted}
                      resourceBasicConfigurationAttempted={resourceBasicConfigurationAttempted}
                      bind:resourceAddCategory={resourceAddCategory}
                      bind:resourceAddSubtype={resourceAddSubtype}
                      bind:resourceName={resourceName}
                      bind:resourceStatus={resourceStatus}
                      bind:resourceLabels={resourceLabels}
                      editingProviderResourceId={editingProviderResourceId}
                      editingResourceId={editingResourceId}
                      activeScopeSummary={activeScopeSummary}
                      onSelectCategory={selectResourceAddCategory}
                      onSelectSubtype={(subtype) => {
                        resourceAddSubtype = subtype;
                        const schema = resourceSchemaForSelection(resourceAddCategory, subtype);
                        resourceKind = resourceAddCategory === 'LLM' && subtype === 'Provider' ? 'AIProvider' : (schema?.kind ?? '');
                      }}
                    />
                  {:else if resourceKind === 'MCPServer' && resourceAddStep === 2}
                    <MCPResourceConfiguration
                      bind:mcpURL={mcpURL}
                      bind:mcpToken={mcpToken}
                      bind:mcpTimeoutSeconds={mcpTimeoutSeconds}
                      bind:mcpMaxResponseBytes={mcpMaxResponseBytes}
                      bind:mcpRequestHeaders={mcpRequestHeaders}
                      bind:mcpToolAllowlist={mcpToolAllowlist}
                      onSubmit={createResource}
                    />
                  {:else if resourceKind === 'MCPServer' && resourceAddStep === 3}
                    <MCPResourceSummary
                      mcpTransport={mcpTransport}
                      mcpURL={mcpURL}
                      mcpToken={mcpToken}
                      mcpToolAllowlist={mcpToolAllowlist}
                      mcpTimeoutSeconds={mcpTimeoutSeconds}
                      mcpMaxResponseBytes={mcpMaxResponseBytes}
                      mcpDraftTest={mcpDraftTest}
                      mcpDraftTestBusy={mcpDraftTestBusy}
                      mcpHeaderCount={mcpHeaderCount}
                      onTestConnection={() => void testMCPDraftConnection()}
                    />
                  {:else if resourceKind === 'AIProvider' && resourceAddStep === 2}
                    <ProviderResourceConfiguration
                      providerTypeOptions={providerTypeOptions}
                      providerPurposeOptions={providerPurposeOptions}
                      providerConfigurationAttempted={providerConfigurationAttempted}
                      bind:providerType={providerType}
                      bind:providerProtocol={providerProtocol}
                      bind:providerPurposeTags={providerPurposeTags}
                      bind:providerBaseURL={providerBaseURL}
                      bind:providerAPIKey={providerAPIKey}
                      bind:providerAPIKeyVisible={providerAPIKeyVisible}
                      providerAPIKeyLoading={providerAPIKeyLoading}
                      bind:providerTimeoutSeconds={providerTimeoutSeconds}
                      bind:providerMaxConcurrency={providerMaxConcurrency}
                      bind:providerRateLimitPerMinute={providerRateLimitPerMinute}
                      providerBaseURLValid={providerBaseURLValid}
                      onSelectProviderType={selectProviderType}
                      onTogglePurpose={toggleProviderPurpose}
                    />
                  {:else if resourceKind === 'AIProvider' && resourceAddStep === 3}
                    <ProviderModelConfiguration
                      providerModelConfigurationAttempted={providerModelConfigurationAttempted}
                      bind:providerModelDraft={providerModelDraft}
                      providerModels={providerModels}
                      editingProviderModelName={editingProviderModelName}
                      providerDefaultModel={providerDefaultModel}
                      providerCapabilityOptions={providerCapabilityOptions}
                      toggleProviderModelCapability={toggleProviderModelCapability}
                      addProviderModel={addProviderModel}
                      setProviderDefaultModel={setProviderDefaultModel}
                      setProviderModelEnabled={setProviderModelEnabled}
                      editProviderModel={editProviderModel}
                      removeProviderModel={removeProviderModel}
                    />
                  {:else if resourceKind === 'AIProvider' && resourceAddStep === 4}
                    <ProviderResourceSummary
                      resourceName={resourceName}
                      providerTypeOptions={providerTypeOptions}
                      providerType={providerType}
                      resourceStatus={resourceStatus}
                      providerBaseURL={providerBaseURL}
                      providerProtocol={providerProtocol}
                      providerTimeoutSeconds={providerTimeoutSeconds}
                      providerMaxConcurrency={providerMaxConcurrency}
                      providerDefaultModel={providerDefaultModel}
                      providerModels={providerModels}
                      providerDraftTestBusy={providerDraftTestBusy}
                      providerDraftTestPassedState={providerDraftTestPassedState}
                      providerDraftTest={providerDraftTest}
                      resourceLabelsConfigured={Boolean(resourceLabelsText({ labels: parseLabels(resourceLabels) } as Resource))}
                      activeScopeSummary={activeScopeSummary}
                      providerPurposeTags={providerPurposeTags}
                      providerPurposeLabel={providerPurposeLabel}
                      providerCapabilityOptions={providerCapabilityOptions}
                      onTestConnection={() => void testProviderDraftConnection()}
                      onSubmit={submitProviderCreate}
                    />
                  {:else if selectedResource?.kind === 'MCPServer'}
                    <MCPEditor
                      bind:editResourceName={editResourceName}
                      bind:editResourceStatus={editResourceStatus}
                      bind:editResourceLabels={editResourceLabels}
                      bind:mcpURL={mcpURL}
                      bind:mcpTimeoutSeconds={mcpTimeoutSeconds}
                      bind:mcpMaxResponseBytes={mcpMaxResponseBytes}
                      bind:mcpToken={mcpToken}
                      bind:mcpRequestHeaders={mcpRequestHeaders}
                      bind:mcpToolAllowlist={mcpToolAllowlist}
                    />
                  {:else}
                    <ResourceSchemaConfiguration
                      resourceAddCategory={resourceAddCategory}
                      resourceAddSubtype={resourceAddSubtype}
                      createSchema={createSchema}
                      bind:resourceConfigValues={resourceConfigValues}
                      bind:resourceSensitiveValues={resourceSensitiveValues}
                      bind:resourceConfig={resourceConfig}
                      onSubmit={createResource}
                    />
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
                        <small>{resourceCategoryFor(selectedResource)} · {resourceSubtypeFor(selectedResource)}</small>
                      {/if}
                    </h2>
                    <p class="resource-editor-name">{selectedResource.name}</p>
                    <p class="muted">
                      {selectedResource.kind} · {scopeName(
                        selectedResource.scope_id
                      )}
                    </p>
                  </div>
                  <button
                    class="danger-button"
                    on:click={deleteSelectedResource}
                    disabled={busy || !selectedResourceCanDelete}
                    title={selectedResourceCanDelete
                      ? '停用资源'
                      : '继承资源仅可查看'}>停用</button
                  >
                </div>
                <div class="detail-meta">
                  <span>状态 <strong>{selectedResource.status}</strong></span
                  ><span>Schema v{selectedResource.schema_version}</span><span
                    >更新于 {formatDate(selectedResource.updated_at)}</span
                  >
                </div>
                {#if selectedResourceHasConnector && selectedResource.kind !== 'AIProvider'}
                  <ResourceConnectionStatus
                    connectionCheck={connectionCheck}
                    busy={busy}
                    connectionBusy={connectionBusy}
                    formatDate={formatDate}
                    capabilityName={capabilityName}
                    onTestConnection={testSelectedResourceConnection}
                  />
                {/if}
                <form
                  class="stack-form editor-form"
                  on:submit|preventDefault={updateSelectedResource}
                >
                  {#if selectedResource.kind === 'AIProvider'}
                    <div class="provider-edit-form">
                      <div class="provider-config-form">
                        <label><span>Provider类型</span><select bind:value={providerType}>{#each providerTypeOptions as option}<option value={option.value}>{option.label}</option>{/each}</select></label>
                        <label><span>Provider协议</span><select bind:value={providerProtocol}><option value="chat_completions">Chat Completions</option></select></label>
                        <div class="provider-purpose-options provider-config-purpose"><span>Provider角色</span><div>{#each providerPurposeOptions as purpose}<button class:active={providerPurposeTags.includes(purpose.value)} type="button" aria-pressed={providerPurposeTags.includes(purpose.value)} on:click={() => toggleProviderPurpose(purpose.value)}>{purpose.label}</button>{/each}</div></div>
                        <label><span>服务地址</span><input bind:value={providerBaseURL} type="url" required /></label>
                        <label><span>请求超时（秒）</span><input bind:value={providerTimeoutSeconds} type="number" min="1" /></label>
                        <label><span>最大并发</span><input bind:value={providerMaxConcurrency} type="number" min="1" /></label>
                        <label><span>限流（请求/分钟）</span><input bind:value={providerRateLimitPerMinute} type="number" min="0" /></label>
                      </div>
                      <div class="provider-model-editor">
                        <div class="provider-model-grid">
                          <label><span>Model 名称</span><input bind:value={providerModelDraft.name} /></label>
                          <label><span>上下文窗口</span><input bind:value={providerModelDraft.contextWindowTokens} type="number" min="1" /></label>
                          <label><span>最大输出 Token</span><input bind:value={providerModelDraft.maxOutputTokens} type="number" min="1" /></label>
                          <label><span>优先级</span><input bind:value={providerModelDraft.priority} type="number" min="0" /></label>
                          <label><span>温度</span><span class="provider-temperature-control"><input bind:value={providerModelDraft.temperature} type="number" min="0" max="2" step="0.1" /><span class="provider-temperature-toggle" data-tooltip="允许调用时调整温度参数"><input type="checkbox" bind:checked={providerModelDraft.temperatureMutable} aria-label="温度可调" /><i aria-hidden="true"></i></span></span></label>
                        </div>
                        <div class="provider-model-capabilities-row"><div class="provider-model-flags"><span>支持能力</span>{#each providerCapabilityOptions as capability}<button class:active={providerModelDraft.capabilities.includes(capability.value)} type="button" on:click={() => toggleProviderModelCapability(capability.value)}>{capability.label}</button>{/each}</div><button class="secondary" type="button" on:click={addProviderModel}>{editingProviderModelName ? '保存修改' : '添加模型'}</button></div>
                        <div class="provider-model-list">{#each providerModels as model}<div class="provider-model-row"><strong>{model.name}</strong><span>{model.contextWindowTokens.toLocaleString()} Token · 温度 {model.temperature}</span><span>{providerModelCapabilities(model).join('、')}</span><label class="provider-model-default"><input type="radio" name="provider-default-model-edit" value={model.name} checked={providerDefaultModel === model.name} on:change={() => setProviderDefaultModel(model.name)} /> 默认</label><button class="icon-button" type="button" aria-label={`编辑 ${model.name}`} on:click={() => editProviderModel(model)}><Pencil size={14} /></button><button class="icon-button danger-action" type="button" aria-label={`删除 ${model.name}`} on:click={() => removeProviderModel(model.name)}><Trash2 size={14} /></button></div>{:else}<div class="empty-state">尚未添加模型。</div>{/each}</div>
                      </div>
                    </div>
                  {:else}
                  <h3 class="editor-section-title">基础配置</h3>
                  <p class="editor-section-description">资源类型和子类型只读，资源名称、启用状态与资源标签可在此调整。</p>
                  <div class="resource-basic-edit-grid">
                    <div class="resource-basic-type-row">
                    <label><span>资源类型</span><select value={resourceCategoryFor(selectedResource)} disabled aria-label="资源类型">
                      {#each Object.keys(resourceCategoryOptions).filter((category) => category !== '全部') as category}<option value={category}>{category}</option>{/each}
                    </select></label>
                    <label><span>资源子类型</span><select value={resourceSubtypeFor(selectedResource)} disabled aria-label="资源子类型">
                      {#each resourceSubtypeOptionsFor(selectedResource) as subtype}<option value={subtype}>{subtype}</option>{/each}
                    </select></label>
                    </div>
                    <div class="resource-basic-identity-row">
                    <label><span><i>*</i>资源名称</span><input bind:value={editResourceName} required /></label>
                    <label><span>资源级别</span><input value={scopeName(selectedResource.scope_id)} readonly /></label>
                    <label class="resource-basic-enabled"><span>是否启用</span><span class="provider-toggle-control"><input type="checkbox" checked={editResourceStatus === 'active'} on:change={(event) => (editResourceStatus = (event.currentTarget as HTMLInputElement).checked ? 'active' : 'disabled')} aria-label="是否启用资源" /><i aria-hidden="true"></i></span></label>
                    </div>
                    <label class="resource-basic-labels"><span>资源标签</span><input bind:value={editResourceLabels} placeholder="填写 key=value，多个标签用逗号分隔，例如 env=prod, owner=platform" autocomplete="off" /></label>
                  </div>
                  {#if selectedResource.kind === 'MCPServer'}
                    <div class="mcp-resource-form editor-mcp-form">
                      <label class="mcp-url-field"><span><i>*</i>Server 地址</span><input bind:value={mcpURL} type="url" placeholder="https://mcp.example.com/mcp" /></label>
                      <div class="mcp-number-grid"><label><span>超时时间（秒）</span><input bind:value={mcpTimeoutSeconds} type="number" min="1" max="600" /></label><label><span>响应体大小限制（字节）</span><input bind:value={mcpMaxResponseBytes} type="number" min="1" max="16777216" step="1024" /></label></div>
                      <label><span>Token</span><input bind:value={mcpToken} type="password" placeholder="留空保持原凭据" /></label>
                      <label><span>请求 Header</span><textarea bind:value={mcpRequestHeaders} rows="3" placeholder="每行一个 Header，例如 X-Tenant: production"></textarea></label>
                      <label class="mcp-tools-field"><span>工具白名单</span><textarea bind:value={mcpToolAllowlist} rows="6" placeholder="支持通配符，例如 docker:*&#10;为空表示允许全部工具"></textarea></label>
                    </div>
                  {:else if selectedSchema?.schema.properties}
                    <div class="schema-inputs">
                      <p class="eyebrow">SCHEMA FIELDS</p>
                      {#each Object.entries(selectedSchema.schema.properties) as [key, field]}
                        <label
                          ><span>{#if resourceSchemaFieldRequired(key)}<i>*</i>{/if}{field.title || key}</span>{#if field.sensitive}<input
                              type="password"
                              bind:value={editResourceSensitiveValues[key]}
                              placeholder={selectedResource.credential_id
                                ? '已有关联凭据，留空保持不变'
                                : '敏感信息将加密保存'}
                              autocomplete="new-password"
                            />{:else if field.enum}<select
                              bind:value={resourceConfigValues[key]}
                              ><option value="">未设置</option
                              >{#each field.enum as option}<option
                                  value={option}>{option}</option
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
                {#if selectedSchema?.schema.properties}<div
                    class="schema-fields"
                  >
                    <p class="eyebrow">SCHEMA FIELDS</p>
                    {#each Object.entries(selectedSchema.schema.properties) as [key, field]}<div
                      >
                        <span>{field.title || key}</span><code
                          >{field.sensitive
                            ? selectedResource.credential_id
                              ? '已由加密凭据保存'
                              : '未设置'
                            : String(
                                selectedResource.config[key] ?? '未设置'
                              )}</code
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
                    <span
                      >{relations.length} 条关系 · {topology.length} 个节点</span
                    >
                  </div>
                  <form
                    class="relation-form"
                    on:submit|preventDefault={createRelation}
                  >
                    <select bind:value={relationTarget} required
                      ><option value="" disabled>选择目标资源</option
                      >{#each resources.filter((item) => item.id !== selectedResource.id) as resource}<option
                          value={resource.id}
                          >{resource.name} · {resource.kind}</option
                        >{/each}</select
                    ><select bind:value={relationType}
                      ><option value="depends_on">depends_on</option><option
                        value="contains">contains</option
                      ><option value="deployed_on">deployed_on</option><option
                        value="exposes">exposes</option
                      ><option value="uses_provider">uses_provider</option
                      ></select
                    ><button class="secondary" disabled={busy}>建立关系</button>
                  </form>
                  {#if relations.length}<div class="relation-list">
                      {#each relations as relation}<div class="relation-row">
                          <span
                            ><strong>{relation.relation_type}</strong><small
                              >{resources.find(
                                (item) =>
                                  item.id ===
                                  (relation.source_resource_id ===
                                  selectedResource.id
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
