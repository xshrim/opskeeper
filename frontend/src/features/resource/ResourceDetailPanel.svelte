<script lang="ts">
  import ProviderConnectionStep from './ProviderConnectionStep.svelte';
  import ProviderModelStep from './ProviderModelStep.svelte';
  import McpConnectionFields from './McpConnectionFields.svelte';
  import ResourceBasicEditFields from './ResourceBasicEditFields.svelte';
  import ResourceSchemaFields from './ResourceSchemaFields.svelte';
  import { resourceSupportsEndpointTimeout } from '../../lib/resources';
  import ResourceConnectionStatus from './ResourceConnectionStatus.svelte';
  import ResourceConfigPreview from './ResourceConfigPreview.svelte';
  import ResourceRelationsSection from './ResourceRelationsSection.svelte';
  import type { ConnectionCheck, ConnectorCapability, Relation, Resource, ResourceSchema, TopologyNode } from '../../lib/api';
  import type { ProviderModel } from './resourceWorkflow';

  export let selectedResource: Resource | null = null;
  export let selectedSchema: ResourceSchema | null = null;
  export let resourceEditorOpen = false;
  export let selectedResourceCanDelete = false;
  export let selectedResourceHasConnector = false;
  export let selectedResourceCanUpdate = false;
  export let busy = false;
  export let resourceActionBusy = false;
  export let connectionBusy = false;
  export let connectionCheck: ConnectionCheck | null = null;
  export let relations: Relation[] = [];
  export let topology: TopologyNode[] = [];
  export let resources: Resource[] = [];
  export let relationTarget = '';
  export let relationType = 'depends_on';
  export let relationBusy = false;
  export let providerType = 'openai_compatible';
  export let providerProtocol = 'chat_completions';
  export let providerBaseURL = '';
  export let providerTimeoutSeconds = 60;
  export let providerMaxConcurrency = 5;
  export let providerRateLimitPerMinute = 0;
  export let providerPurposeTags: string[] = [];
  export let providerModels: ProviderModel[] = [];
  export let providerModelDraft: ProviderModel;
  export let editingProviderModelName = '';
  export let providerDefaultModel = '';
  export let providerTypeOptions: Array<{ value: string; label: string; baseURL: string }> = [];
  export let providerPurposeOptions: Array<{ value: string; label: string; requiredCapabilities?: string[] }> = [];
  export let providerCapabilityOptions: Array<{ value: string; label: string }> = [];
  export let mcpURL = '';
  export let mcpToken = '';
  export let mcpRequestHeaders = '';
  export let mcpToolAllowlist = '';
  export let mcpTimeoutSeconds = 120;
  export let mcpMaxResponseBytes = 4 * 1024 * 1024;
  export let mcpTLSCA = '';
  export let mcpTLSCert = '';
  export let mcpTLSKey = '';
  export let mcpTLSSkipVerify = false;
  export let editResourceName = '';
  export let editResourceStatus = 'active';
  export let editResourceLabels = '';
  export let editResourceConfig = '{}';
  export let resourceConfigValues: Record<string, string> = {};
  export let editResourceSensitiveValues: Record<string, string> = {};
  export let genericTimeoutSeconds = 60;
  export let capabilityName: (capability: ConnectorCapability) => string;
  export let formatDate: (value: string) => string;
  export let scopeName: (id: string) => string;
  export let resourceCategoryFor: (resource: Resource) => string;
  export let resourceSubtypeFor: (resource: Resource) => string;
  export let resourceSchemaFieldRequired: (key: string) => boolean;
  export let onDelete: () => void = () => {};
  export let onTestConnection: () => void = () => {};
  export let onUpdate: () => void = () => {};
  export let onTogglePurpose: (purpose: string) => void = () => {};
  export let onToggleCapability: (capability: string) => void = () => {};
  export let onAddModel: () => void = () => {};
  export let onSetDefault: (name: string) => void = () => {};
  export let onEditModel: (model: ProviderModel) => void = () => {};
  export let onRemoveModel: (name: string) => void = () => {};
  export let onCreateRelation: () => void = () => {};
  export let onDeleteRelation: (relation: Relation) => void = () => {};
</script>

{#if selectedResource}
  <section class="panel detail-panel" class:open={resourceEditorOpen}>
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
        <p class="muted">{selectedResource.kind} · {scopeName(selectedResource.scope_id)}</p>
      </div>
      <button class="danger-button" on:click={onDelete} disabled={busy || resourceActionBusy || !selectedResourceCanDelete} title={selectedResourceCanDelete ? '停用资源' : '继承资源仅可查看'}>停用</button>
    </div>
    <div class="detail-meta">
      <span>状态 <strong>{selectedResource.status}</strong></span>
      <span>Schema v{selectedResource.schema_version}</span>
      <span>更新于 {formatDate(selectedResource.updated_at)}</span>
    </div>
    {#if selectedResourceHasConnector && selectedResource.kind !== 'AIProvider'}
      <ResourceConnectionStatus check={connectionCheck} {busy} {connectionBusy} {formatDate} {capabilityName} onTest={onTestConnection} />
    {/if}
    <form class="stack-form editor-form" on:submit|preventDefault={onUpdate}>
      {#if selectedResource.kind === 'AIProvider'}
        <div class="provider-edit-form">
          <ProviderConnectionStep bind:type={providerType} bind:protocol={providerProtocol} bind:baseURL={providerBaseURL} bind:timeoutSeconds={providerTimeoutSeconds} bind:maxConcurrency={providerMaxConcurrency} bind:rateLimitPerMinute={providerRateLimitPerMinute} bind:purposeTags={providerPurposeTags} typeOptions={providerTypeOptions} purposeOptions={providerPurposeOptions} showAPIKey={false} onTogglePurpose={onTogglePurpose} />
          <ProviderModelStep bind:draft={providerModelDraft} bind:models={providerModels} bind:defaultModel={providerDefaultModel} capabilityOptions={providerCapabilityOptions} editingModelName={editingProviderModelName} radioName="provider-default-model-edit" showEnabledControl={false} onToggleCapability={onToggleCapability} onAddModel={onAddModel} onSetDefault={onSetDefault} onEditModel={onEditModel} onRemoveModel={onRemoveModel} />
        </div>
      {:else}
        <ResourceBasicEditFields resource={selectedResource} bind:name={editResourceName} bind:status={editResourceStatus} bind:labels={editResourceLabels} {scopeName} />
        {#if selectedResource.kind === 'MCPServer'}
          <div class="mcp-resource-form editor-mcp-form">
            <McpConnectionFields bind:url={mcpURL} bind:token={mcpToken} bind:requestHeaders={mcpRequestHeaders} bind:toolAllowlist={mcpToolAllowlist} bind:timeoutSeconds={mcpTimeoutSeconds} bind:maxResponseBytes={mcpMaxResponseBytes} bind:tlsCA={mcpTLSCA} bind:tlsCert={mcpTLSCert} bind:tlsKey={mcpTLSKey} bind:skipTLSVerify={mcpTLSSkipVerify} tokenPlaceholder="留空保持原凭据" />
          </div>
        {:else}
          <ResourceSchemaFields schema={selectedSchema} bind:values={resourceConfigValues} bind:sensitiveValues={editResourceSensitiveValues} bind:rawConfig={editResourceConfig} bind:timeoutSeconds={genericTimeoutSeconds} showTimeout={resourceSupportsEndpointTimeout(selectedResource.kind)} editMode credentialConfigured={Boolean(selectedResource.credential_id)} isRequired={resourceSchemaFieldRequired} />
        {/if}
      {/if}
      <button class="secondary" disabled={busy || !selectedResourceCanUpdate} title={selectedResourceCanUpdate ? '保存资源修改' : '继承资源仅可查看'}>保存</button>
    </form>
    <ResourceConfigPreview resource={selectedResource} schema={selectedSchema} />
    <ResourceRelationsSection resource={selectedResource} {resources} {relations} {topology} bind:target={relationTarget} bind:relationType {busy} {relationBusy} onCreate={onCreateRelation} onDelete={onDeleteRelation} />
  </section>
{:else}
  <section class="panel empty-detail">
    <div class="empty-state"><span class="empty-icon">◇</span><h2>选择一个资源</h2><p>从左侧目录选择资源查看作用域、配置和关系。</p></div>
  </section>
{/if}
