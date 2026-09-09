<script lang="ts">
  import { onMount } from 'svelte';
  import { Plus, RefreshCw } from 'lucide-svelte';
  import ResourceCatalogRail from './ResourceCatalogRail.svelte';
  import ResourceCatalogList from './ResourceCatalogList.svelte';
  import ResourceCatalogDetails from './ResourceCatalogDetails.svelte';
  import ResourceWorkflowPanel from './ResourceWorkflowPanel.svelte';
  import ResourceBasicConfigStep from './ResourceBasicConfigStep.svelte';
  import McpConnectionFields from './McpConnectionFields.svelte';
  import ProviderConnectionStep from './ProviderConnectionStep.svelte';
  import ProviderModelStep from './ProviderModelStep.svelte';
  import ProviderReviewStep from './ProviderReviewStep.svelte';
  import McpReviewStep from './McpReviewStep.svelte';
  import DockerConnectionStep from './DockerConnectionStep.svelte';
  import DockerReviewStep from './DockerReviewStep.svelte';
  import KubernetesConnectionStep from './KubernetesConnectionStep.svelte';
  import KubernetesReviewStep from './KubernetesReviewStep.svelte';
  import ResourceSchemaFields from './ResourceSchemaFields.svelte';
  import ResourceDetailPanel from './ResourceDetailPanel.svelte';
  import { resourceHasConnector, resourceSupportsEndpointTimeout } from '../../lib/resources';
  import {
    createResourceRelation,
    removeResourceRelation,
    removeResource,
    setResourceEnabled,
    testResourceConnector,
    createMCPCredential as createMCPCredentialAction,
    createDockerCredential as createDockerCredentialAction,
    createProviderCredential as createProviderCredentialAction,
    createSchemaCredential,
    saveMCPCredential as saveMCPCredentialAction,
    saveDockerCredential as saveDockerCredentialAction,
    createKubernetesCredential as createKubernetesCredentialAction,
    saveKubernetesCredential as saveKubernetesCredentialAction,
    saveProviderCredential as saveProviderCredentialAction,
    testDraftAIProviderConnection,
    testDraftMCPConnection,
    loadMCPSnapshots as loadMCPSnapshotsAction,
    loadResourceCredentialSecret,
    syncAIProviderBindings,
    createResourceRecord,
    updateResourceRecord
  } from './resourceActions';
  import {
    loadResourceConnectionCheck,
    loadResourceRelations,
    prependMCPSnapshot
  } from './resourceData';
  import {
    resourceCategoryFor,
    resourceCategoryIcon,
    resourceCategoryOptions,
    resourceEndpointFor,
    resourceLabelsText,
    resourceSchemaForSelection as findResourceSchemaForSelection,
    resourceSubtypeFor,
    connectorCapabilityName
  } from './resourceCatalog';
  import {
    buildResourceSchemaConfig as buildSchemaConfig,
    mcpConfigForSave as buildMCPConfig,
    mcpConfigurationValid as validateMCPConfiguration,
    mcpTransportForSubtype,
    parseMCPHeaders,
    parseResourceLabels as parseLabels,
    providerPurposeMissingCapabilities as missingProviderCapabilities,
    providerTypeLabel as getProviderTypeLabel,
    providerModelsForResource,
    providerDefaultModelForResource,
    providerModelCapabilities as getProviderModelCapabilities,
    providerPurposeLabel,
    providerBaseURLValid as isProviderBaseURLValid,
    providerConfigForCreate as buildProviderConfigForCreate,
    providerTypeOptions,
    providerCapabilityOptions,
    providerPurposeOptions,
    emptyProviderModelDraft,
    dockerConfigForSave,
    dockerConnectionConfigurationValid,
    dockerCredentialForSave,
    dockerHostValid,
    dockerHostSupportsTLS,
    dockerTLSValueValid,
    dockerTLSValueForDisplay,
    type DockerAccessMode,
    type KubernetesConnectionMode,
    kubernetesConfigForSave,
    kubernetesCredentialForSave,
    kubernetesConfigurationValid,
    kubernetesKubeconfigText,
    resourceAddStepTitle,
    resourceAddStepDescription,
    type ProviderModel
  } from './resourceWorkflow';
  import { api, ApiError } from '../../lib/api';
  import type {
    ConnectionCheck,
    MCPSnapshot,
    Relation,
    Resource,
    ResourceSchema,
    TopologyNode
  } from '../../lib/api';

  type AIProviderBindingSummary = {
    scope_id: string;
    provider_resource_id: string;
    tag: string;
  };

  export let visibleResources: Resource[] = [];
  export let selectedResourceId = '';
  export let resourceConnectionChecks: Record<string, ConnectionCheck | null> =
    {};
  export let operationSnapshots: Record<string, MCPSnapshot[]> = {};
  export let busy = false;
  export let connectionBusy = false;
  export let resourceCanManage: (
    resource: Resource,
    permission: string
  ) => boolean;
  export let resourcePermissionLabel: (resource: Resource) => string = () => '可查看';
  export let onSelectResourceScope: (scopeId: string) => void = () => {};
  export let onWorkspaceReload: () => void | Promise<void> = () => {};
  export let scopeType: (id: string) => string;
  export let formatDate: (value: string) => string;
  export let resourceIcon: (kind: string) => string;
  export let selectedResource: Resource | null = null;
  export let selectedResourceCanDelete = false;
  export let selectedResourceHasConnector = false;
  export let selectedScopeId = '';
  export let aiProviderBindings: AIProviderBindingSummary[] = [];
  export let selectedResourceCanUpdate = false;
  export let scopeName: (id: string) => string;
  export let onNotice: (message: string) => void = () => {};
  export let onError: (message: string) => void = () => {};
  export let resources: Resource[] = [];
  export let schemas: ResourceSchema[] = [];
  export let childSurfaceActive = false;
  let connectionDetailResourceId = '';
  let connectionBusyResourceIds: string[] = [];
  let autoSummaryTestKey = '';
  let resourceAddMenuOpen = false;
  let resourceRefreshBusy = false;
  let resourceEditorOpen = false;
  let resourceKind = '';
  let resourceAddStep = 1;
  let resourceAddCategory = '';
  let resourceAddSubtype = '';
  let resourceName = '';
  let resourceStatus = 'active';
  let resourceLabels = '';
  let resourceConfig = '{}';
  let resourceConfigValues: Record<string, string> = {};
  let resourceSensitiveValues: Record<string, string> = {};
  let editResourceName = '';
  let editResourceStatus = 'active';
  let editResourceLabels = '';
  let editResourceConfig = '{}';
  let editResourceSensitiveValues: Record<string, string> = {};
  let genericTimeoutSeconds = 60;
  let editingProviderResourceId = '';
  let editingResourceId = '';
  let editingDockerResourceId = '';
  let editingKubernetesResourceId = '';
  let providerType = 'openai_compatible';
  let providerProtocol = 'chat_completions';
  let providerBaseURL = '';
  let providerAPIKey = '';
  let providerAPIKeyVisible = false;
  let providerAPIKeyLoading = false;
  let providerTimeoutSeconds = 60;
  let providerMaxConcurrency = 5;
  let providerRateLimitPerMinute = 0;
  let providerModels: ProviderModel[] = [];
  let providerModelDraft: ProviderModel = emptyProviderModelDraft();
  let editingProviderModelName = '';
  let providerDefaultModel = '';
  let providerPurposeTags: string[] = [];
  let mcpTransport = 'streamable_http';
  let mcpURL = '';
  let mcpToken = '';
  let mcpRequestHeaders = '';
  let mcpToolAllowlist = '';
  let mcpTimeoutSeconds = 120;
  let mcpMaxResponseBytes = 4 * 1024 * 1024;
  let mcpDraftTest: any = null;
  let mcpDraftTestBusy = false;
  let mcpConfigurationAttempted = false;
  let providerConfigurationAttempted = false;
  let providerModelConfigurationAttempted = false;
  let providerModelValidationMessage = '';
  let providerDraftTest: any = null;
  let providerDraftTestBusy = false;
  let providerDraftTestPassedState = false;
  let providerSummaryAttempted = false;
  let resourceTypeSelectionAttempted = false;
  let resourceBasicConfigurationAttempted = false;
  let dockerAccessMode: DockerAccessMode = 'direct';
  let dockerHost = '';
  let dockerTimeoutSeconds = 10;
  let dockerCABase64 = '';
  let dockerCertBase64 = '';
  let dockerKeyBase64 = '';
  let dockerSkipTLSVerify = false;
  let dockerMCPServerResourceId = '';
  let dockerAgentConnectionOverride = false;
  let dockerConfigurationAttempted = false;
  let dockerCredentialLoading = false;
  let dockerDraftTestBusy = false;
  let dockerDraftTest: {
    status?: string;
    message?: string;
    error?: string;
    latency?: number;
    toolCount?: number;
  } | null = null;
  let kubernetesConnectionMode: KubernetesConnectionMode = 'kubeconfig';
  let kubernetesServer = '';
  let kubernetesCABase64 = '';
  let kubernetesToken = '';
  let kubernetesCertBase64 = '';
  let kubernetesKeyBase64 = '';
  let kubernetesKubeconfig = '';
  let kubernetesSkipTLSVerify = false;
  let kubernetesMCPServerResourceId = '';
  let kubernetesConnectionOverride = false;
  let kubernetesConfigurationAttempted = false;
  let kubernetesCredentialLoading = false;
  let kubernetesDraftTestBusy = false;
  let kubernetesDraftTest: any = null;
  export let activeMessage = '';
  export let activeMessageTone: 'success' | 'error' = 'success';
  let selectedSchema: ResourceSchema | null = null;
  let createSchema: ResourceSchema | null = null;
  let resourceAddSubtypeOptions: string[] = [];
  let connectionCheck: ConnectionCheck | null = null;
  let relations: Relation[] = [];
  let topology: TopologyNode[] = [];
  let relationTarget = '';
  let relationType = 'depends_on';

  let relationBusy = false;
  let resourceActionBusy = false;

  const capabilityName = connectorCapabilityName;
  const providerTypeLabel = (type: unknown) => getProviderTypeLabel(type, providerTypeOptions);
  const providerModelCapabilities = (model: Record<string, unknown> | undefined) =>
    getProviderModelCapabilities(model, providerCapabilityOptions);

  $: selectedSchema = schemas.find((schema) => schema.kind === selectedResource?.kind) ?? null;
  $: createSchema = schemas.find((schema) => schema.kind === resourceKind) ?? null;
  $: resourceAddSubtypeOptions = resourceAddCategory
    ? (resourceCategoryOptions[resourceAddCategory] ?? [])
    : [];
  $: dockerMCPServers = resources
    .filter((resource) => resource.kind === 'MCPServer' && (resource.status === 'active' || resource.id === dockerMCPServerResourceId))
    .sort((left, right) => left.name.localeCompare(right.name));
  $: kubernetesMCPServers = dockerMCPServers;
  $: childSurfaceActive = resourceAddMenuOpen || resourceEditorOpen;
  $: if (!resourceAddMenuOpen || !((resourceKind === 'MCPServer' && resourceAddStep === 3)
    || (resourceKind === 'AIProvider' && resourceAddStep === 4)
    || (resourceKind === 'Docker' && resourceAddStep === 3)
    || (resourceKind === 'Kubernetes' && resourceAddStep === 3))) autoSummaryTestKey = '';
  $: if (resourceAddMenuOpen && ((resourceKind === 'MCPServer' && resourceAddStep === 3)
    || (resourceKind === 'AIProvider' && resourceAddStep === 4)
    || (resourceKind === 'Docker' && resourceAddStep === 3)
    || (resourceKind === 'Kubernetes' && resourceAddStep === 3))) {
    const key = `${resourceKind}:${resourceAddStep}:${resourceKind === 'MCPServer'
      ? mcpDraftSignature()
      : resourceKind === 'AIProvider'
        ? providerDraftSignature()
        : resourceKind === 'Docker' ? JSON.stringify(dockerDraft()) : JSON.stringify(kubernetesDraft())}`;
    if (autoSummaryTestKey !== key) {
      autoSummaryTestKey = key;
      void (resourceKind === 'MCPServer'
        ? testMCPDraftConnection()
        : resourceKind === 'AIProvider'
          ? testProviderDraftConnection()
          : resourceKind === 'Docker' ? testDockerDraftConnection() : testKubernetesDraftConnection());
    }
  }

  onMount(() => {
    if (selectedResourceId) void loadResourceDetails(selectedResourceId);
  });

  function resourceKindForSelection(category: string, subtype = '') {
    if (category === 'LLM' && subtype === 'Provider') return 'AIProvider';
    const schema = resourceSchemaForSelection(category, subtype);
    return schema && resourceCategoryFor(schema) === category ? schema.kind : category;
  }

  async function loadResourceDetails(id: string) {
    selectedResourceId = id;
    connectionCheck = null;
    try {
      const details = await loadResourceRelations(id);
      relations = details.relations;
      topology = details.topology;
    } catch (error) {
      onError(describeError(error, '资源关系加载失败'));
    }
    await loadConnectionCheck(id);
  }

  async function loadConnectionCheck(id: string) {
    const current = resources.find((item) => item.id === id);
    if (!current) {
      resourceConnectionChecks = { ...resourceConnectionChecks, [id]: null };
      return;
    }
    try {
      const check = await loadResourceConnectionCheck(current);
      if (check || !Object.prototype.hasOwnProperty.call(resourceConnectionChecks, id)) {
        resourceConnectionChecks = { ...resourceConnectionChecks, [id]: check };
      }
      if (selectedResourceId === id) connectionCheck = check ?? resourceConnectionChecks[id] ?? null;
    } catch (error) {
      if (selectedResourceId === id) onError(describeError(error, '连接状态加载失败'));
    }
  }

  function resourceScopeLabel(resource: Resource) {
    const labels: Record<string, string> = {
      platform: '平台',
      team: '团队',
      project: '项目'
    };
    return labels[scopeType(resource.scope_id)] ?? '资源';
  }

  function mcpServerNameFor(resource: Resource) {
    const id = resource.agent_ref;
    if (!id) return '';
    return resources.find((item) => item.id === id)?.name ?? '关联 MCPServer';
  }

  function resourceSchemaForSelection(category: string, subtype: string) {
    return findResourceSchemaForSelection(schemas, category, subtype);
  }

  function resourceSchemaFieldRequired(key: string) {
    return createSchema?.schema.required?.includes(key) ?? false;
  }

  async function createResourceCredential(
    schema: ResourceSchema | null | undefined,
    values: Record<string, string>
  ) {
    const scopeId = selectedResource?.scope_id ?? selectedScopeId;
    return createSchemaCredential(schema, values, scopeId, resourceName || selectedResource?.name || '');
  }

  function providerBindingsFor(resource: Resource) {
    return aiProviderBindings
      .filter((binding) => binding.provider_resource_id === resource.id)
      .sort((left, right) => {
        const scopeOrder = { platform: 0, team: 1, project: 2 } as Record<string, number>;
        const scopeDifference =
          (scopeOrder[scopeType(left.scope_id)] ?? 9) -
          (scopeOrder[scopeType(right.scope_id)] ?? 9);
        return scopeDifference || left.tag.localeCompare(right.tag);
      });
  }

  function activeScopeSummary() {
    const labels: Record<string, string> = {
      platform: '平台级',
      team: '团队级',
      project: '项目级'
    };
    const type = scopeType(selectedScopeId);
    return `${labels[type] ?? '当前级别'} · ${scopeName(selectedScopeId) || '平台'}`;
  }

  function syncProviderEditor(resource: Resource) {
    resourceName = resource.name;
    resourceStatus = resource.status;
    const serializedLabels = Object.entries(resource.labels ?? {})
      .map(([key, value]) => `${key}=${value}`)
      .join(', ');
    resourceLabels = serializedLabels;
    editResourceLabels = serializedLabels;
    const config = resource.config ?? {};
    providerType = String(config.provider_type ?? 'openai_compatible');
    providerProtocol = String(config.protocol ?? 'chat_completions');
    providerBaseURL = String(config.base_url ?? '');
    providerAPIKey = '';
    providerAPIKeyVisible = false;
    providerTimeoutSeconds = Number(config.timeout_seconds ?? 60);
    providerMaxConcurrency = Number(config.max_concurrency ?? 5);
    providerRateLimitPerMinute = Number(config.rate_limit_per_minute ?? 0);
    const models = Array.isArray(config.models) ? config.models : [];
    providerModels = models.map((item) => {
      const model = item as Record<string, unknown>;
      return {
        name: String(model.name ?? ''),
        contextWindowTokens: Number(model.context_window_tokens ?? model.context_window ?? 128000),
        maxOutputTokens: Number(model.max_output_tokens ?? 128000),
        temperature: Number(model.temperature ?? 0.7),
        temperatureMutable: model.temperature_mutable !== false,
        capabilities: Array.isArray(model.capabilities) ? model.capabilities.map(String) : ['text'],
        enabled: model.enabled !== false,
        priority: Number(model.priority ?? 0)
      };
    });
    providerDefaultModel = String(config.default_model ?? providerModels[0]?.name ?? '');
    providerModelDraft = emptyProviderModelDraft();
    editingProviderModelName = '';
  }

  function syncDockerEditor(resource: Resource) {
    const config = resource.config ?? {};
    resourceName = resource.name;
    resourceStatus = resource.status;
    resourceLabels = Object.entries(resource.labels ?? {})
      .map(([key, value]) => `${key}=${value}`)
      .join(', ');
    editResourceLabels = resourceLabels;
    dockerAccessMode = String(resource.subtype ?? 'direct').toLowerCase() === 'agent' ? 'agent' : 'direct';
    dockerHost = String(config.host ?? '');
    dockerTimeoutSeconds = Number(config.timeout ?? 10);
    dockerSkipTLSVerify = config.skip_tls_verify === true || String(config.skip_tls_verify ?? '').toLowerCase() === 'true';
    dockerMCPServerResourceId = String(resource.agent_ref ?? '');
    dockerAgentConnectionOverride = Object.keys(config).length > 0;
    dockerCABase64 = '';
    dockerCertBase64 = '';
    dockerKeyBase64 = '';
    dockerCredentialLoading = false;
    if ((dockerAccessMode === 'direct' || dockerAgentConnectionOverride) && resource.credential_id) {
      dockerCredentialLoading = true;
      void loadResourceCredentialSecret(resource.credential_id).then((credential) => {
        if (selectedResourceId !== resource.id) return;
        try {
          const secret = JSON.parse(credential.secret) as Record<string, unknown>;
          dockerCABase64 = dockerTLSValueForDisplay(String(secret.tls_ca ?? ''));
          dockerCertBase64 = dockerTLSValueForDisplay(String(secret.tls_cert ?? ''));
          dockerKeyBase64 = dockerTLSValueForDisplay(String(secret.tls_key ?? ''));
        } catch {
          dockerCABase64 = '';
          dockerCertBase64 = '';
          dockerKeyBase64 = '';
        } finally {
          dockerCredentialLoading = false;
        }
      }).catch(() => {
        if (selectedResourceId === resource.id) dockerCredentialLoading = false;
      });
    }
  }

  function syncKubernetesEditor(resource: Resource) {
    const config = resource.config ?? {};
    resourceName = resource.name; resourceStatus = resource.status;
    resourceLabels = Object.entries(resource.labels ?? {}).map(([key, value]) => `${key}=${value}`).join(', '); editResourceLabels = resourceLabels;
    kubernetesConnectionMode = String(config.connection_mode ?? (config.server ? 'endpoint' : 'kubeconfig')) === 'endpoint' ? 'endpoint' : 'kubeconfig';
    kubernetesServer = String(config.server ?? ''); kubernetesCABase64 = ''; kubernetesCertBase64 = ''; kubernetesKeyBase64 = ''; kubernetesSkipTLSVerify = config.skip_tls_verify === true;
    kubernetesMCPServerResourceId = String(resource.agent_ref ?? ''); kubernetesKubeconfig = ''; kubernetesToken = '';
    kubernetesConnectionOverride = Boolean(resource.credential_id) || Object.keys(config).some((key) => key !== 'connection_mode');
    if (resource.credential_id) { kubernetesCredentialLoading = true; void loadResourceCredentialSecret(resource.credential_id).then((credential) => { if (selectedResourceId !== resource.id) return; try { const secret = JSON.parse(credential.secret) as Record<string, unknown>; const encoded = String(secret.kubeconfig_base64 ?? ''); kubernetesKubeconfig = encoded ? kubernetesKubeconfigText(encoded) : String(secret.kubeconfig ?? ''); kubernetesToken = String(secret.token ?? ''); kubernetesCABase64 = dockerTLSValueForDisplay(String(secret.ca_file ?? '')); kubernetesCertBase64 = dockerTLSValueForDisplay(String(secret.client_cert_file ?? '')); kubernetesKeyBase64 = dockerTLSValueForDisplay(String(secret.client_key_file ?? '')); } catch { kubernetesKubeconfig = ''; kubernetesToken = credential.secret; kubernetesCABase64 = ''; kubernetesCertBase64 = ''; kubernetesKeyBase64 = ''; } finally { kubernetesCredentialLoading = false; } }).catch(() => { kubernetesCredentialLoading = false; }); }
  }

  function syncResourceEditor(resource: Resource) {
    editResourceName = resource.name;
    editResourceStatus = resource.status;
    editResourceLabels = Object.entries(resource.labels ?? {})
      .map(([key, value]) => `${key}=${value}`)
      .join(', ');
    editResourceConfig = JSON.stringify(resource.config ?? {}, null, 2);
    resourceConfigValues = Object.fromEntries(
      Object.entries(resource.config ?? {}).map(([key, value]) => [key, String(value)])
    );
    editResourceSensitiveValues = {};
    genericTimeoutSeconds = Number(resource.config?.timeout_seconds ?? 60);
    if (resource.kind === 'MCPServer') {
      const config = resource.config ?? {};
      mcpTransport = mcpTransportForSubtype(resource.subtype || String(config.subtype ?? ''));
      mcpURL = String(config.url ?? '');
      mcpToken = '';
      mcpRequestHeaders = Object.entries((config.request_headers ?? {}) as Record<string, unknown>)
        .map(([key, value]) => `${key}: ${String(value)}`).join('\n');
      mcpToolAllowlist = Array.isArray(config.tool_allowlist)
        ? config.tool_allowlist.map(String).join('\n')
        : String(config.tool_allowlist ?? '');
      mcpTimeoutSeconds = Number(config.timeout_seconds ?? 120);
      mcpMaxResponseBytes = Number(config.max_response_bytes ?? 4 * 1024 * 1024);
      mcpDraftTest = null;
      mcpConfigurationAttempted = false;
      if (resource.credential_id) {
        void loadResourceCredentialSecret(resource.credential_id).then((credential) => {
          if (selectedResourceId !== resource.id) return;
          try {
            const secret = JSON.parse(credential.secret) as { token?: string; headers?: Record<string, unknown> };
            mcpToken = String(secret.token ?? '');
            if (secret.headers) mcpRequestHeaders = Object.entries(secret.headers).map(([key, value]) => `${key}: ${String(value)}`).join('\n');
          } catch {
            mcpToken = credential.secret.trim();
          }
        }).catch(() => undefined);
      }
    }
    if (resource.kind === 'AIProvider') syncProviderEditor(resource);
  }

  function openProviderWorkflowForEdit(resource: Resource) {
    onSelectResourceScope(resource.scope_id);
    selectedScopeId = resource.scope_id;
    selectedResourceId = resource.id;
    resourceKind = 'AIProvider';
    resourceAddCategory = 'LLM';
    resourceAddSubtype = 'Provider';
    editingProviderResourceId = resource.id;
    editingResourceId = '';
    syncProviderEditor(resource);
    providerPurposeTags = aiProviderBindings
      .filter((binding) => binding.scope_id === resource.scope_id && binding.provider_resource_id === resource.id)
      .map((binding) => binding.tag);
    providerDraftTest = null;
    resourceAddStep = 1;
    resourceBasicConfigurationAttempted = false;
    resourceEditorOpen = false;
    resourceAddMenuOpen = true;
    if (resource.credential_id) {
      providerAPIKeyLoading = true;
      void loadResourceCredentialSecret(resource.credential_id).then((credential) => {
        if (editingProviderResourceId === resource.id) {
          providerAPIKey = credential.secret;
          providerAPIKeyLoading = false;
        }
      }, (error) => {
        if (editingProviderResourceId === resource.id) {
          providerAPIKey = '';
          providerAPIKeyLoading = false;
          onError(describeError(error, '无法读取 Provider API Key'));
        }
      });
    } else {
      providerAPIKeyLoading = false;
    }
  }

  function openMCPWorkflowForEdit(resource: Resource) {
    onSelectResourceScope(resource.scope_id);
    selectedScopeId = resource.scope_id;
    selectedResourceId = resource.id;
    resourceKind = 'MCPServer';
    resourceAddCategory = 'MCPServer';
    resourceAddSubtype = resourceSubtypeFor(resource);
    editingProviderResourceId = '';
    editingResourceId = resource.id;
    resourceName = resource.name;
    resourceStatus = resource.status;
    resourceLabels = Object.entries(resource.labels ?? {}).map(([key, value]) => `${key}=${value}`).join(', ');
    syncResourceEditor(resource);
    mcpDraftTest = null;
    mcpConfigurationAttempted = false;
    resourceAddStep = 1;
    resourceTypeSelectionAttempted = false;
    resourceBasicConfigurationAttempted = false;
    resourceEditorOpen = false;
    resourceAddMenuOpen = true;
  }

  function openDockerWorkflowForEdit(resource: Resource) {
    onSelectResourceScope(resource.scope_id);
    selectedScopeId = resource.scope_id;
    selectedResourceId = resource.id;
    resourceKind = 'Docker';
    resourceAddCategory = 'Docker';
    resourceAddSubtype = resourceSubtypeFor(resource);
    editingProviderResourceId = '';
    editingResourceId = '';
    editingDockerResourceId = resource.id;
    syncDockerEditor(resource);
    resourceAddStep = 1;
    resourceTypeSelectionAttempted = false;
    resourceBasicConfigurationAttempted = false;
    dockerConfigurationAttempted = false;
    resourceEditorOpen = false;
    resourceAddMenuOpen = true;
  }

  function openResourceEditor(resource: Resource) {
    if (resource.kind === 'AIProvider') {
      openProviderWorkflowForEdit(resource);
      return;
    }
    if (resource.kind === 'MCPServer') {
      openMCPWorkflowForEdit(resource);
      return;
    }
    if (resource.kind === 'Docker') {
      openDockerWorkflowForEdit(resource);
      return;
    }
    if (resource.kind === 'Kubernetes') { openKubernetesWorkflowForEdit(resource); return; }
    selectedResourceId = resource.id;
    syncResourceEditor(resource);
    resourceEditorOpen = true;
    void loadResourceDetails(resource.id);
  }

  function openKubernetesWorkflowForEdit(resource: Resource) { onSelectResourceScope(resource.scope_id); selectedScopeId = resource.scope_id; selectedResourceId = resource.id; resourceKind = 'Kubernetes'; resourceAddCategory = 'Kubernetes'; resourceAddSubtype = resourceSubtypeFor(resource); editingKubernetesResourceId = resource.id; editingResourceId = ''; editingDockerResourceId = ''; syncKubernetesEditor(resource); resourceAddStep = 1; resourceAddMenuOpen = true; resourceEditorOpen = false; }

  function selectResourceAddCategory(category: string) {
    resourceAddCategory = category;
    resourceAddSubtype = '';
    const firstSubtype = resourceCategoryOptions[category]?.[0] ?? '';
    resourceKind = resourceKindForSelection(category, firstSubtype);
    if (resourceKind === 'MCPServer') mcpTransport = mcpTransportForSubtype(firstSubtype);
    resourceAddStep = 1;
    autoSummaryTestKey = '';
  }

  function toggleResourceAddMenu() {
    resourceAddMenuOpen = !resourceAddMenuOpen;
    resourceEditorOpen = false;
    editingProviderResourceId = '';
    editingResourceId = '';
    editingDockerResourceId = '';
    editingKubernetesResourceId = '';
    resourceAddStep = 1;
    resourceTypeSelectionAttempted = false;
    resourceBasicConfigurationAttempted = false;
    if (resourceAddMenuOpen) {
      resetResourceConfig();
      resetProviderDraft();
      resetDockerDraft();
      resetKubernetesDraft();
      resourceName = '';
      resourceLabels = '';
      resourceStatus = 'active';
      resourceAddCategory = resourceCategory === '全部' ? '' : resourceCategory;
      resourceAddSubtype = resourceSubtype === '全部' ? '' : resourceSubtype;
      resourceKind = '';
    if (resourceAddCategory) {
        const firstSubtype = resourceCategoryOptions[resourceAddCategory]?.[0] ?? '';
        resourceKind = resourceKindForSelection(resourceAddCategory, resourceAddSubtype || firstSubtype);
      }
    }
  }

  function chooseResourceAddSubtype(category: string, subtype: string, resetDraft = true) {
    const schema = resourceSchemaForSelection(category, subtype);
    resourceAddCategory = category;
    resourceAddSubtype = subtype;
    resourceKind = category === 'LLM' && subtype === 'Provider' ? 'AIProvider' : (schema?.kind ?? '');
    if (resourceKind === 'MCPServer') mcpTransport = mcpTransportForSubtype(subtype);
    resourceCategory = category;
    resourceSubtype = subtype;
    if (resetDraft) {
      resetResourceConfig();
      resetProviderDraft();
      resetDockerDraft();
      resetKubernetesDraft();
    }
    if (resourceKind === 'Docker') {
      dockerAccessMode = subtype.trim().toLowerCase() === 'agent' ? 'agent' : 'direct';
    }
    resourceAddStep = 1;
    resourceTypeSelectionAttempted = false;
    resourceBasicConfigurationAttempted = false;
    resourceAddMenuOpen = true;
    resourceEditorOpen = false;
  }

  function selectProviderType(type: string) {
    providerType = type;
    const preset = providerTypeOptions.find((item) => item.value === type) as { value: string; label: string; baseURL?: string } | undefined;
    if (preset?.baseURL) providerBaseURL = preset.baseURL;
  }

  function toggleProviderPurpose(purpose: string) {
    providerPurposeTags = providerPurposeTags.includes(purpose)
      ? providerPurposeTags.filter((item) => item !== purpose)
      : [...providerPurposeTags, purpose];
  }

  function toggleProviderModelCapability(capability: string) {
    providerModelDraft = {
      ...providerModelDraft,
      capabilities: providerModelDraft.capabilities.includes(capability)
        ? providerModelDraft.capabilities.filter((item) => item !== capability)
        : [...providerModelDraft.capabilities, capability]
    };
  }

  function setProviderDefaultModel(name: string) {
    providerDefaultModel = name;
    providerModels = providerModels.map((model) => model.name === name ? { ...model, enabled: true } : model);
  }

  function setProviderModelEnabled(name: string, enabled: boolean) {
    if (name === providerDefaultModel && !enabled) return;
    providerModels = providerModels.map((model) => model.name === name ? { ...model, enabled } : model);
  }

  function resetResourceConfig() {
    resourceConfigValues = {};
    resourceSensitiveValues = {};
    resourceConfig = '{}';
    genericTimeoutSeconds = 60;
    mcpTransport = 'streamable_http';
    mcpURL = '';
    mcpToken = '';
    mcpRequestHeaders = '';
    mcpToolAllowlist = '';
    mcpTimeoutSeconds = 120;
    mcpMaxResponseBytes = 4 * 1024 * 1024;
    mcpDraftTest = null;
    mcpConfigurationAttempted = false;
  }

  function resetDockerDraft() {
    dockerAccessMode = 'direct';
    dockerHost = '';
    dockerTimeoutSeconds = 10;
    dockerCABase64 = '';
    dockerCertBase64 = '';
    dockerKeyBase64 = '';
    dockerSkipTLSVerify = false;
    dockerMCPServerResourceId = '';
    dockerAgentConnectionOverride = false;
    dockerConfigurationAttempted = false;
    dockerCredentialLoading = false;
    dockerDraftTestBusy = false;
    dockerDraftTest = null;
    editingDockerResourceId = '';
  }

  function resetKubernetesDraft() { kubernetesConnectionMode='kubeconfig'; kubernetesServer=''; kubernetesCABase64=''; kubernetesToken=''; kubernetesCertBase64=''; kubernetesKeyBase64=''; kubernetesKubeconfig=''; kubernetesSkipTLSVerify=false; kubernetesMCPServerResourceId=''; kubernetesConnectionOverride=false; kubernetesConfigurationAttempted=false; kubernetesCredentialLoading=false; kubernetesDraftTestBusy=false; kubernetesDraftTest=null; editingKubernetesResourceId=''; }

  function resetProviderDraft() {
    providerType = 'openai_compatible';
    providerProtocol = 'chat_completions';
    providerBaseURL = '';
    providerAPIKey = '';
    providerAPIKeyVisible = false;
    providerAPIKeyLoading = false;
    providerTimeoutSeconds = 60;
    providerMaxConcurrency = 5;
    providerRateLimitPerMinute = 0;
    providerModels = [];
    providerModelDraft = emptyProviderModelDraft();
    editingProviderModelName = '';
    providerDefaultModel = '';
    providerPurposeTags = [];
    editingProviderResourceId = '';
    providerConfigurationAttempted = false;
    providerModelConfigurationAttempted = false;
    providerModelValidationMessage = '';
    providerSummaryAttempted = false;
    providerDraftTest = null;
  }

  function providerModelDraftComplete() {
    return Boolean(providerModelDraft.name.trim() && providerModelDraft.contextWindowTokens > 0 && providerModelDraft.capabilities.length > 0);
  }

  function providerDraftSignature() {
    const defaultModel = providerModels.find((model) => model.name === providerDefaultModel);
    return JSON.stringify({
      scope: selectedScopeId,
      providerType,
      baseURL: providerBaseURL.trim(),
      apiKey: providerAPIKey,
      timeoutSeconds: providerTimeoutSeconds,
      model: defaultModel ? {
        name: defaultModel.name,
        contextWindowTokens: defaultModel.contextWindowTokens,
        temperature: defaultModel.temperature,
        capabilities: defaultModel.capabilities
      } : null
    });
  }

  $: providerDraftTestPassedState = Boolean(
    providerDraftTest?.signature === providerDraftSignature() &&
      providerDraftTest.result?.status === 'succeeded'
  );

  function mcpConfigurationValid() {
    return validateMCPConfiguration(mcpTransport, mcpURL, mcpRequestHeaders);
  }

  function providerBaseURLValid() {
    return isProviderBaseURLValid(providerBaseURL);
  }

  function providerNameDuplicate() {
    const name = resourceName.trim().toLocaleLowerCase();
    if (!name) return false;
    return resources.some(
      (resource) =>
        resource.kind === 'AIProvider' &&
        resource.scope_id === selectedScopeId &&
        resource.id !== editingProviderResourceId &&
        resource.name.trim().toLocaleLowerCase() === name
    );
  }

  function providerConfigurationComplete() {
    return Boolean(
      !providerNameDuplicate() &&
        providerType &&
        providerBaseURLValid()
    );
  }

  function providerConfigurationIssues() {
    const issues: string[] = [];
    if (providerNameDuplicate()) issues.push('资源名称已存在');
    if (!providerType) issues.push('Provider 类型');
    if (!providerBaseURL.trim()) issues.push('服务地址');
    else if (!providerBaseURLValid()) issues.push('服务地址格式');
    return issues;
  }

  function providerDefaultModelDraft() {
    return providerModels.find((model) => model.name === providerDefaultModel);
  }

  function providerPurposeMissingCapabilities(purpose: string) {
    const option = providerPurposeOptions.find((item) => item.value === purpose);
    const defaultModel = providerDefaultModelDraft();
    const required = option?.requiredCapabilities ?? [];
    return missingProviderCapabilities(required, defaultModel);
  }

  function providerPurposeAvailable(purpose: string) {
    return Boolean(providerDefaultModelDraft()) &&
      providerPurposeMissingCapabilities(purpose).length === 0;
  }

  function providerPurposeConfigurationValid() {
    return providerPurposeTags.every((purpose) => providerPurposeAvailable(purpose));
  }

  function providerSummaryValidationMessage() {
    if (providerAPIKeyLoading) return '正在读取 Provider API Key，请稍候后再保存。';
    if (!providerPurposeConfigurationValid())
      return '当前选择的角色与默认 Model 的能力不匹配，请调整角色或 Model 能力。';
    return '';
  }

  function continueProviderAdd() {
    if (resourceAddStep === 2) {
      providerConfigurationAttempted = true;
      if (providerConfigurationComplete()) {
        providerConfigurationAttempted = false;
        resourceAddStep = 3;
      }
      return;
    }
    if (resourceAddStep === 3) {
      providerModelConfigurationAttempted = true;
      if (providerModels.length > 0) {
        providerModelConfigurationAttempted = false;
        autoSummaryTestKey = '';
        resourceAddStep = 4;
      }
    }
  }

  function resourceAddStepValidationMessage() {
    if (resourceAddStep === 1 && resourceBasicConfigurationAttempted) {
      const issues: string[] = [];
      if (!resourceAddCategory) issues.push('资源类型');
      if (!resourceAddSubtype) issues.push('资源子类型');
      if (!resourceName.trim()) issues.push('资源名称');
      return issues.length ? `请填写：${issues.join('、')}。` : '';
    }
    if (
      resourceKind === 'MCPServer' &&
      resourceAddStep === 2 &&
      mcpConfigurationAttempted &&
      !mcpConfigurationValid()
    )
      return '请填写有效的 Server 地址和请求 Header。';
    if (
      resourceKind === 'AIProvider' &&
      resourceAddStep === 2 &&
      providerConfigurationAttempted &&
      !providerConfigurationComplete()
    )
      return `请检查：${providerConfigurationIssues().join('、')}。`;
    if (
      resourceKind === 'AIProvider' &&
      resourceAddStep === 3 &&
      providerModelValidationMessage
    )
      return providerModelValidationMessage;
    if (
      resourceKind === 'AIProvider' &&
      resourceAddStep === 3 &&
      providerModelConfigurationAttempted &&
      providerModels.length === 0
    )
      return '请至少添加一个 Model 后继续。';
    if (
      resourceKind === 'AIProvider' &&
      resourceAddStep === 4 &&
      providerSummaryAttempted &&
      !providerPurposeConfigurationValid()
    )
      return '当前选择的角色与默认 Model 的能力不匹配，请调整角色或 Model 能力。';
    if (resourceKind === 'Docker' && resourceAddStep === 2 && dockerConfigurationAttempted && !dockerConfigurationComplete()) {
      const issues = dockerConfigurationIssues();
      return `请检查：${issues.length ? issues.join('、') : 'Docker 配置'}。`;
    }
    if (resourceKind === 'Kubernetes' && resourceAddStep === 2 && kubernetesConfigurationAttempted && !kubernetesConfigurationComplete()) return '请检查 Kubernetes 配置。';
    return '';
  }

  function providerConfigForCreate(): Record<string, unknown> {
    return buildProviderConfigForCreate({
      type: providerType,
      protocol: providerProtocol,
      baseURL: providerBaseURL,
      timeoutSeconds: providerTimeoutSeconds,
      maxConcurrency: providerMaxConcurrency,
      rateLimitPerMinute: providerRateLimitPerMinute,
      enabled: resourceStatus === 'active',
      defaultModel: providerDefaultModel,
      models: providerModels
    });
  }

  function mcpConfigForSave(): Record<string, unknown> {
    return buildMCPConfig({
      transport: mcpTransport,
      url: mcpURL,
      toolAllowlist: mcpToolAllowlist,
      timeoutSeconds: mcpTimeoutSeconds,
      maxResponseBytes: mcpMaxResponseBytes
    });
  }

  async function createProviderCredential(name = resourceName) {
    if (!selectedScopeId) throw new Error('未选择资源归属级别，无法保存 Provider。');
    return createProviderCredentialAction(selectedScopeId, name, providerAPIKey);
  }

  async function saveProviderCredential(provider: Resource) {
    return saveProviderCredentialAction(provider, selectedScopeId, resourceName, providerAPIKey);
  }

  async function createMCPCredential() {
    const token = mcpToken.trim();
    const headers = parseMCPHeaders(mcpRequestHeaders);
    if (!selectedScopeId || (!token && Object.keys(headers).length === 0)) return '';
    return createMCPCredentialAction(selectedScopeId, resourceName, token, headers);
  }

  async function saveMCPCredential(existing: Resource) {
    const headers = parseMCPHeaders(mcpRequestHeaders);
    return saveMCPCredentialAction(existing, selectedScopeId, editResourceName.trim() || resourceName.trim(), mcpToken, headers);
  }

  async function createDockerCredential() {
    return createDockerCredentialAction(selectedScopeId, resourceName, dockerCredentialForSave(dockerDraft()));
  }

  async function saveDockerCredential(existing: Resource) {
    return saveDockerCredentialAction(existing, selectedScopeId, resourceName, dockerCredentialForSave(dockerDraft()));
  }

  function mcpDraftSignature() {
    return JSON.stringify({ transport: mcpTransport, url: mcpURL.trim(), token: mcpToken, headers: mcpRequestHeaders, tools: mcpToolAllowlist, timeout: mcpTimeoutSeconds, max: mcpMaxResponseBytes });
  }

  function mcpHeaderCount() {
    try { return Object.keys(parseMCPHeaders(mcpRequestHeaders)).length; } catch { return 0; }
  }

  async function testMCPDraftConnection() {
    const signature = mcpDraftSignature();
    mcpDraftTestBusy = true;
    mcpDraftTest = { signature };
    try {
      if (!mcpConfigurationValid()) throw new Error('请填写有效的 Server 地址和请求 Header。');
      const result = await testDraftMCPConnection({
        transport: mcpTransport,
        url: mcpURL.trim(),
        token: mcpToken.trim(),
        request_headers: parseMCPHeaders(mcpRequestHeaders),
        tool_allowlist: mcpToolAllowlist.split(/[\n,]/).map((item) => item.trim()).filter(Boolean),
        timeout_seconds: Number(mcpTimeoutSeconds),
        max_response_bytes: Number(mcpMaxResponseBytes)
      });
      mcpDraftTest = result.status === 'succeeded' ? { signature, result } : { signature, result, error: result.error_message || 'MCP Server 不可用。' };
    } catch (error) {
      mcpDraftTest = { signature, error: describeError(error, 'MCP Server 验证失败') };
    } finally {
      mcpDraftTestBusy = false;
    }
  }

  async function testProviderDraftConnection() {
    providerDraftTestBusy = true;
    const initialSignature = providerDraftSignature();
    providerDraftTest = { signature: initialSignature, error: '正在测试默认 Model，请稍候…' };
    const defaultModel = providerModels.find((model) => model.name === providerDefaultModel);
    if (!selectedScopeId) {
      providerDraftTest = { signature: initialSignature, error: '未选择资源归属级别，无法执行连接测试。' };
      providerDraftTestBusy = false;
      return;
    }
    if (!defaultModel) {
      providerDraftTest = { signature: initialSignature, error: '尚未选择默认 Model，请先在 Model 配置步骤中选择。' };
      providerDraftTestBusy = false;
      return;
    }
    if (!providerBaseURLValid()) {
      providerDraftTest = { signature: initialSignature, error: '服务地址无效，请返回 Provider 配置检查地址。' };
      providerDraftTestBusy = false;
      return;
    }
    try {
      const result = await testDraftAIProviderConnection({
        scope_id: selectedScopeId,
        provider_type: providerType,
        base_url: providerBaseURL.trim(),
        model_name: defaultModel.name,
        api_key: providerAPIKey,
        timeout_seconds: providerTimeoutSeconds,
        context_window: defaultModel.contextWindowTokens,
        temperature: defaultModel.temperature,
        capabilities: defaultModel.capabilities,
        stream: defaultModel.capabilities.includes('stream')
      });
      providerDraftTest = { signature: initialSignature, result };
    } catch (error) {
      providerDraftTest = { signature: initialSignature, error: describeError(error, 'Provider 连接测试失败') };
    } finally {
      providerDraftTestBusy = false;
    }
  }

  function addProviderModel() {
    providerModelConfigurationAttempted = true;
    if (!providerModelDraftComplete()) {
      providerModelValidationMessage = '请补全带 * 的 Model 字段，并至少选择一项能力。';
      return;
    }
    const model = { ...providerModelDraft, name: providerModelDraft.name.trim(), capabilities: [...providerModelDraft.capabilities] };
    if (providerModels.some((item) => item.name === model.name && item.name !== editingProviderModelName)) {
      providerModelValidationMessage = `Model “${model.name}”已存在，请使用其他名称。`;
      return;
    }
    if (editingProviderModelName) {
      const previousName = editingProviderModelName;
      providerModels = providerModels.map((item) => item.name === previousName ? model : item);
      if (providerDefaultModel === previousName) providerDefaultModel = model.name;
    } else {
      providerModels = [...providerModels, model];
      if (!providerDefaultModel) providerDefaultModel = model.name;
    }
    providerModelDraft = emptyProviderModelDraft();
    editingProviderModelName = '';
    providerModelConfigurationAttempted = false;
    providerModelValidationMessage = '';
  }

  function editProviderModel(model: ProviderModel) {
    editingProviderModelName = model.name;
    providerModelDraft = { ...model, capabilities: [...model.capabilities] };
    providerModelConfigurationAttempted = false;
    providerModelValidationMessage = '';
  }

  function removeProviderModel(name: string) {
    providerModels = providerModels.filter((model) => model.name !== name);
    if (editingProviderModelName === name) {
      providerModelDraft = emptyProviderModelDraft();
      editingProviderModelName = '';
    }
    if (providerDefaultModel === name) {
      providerDefaultModel = providerModels[0]?.name ?? '';
      if (providerDefaultModel) setProviderDefaultModel(providerDefaultModel);
    }
  }

  function resourceBasicConfigurationComplete() {
    return Boolean(resourceAddCategory && resourceAddSubtype && resourceName.trim() && selectedScopeId);
  }

  function resourceSchemaConfigurationComplete() {
    if (!createSchema?.schema.required?.length) return true;
    return createSchema.schema.required.every((key) => {
      const field = createSchema?.schema.properties?.[key];
      const value = field?.sensitive ? resourceSensitiveValues[key] : resourceConfigValues[key];
      return Boolean(String(value ?? '').trim());
    });
  }

  function dockerDraft() {
    const tlsSupported = dockerHostSupportsTLS(dockerHost);
    return {
      accessMode: dockerAccessMode,
      host: dockerHost,
      timeoutSeconds: dockerTimeoutSeconds,
      caBase64: tlsSupported ? dockerCABase64 : '',
      certBase64: tlsSupported ? dockerCertBase64 : '',
      keyBase64: tlsSupported ? dockerKeyBase64 : '',
      skipTLSVerify: tlsSupported && dockerSkipTLSVerify,
      mcpServerResourceId: dockerMCPServerResourceId,
      connectionOverride: dockerAgentConnectionOverride
    } as const;
  }

  function kubernetesIsAgent() { return resourceAddSubtype.trim().toLowerCase() === 'agent'; }
  function kubernetesDraft() { return { isAgent:kubernetesIsAgent(), connectionOverride:kubernetesConnectionOverride, connectionMode:kubernetesConnectionMode, server:kubernetesServer, caBase64:kubernetesCABase64, token:kubernetesToken, certBase64:kubernetesCertBase64, keyBase64:kubernetesKeyBase64, kubeconfig:kubernetesKubeconfig, skipTLSVerify:kubernetesSkipTLSVerify } as const; }
  function kubernetesConfigurationComplete() { if (kubernetesCredentialLoading) return false; if (kubernetesIsAgent() && !kubernetesMCPServerResourceId.trim()) return false; if (kubernetesIsAgent() && !kubernetesMCPServers.some((server) => server.id === kubernetesMCPServerResourceId && server.status === 'active')) return false; return kubernetesConfigurationValid(kubernetesDraft()); }
  async function createKubernetesCredential() { return createKubernetesCredentialAction(selectedScopeId, resourceName, kubernetesCredentialForSave(kubernetesDraft())); }
  async function saveKubernetesCredential(existing: Resource) { return saveKubernetesCredentialAction(existing, selectedScopeId, resourceName, kubernetesCredentialForSave(kubernetesDraft())); }
  async function testKubernetesDraftConnection() { kubernetesDraftTestBusy=true; const draft=kubernetesDraft(); kubernetesDraftTest={error:'正在测试 Kubernetes 连接，请稍候…'}; try { if (!kubernetesConfigurationComplete()) throw new Error('请检查 Kubernetes 配置。'); if (draft.isAgent && !draft.connectionOverride) { const server = resources.find((resource) => resource.id === kubernetesMCPServerResourceId); if (!server) throw new Error('未找到关联的 MCPServer。'); const snapshot = await api.discoverMCP(server.id); kubernetesDraftTest={status:snapshot.status,message:snapshot.error_message || (snapshot.status === 'succeeded' ? `MCPServer 连接正常，发现 ${snapshot.tools?.length ?? 0} 个工具` : 'MCPServer 连接失败'),latency:snapshot.latency_ms,error:snapshot.status === 'succeeded' ? '' : snapshot.error_message || 'MCPServer 连接失败'}; return; } const result=await api.testDraftKubernetes({kubeconfig:kubernetesKubeconfigText(draft.kubeconfig),server:draft.server,ca_file:draft.caBase64,token:draft.token,client_cert_file:draft.certBase64,client_key_file:draft.keyBase64,skip_tls_verify:draft.skipTLSVerify}); kubernetesDraftTest={status:result.status,message:result.message,latency:result.latency_ms,error:result.status==='succeeded'?'':result.message}; } catch(error) { kubernetesDraftTest={error:describeError(error,'Kubernetes 连接测试失败')}; } finally { kubernetesDraftTestBusy=false; } }

  function dockerConfigurationComplete() {
    if (dockerCredentialLoading) return false;
    if (!dockerConnectionConfigurationValid(dockerDraft())) return false;
    if (dockerAccessMode === 'agent' && !dockerAgentConnectionOverride) {
      const server = resources.find((resource) => resource.id === dockerMCPServerResourceId);
      return Boolean(server && server.kind === 'MCPServer' && server.status === 'active');
    }
    return true;
  }

  function dockerConfigurationIssues() {
    const issues: string[] = [];
    if (dockerCredentialLoading) issues.push('正在读取 TLS 凭据');
    if (dockerAccessMode === 'agent' && !dockerAgentConnectionOverride) {
      if (!dockerMCPServerResourceId) issues.push('关联 MCPServer');
      else if (!dockerMCPServers.some((server) => server.id === dockerMCPServerResourceId && server.status === 'active')) issues.push('活动的 MCPServer');
      return issues;
    }
    if (!dockerHost.trim() || !dockerHostValidForDraft()) issues.push('Docker Host URL');
    if (Boolean(dockerCertBase64.trim()) !== Boolean(dockerKeyBase64.trim())) issues.push('客户端证书和私钥');
    if (dockerCABase64.trim() && !dockerTLSValueValid(dockerCABase64)) issues.push('CA 证书 Base64 内容');
    if (dockerCertBase64.trim() && !dockerTLSValueValid(dockerCertBase64)) issues.push('客户端证书 Base64 内容');
    if (dockerKeyBase64.trim() && !dockerTLSValueValid(dockerKeyBase64)) issues.push('客户端私钥 Base64 内容');
    if ((dockerCABase64.trim() || dockerCertBase64.trim() || dockerKeyBase64.trim() || dockerSkipTLSVerify) && !dockerHost.trim()) issues.push('启用 TLS 时填写 Docker Host URL');
    return issues;
  }

  function dockerHostValidForDraft() {
    return dockerHostValid(dockerHost);
  }

  function selectDockerAccessMode(mode: DockerAccessMode) {
    dockerAccessMode = mode;
    resourceAddSubtype = mode === 'agent' ? 'Agent' : 'Direct';
    if (mode === 'agent') {
      dockerAgentConnectionOverride = false;
      dockerHost = '';
      dockerCABase64 = '';
      dockerCertBase64 = '';
      dockerKeyBase64 = '';
      dockerSkipTLSVerify = false;
    } else {
      dockerMCPServerResourceId = '';
    }
    dockerConfigurationAttempted = false;
    dockerDraftTest = null;
  }

  function dockerMCPServerName() {
    return resources.find((resource) => resource.id === dockerMCPServerResourceId)?.name ?? '';
  }

  function resetDockerDraftTest() {
    if (!dockerDraftTestBusy) {
      dockerDraftTest = null;
    }
  }

  async function testDockerDraftConnection() {
    dockerDraftTestBusy = true;
    dockerDraftTest = { error: '正在测试 Docker 连接，请稍候…' };
    try {
      if (!dockerConfigurationComplete()) {
        throw new Error(`请检查：${dockerConfigurationIssues().join('、') || 'Docker 配置'}。`);
      }
      if (dockerAccessMode === 'agent' && !dockerAgentConnectionOverride) {
        const server = resources.find((resource) => resource.id === dockerMCPServerResourceId);
        if (!server) throw new Error('未找到关联的 MCPServer。');
        const snapshot = await api.discoverMCP(server.id);
        dockerDraftTest = {
          status: snapshot.status,
          message: snapshot.error_message || (snapshot.status === 'succeeded' ? `MCPServer 连接正常，发现 ${snapshot.tools?.length ?? 0} 个工具` : 'MCPServer 连接失败'),
          latency: snapshot.latency_ms,
          toolCount: snapshot.tools?.length ?? 0,
          error: snapshot.status === 'succeeded' ? '' : snapshot.error_message || 'MCPServer 连接失败'
        };
        return;
      }
      const credential = dockerCredentialForSave(dockerDraft());
      const result = await api.testDraftDocker({
        host: dockerHost.trim(),
        timeout: dockerTimeoutSeconds,
        tls_ca: credential.tls_ca,
        tls_cert: credential.tls_cert,
        tls_key: credential.tls_key,
        skip_tls_verify: dockerSkipTLSVerify
      });
      dockerDraftTest = {
        status: result.status,
        message: result.message,
        latency: result.latency_ms,
        error: result.status === 'succeeded' ? '' : result.message
      };
    } catch (error) {
      dockerDraftTest = { error: describeError(error, 'Docker 连接测试失败') };
    } finally {
      dockerDraftTestBusy = false;
    }
  }

  function continueResourceAdd() {
    resourceTypeSelectionAttempted = true;
    resourceBasicConfigurationAttempted = true;
    if (!resourceBasicConfigurationComplete()) return;
    resourceTypeSelectionAttempted = false;
    resourceBasicConfigurationAttempted = false;
    chooseResourceAddSubtype(resourceAddCategory, resourceAddSubtype, !editingProviderResourceId && !editingResourceId && !editingDockerResourceId);
    resourceAddStep = 2;
  }

  function continueDockerAdd() {
    dockerConfigurationAttempted = true;
    if (dockerConfigurationComplete()) {
      dockerConfigurationAttempted = false;
      autoSummaryTestKey = '';
      resourceAddStep = 3;
    }
  }
  function continueKubernetesAdd() { kubernetesConfigurationAttempted=true; if (kubernetesConfigurationComplete()) { kubernetesConfigurationAttempted=false; autoSummaryTestKey=''; resourceAddStep=3; } }

  async function updateSelectedResource() {
    if (!selectedResource) return;
    if (selectedResource.kind === 'AIProvider') {
      await updateProviderFromWorkflow();
      return;
    }
    if (selectedResource.kind === 'MCPServer') {
      await updateMCPFromWorkflow();
      return;
    }
    try {
      const config = buildSchemaConfig(selectedSchema, resourceConfigValues, editResourceConfig);
      if (resourceSupportsEndpointTimeout(selectedResource.kind)) config.timeout_seconds = genericTimeoutSeconds;
      const credentialId = await createResourceCredential(selectedSchema, editResourceSensitiveValues);
      const updated = await updateResourceRecord(selectedResource.id, {
        name: editResourceName,
        subtype: resourceSubtypeFor({ kind: selectedResource.kind, config }),
        status: editResourceStatus,
        labels: parseLabels(editResourceLabels),
        config,
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      resources = resources.map((resource) => resource.id === updated.id ? updated : resource);
      syncResourceEditor(updated);
      onNotice(`资源“${updated.name}”已更新`);
    } catch (error) {
      onError(describeError(error, '更新资源失败'));
    }
  }

  async function createResource() {
    if (resourceKind === 'AIProvider') {
      await createSpecialResource();
      return;
    }
    if (resourceKind === 'MCPServer') {
      await createSpecialResource();
      return;
    }
    if (resourceKind === 'Docker') {
      await createDockerFromWorkflow();
      return;
    }
    if (resourceKind === 'Kubernetes') { await createKubernetesFromWorkflow(); return; }
    try {
      if (!resourceSchemaConfigurationComplete()) {
        resourceBasicConfigurationAttempted = true;
        throw new Error('请填写所有必填配置项。');
      }
      if (!resourceAddCategory || !resourceAddSubtype || !resourceName.trim()) {
        throw new Error('请先完成基础配置中的资源类型、资源子类型和资源名称。');
      }
      const config = buildSchemaConfig(createSchema, resourceConfigValues, resourceConfig);
      if (resourceSupportsEndpointTimeout(resourceKind)) config.timeout_seconds = genericTimeoutSeconds;
      const credentialId = await createResourceCredential(createSchema, resourceSensitiveValues);
      const created = await createResourceRecord({
        scope_id: selectedScopeId,
        kind: resourceKind,
        subtype: resourceAddSubtype || resourceSubtypeFor({ kind: resourceKind, config }),
        name: resourceName.trim(),
        status: resourceStatus,
        labels: parseLabels(resourceLabels),
        config,
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      resources = [created, ...resources];
      selectedResourceId = created.id;
      resourceName = '';
      resourceLabels = '';
      resourceConfig = '{}';
      resourceConfigValues = {};
      resourceSensitiveValues = {};
      resourceEditorOpen = false;
      resourceAddMenuOpen = false;
      resourceAddStep = 1;
      onNotice(`资源“${created.name}”已创建`);
      await testResourceConnection(created, false);
      await loadResourceDetails(created.id);
    } catch (error) {
      onError(describeError(error, '创建资源失败'));
    }
  }

  async function createKubernetesFromWorkflow() { await runResourceAction(async () => { if (!resourceBasicConfigurationComplete()) throw new Error('请先完成基础配置中的资源类型、资源子类型和资源名称。'); if (!kubernetesConfigurationComplete()) { kubernetesConfigurationAttempted=true; throw new Error('请检查 Kubernetes 配置。'); } const draft=kubernetesDraft(); const credentialValues=kubernetesCredentialForSave(draft); const credentialId=Object.keys(credentialValues).length ? await createKubernetesCredential() : ''; const created=await createResourceRecord({scope_id:selectedScopeId,kind:'Kubernetes',subtype:kubernetesIsAgent()?'Agent':'Direct',agent_ref:kubernetesIsAgent()?kubernetesMCPServerResourceId:undefined,name:resourceName.trim(),status:resourceStatus,labels:parseLabels(resourceLabels),config:kubernetesConfigForSave(draft),...(credentialId?{credential_id:credentialId}:{})}); resources=[created,...resources]; selectedResourceId=created.id; resetKubernetesDraft(); resourceName=''; resourceLabels=''; resourceAddMenuOpen=false; resourceAddStep=1; onNotice(`资源“${created.name}”已创建`); await testResourceConnection(created,false); await loadResourceDetails(created.id); }); }

  async function createDockerFromWorkflow() {
    await runResourceAction(async () => {
      if (!resourceBasicConfigurationComplete()) throw new Error('请先完成基础配置中的资源类型、资源子类型和资源名称。');
      if (!dockerConfigurationComplete()) {
        dockerConfigurationAttempted = true;
        throw new Error(`请检查：${dockerConfigurationIssues().join('、') || 'Docker 配置'}。`);
      }
      const draft = dockerDraft();
      const credentialId = draft.accessMode === 'direct' || draft.connectionOverride ? await createDockerCredential() : '';
      const created = await createResourceRecord({
        scope_id: selectedScopeId,
        kind: 'Docker',
        subtype: draft.accessMode === 'agent' ? 'Agent' : 'Direct',
        agent_ref: draft.accessMode === 'agent' ? draft.mcpServerResourceId : undefined,
        name: resourceName.trim(),
        status: resourceStatus,
        labels: parseLabels(resourceLabels),
        config: dockerConfigForSave(draft),
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      resources = [created, ...resources];
      selectedResourceId = created.id;
      resetDockerDraft();
      resourceName = '';
      resourceLabels = '';
      resourceAddMenuOpen = false;
      resourceAddStep = 1;
      onNotice(`资源“${created.name}”已创建`);
      await testResourceConnection(created, false);
      await loadResourceDetails(created.id);
    });
  }

  async function runResourceAction(operation: () => Promise<void>) {
    busy = true;
    onError('');
    try {
      await operation();
    } catch (error) {
      onError(describeError(error, '操作失败'));
    } finally {
      busy = false;
    }
  }

  async function createSpecialResource() {
    await runResourceAction(async () => {
      if (!resourceAddCategory || !resourceAddSubtype || !resourceName.trim()) {
        throw new Error('请先完成基础配置中的资源类型、资源子类型和资源名称。');
      }
      const isProvider = resourceKind === 'AIProvider';
      if (isProvider && providerNameDuplicate()) {
        throw new Error('当前级别已存在同名 AI Provider，请更换名称。');
      }
      if (isProvider) {
        const summaryError = providerSummaryValidationMessage();
        if (summaryError) throw new Error(summaryError);
      }
      if (resourceKind === 'MCPServer' && !mcpConfigurationValid()) {
        throw new Error('请填写有效的 MCP Server 地址和配置。');
      }
      const config = isProvider
        ? providerConfigForCreate()
        : resourceKind === 'MCPServer'
          ? mcpConfigForSave()
          : buildSchemaConfig(createSchema, resourceConfigValues, resourceConfig);
      if (!isProvider && resourceKind !== 'MCPServer' && resourceSupportsEndpointTimeout(resourceKind)) config.timeout_seconds = genericTimeoutSeconds;
      const credentialId = isProvider
        ? await createProviderCredential()
        : resourceKind === 'MCPServer'
          ? await createMCPCredential()
          : await createResourceCredential(createSchema, resourceSensitiveValues);
      const created = await createResourceRecord({
        scope_id: selectedScopeId,
        kind: resourceKind,
        subtype: resourceAddSubtype || resourceSubtypeFor({ kind: resourceKind, config }),
        name: resourceName.trim(),
        status: resourceStatus,
        labels: parseLabels(resourceLabels),
        config,
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      if (isProvider && providerPurposeTags.length > 0) {
        const currentScopeBindings = await syncAIProviderBindings(selectedScopeId, created.id, [], providerPurposeTags);
        aiProviderBindings = [
          ...aiProviderBindings.filter((binding) => binding.scope_id !== selectedScopeId),
          ...currentScopeBindings
        ];
      }
      resources = [created, ...resources];
      selectedResourceId = created.id;
      resourceName = '';
      resourceLabels = '';
      resourceConfig = '{}';
      resourceConfigValues = {};
      resourceSensitiveValues = {};
      resourceEditorOpen = false;
      resourceAddMenuOpen = false;
      resourceAddStep = 1;
      onNotice(`资源“${created.name}”已创建`);
      await testResourceConnection(created, false);
      await loadResourceDetails(created.id);
    });
  }

  async function updateProviderFromWorkflow() {
    const provider = resources.find((resource) => resource.id === editingProviderResourceId);
    if (!provider) return;
    await runResourceAction(async () => {
      if (providerNameDuplicate()) {
        throw new Error('当前级别已存在同名 AI Provider，请更换名称。');
      }
      const summaryError = providerSummaryValidationMessage();
      if (summaryError) throw new Error(summaryError);
      const credentialId = await saveProviderCredential(provider);
      const config = providerConfigForCreate();
      const updated = await updateResourceRecord(provider.id, {
        name: resourceName.trim(),
        subtype: resourceSubtypeFor({ kind: 'AIProvider', config }),
        status: resourceStatus,
        labels: parseLabels(resourceLabels),
        config,
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      const existingTags = aiProviderBindings
        .filter((binding) => binding.scope_id === selectedScopeId && binding.provider_resource_id === provider.id)
        .map((binding) => binding.tag);
      const currentScopeBindings = await syncAIProviderBindings(selectedScopeId, provider.id, existingTags, providerPurposeTags);
      aiProviderBindings = [
        ...aiProviderBindings.filter((binding) => binding.scope_id !== selectedScopeId),
        ...currentScopeBindings
      ];
      resources = resources.map((resource) => resource.id === updated.id ? updated : resource);
      selectedResourceId = updated.id;
      editingProviderResourceId = '';
      resourceAddMenuOpen = false;
      resourceAddStep = 1;
      onNotice(`Provider“${updated.name}”已更新`);
      await testResourceConnection(updated, false);
      await loadResourceDetails(updated.id);
    });
  }

  async function updateMCPFromWorkflow() {
    const server = resources.find((resource) => resource.id === editingResourceId);
    if (!server) return;
    await runResourceAction(async () => {
      if (!mcpConfigurationValid()) {
        throw new Error('请填写有效的 MCP Server 地址和配置。');
      }
      const config = mcpConfigForSave();
      const credentialId = await saveMCPCredential(server);
      const updated = await updateResourceRecord(server.id, {
        name: resourceName.trim(),
        subtype: resourceSubtypeFor({ kind: 'MCPServer', subtype: resourceAddSubtype, config }),
        status: resourceStatus,
        labels: parseLabels(resourceLabels),
        config,
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      resources = resources.map((resource) => resource.id === updated.id ? updated : resource);
      selectedResourceId = updated.id;
      editingResourceId = '';
      resourceAddMenuOpen = false;
      resourceAddStep = 1;
      onNotice(`MCPServer“${updated.name}”已更新`);
      await testResourceConnection(updated, false);
      await loadResourceDetails(updated.id);
    });
  }

  async function updateDockerFromWorkflow() {
    const docker = resources.find((resource) => resource.id === editingDockerResourceId);
    if (!docker) return;
    await runResourceAction(async () => {
      if (!dockerConfigurationComplete()) {
        dockerConfigurationAttempted = true;
        throw new Error(`请检查：${dockerConfigurationIssues().join('、') || 'Docker 配置'}。`);
      }
      const draft = dockerDraft();
      const credentialId = draft.accessMode === 'direct' || draft.connectionOverride ? await saveDockerCredential(docker) : null;
      const updated = await updateResourceRecord(docker.id, {
        name: resourceName.trim(),
        subtype: draft.accessMode === 'agent' ? 'Agent' : 'Direct',
        agent_ref: draft.accessMode === 'agent' ? draft.mcpServerResourceId : null,
        status: resourceStatus,
        labels: parseLabels(resourceLabels),
        config: dockerConfigForSave(draft),
        credential_id: credentialId || null
      });
      resources = resources.map((resource) => resource.id === updated.id ? updated : resource);
      selectedResourceId = updated.id;
      editingDockerResourceId = '';
      resourceAddMenuOpen = false;
      resourceAddStep = 1;
      onNotice(`Docker 资源“${updated.name}”已更新`);
      await testResourceConnection(updated, false);
      await loadResourceDetails(updated.id);
    });
  }
  async function updateKubernetesFromWorkflow() { const item=resources.find((resource)=>resource.id===editingKubernetesResourceId); if (!item) return; await runResourceAction(async()=>{ if (!kubernetesConfigurationComplete()) throw new Error('请检查 Kubernetes 配置。'); const draft=kubernetesDraft(); const credentialValues=kubernetesCredentialForSave(draft); const credentialId=Object.keys(credentialValues).length ? await saveKubernetesCredential(item) : null; const updated=await updateResourceRecord(item.id,{name:resourceName.trim(),subtype:kubernetesIsAgent()?'Agent':'Direct',agent_ref:kubernetesIsAgent()?kubernetesMCPServerResourceId:null,status:resourceStatus,labels:parseLabels(resourceLabels),config:kubernetesConfigForSave(draft),credential_id:credentialId}); resources=resources.map((resource)=>resource.id===updated.id?updated:resource); selectedResourceId=updated.id; editingKubernetesResourceId=''; resourceAddMenuOpen=false; resourceAddStep=1; onNotice(`Kubernetes 资源“${updated.name}”已更新`); await testResourceConnection(updated,false); await loadResourceDetails(updated.id); }); }

  function submitProviderCreate() {
    providerSummaryAttempted = true;
    void (editingProviderResourceId ? updateProviderFromWorkflow() : createSpecialResource());
  }

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 403) return '当前账号没有执行此操作的权限。';
      if (error.status === 401) return '会话已过期，请重新登录。';
      return error.message || fallback;
    }
    return error instanceof Error ? error.message || fallback : fallback;
  }

  async function loadMCPSnapshots(resourceId: string) {
    try {
      operationSnapshots = {
        ...operationSnapshots,
        [resourceId]: await loadMCPSnapshotsAction(resourceId)
      };
    } catch (error) {
      onError(describeError(error, 'MCP 工具快照加载失败'));
    }
  }

  async function testResourceConnection(resource: Resource, notify = true) {
    if (!resourceHasConnector(resource)) return;
    if (connectionBusyResourceIds.includes(resource.id)) return;
    connectionBusyResourceIds = [...connectionBusyResourceIds, resource.id];
    connectionBusy = true;
    onError('');
    try {
      const result = await testResourceConnector(resource, selectedScopeId);
      const check = result.check;
      connectionDetailResourceId = resource.id;
      if (result.snapshot) operationSnapshots = prependMCPSnapshot(operationSnapshots, resource.id, result.snapshot);
      if (selectedResourceId === resource.id) connectionCheck = check;
      resourceConnectionChecks = { ...resourceConnectionChecks, [resource.id]: check };
      if (notify && resource.kind !== 'AIProvider') onNotice(check.status === 'succeeded' ? `资源“${resource.name}”连接测试通过` : `资源“${resource.name}”连接测试失败`);
    } catch (error) {
      const message = describeError(error, '连接测试失败');
      const failedCheck: ConnectionCheck = { id: `connection-${resource.id}`, resource_id: resource.id, status: 'failed', message, latency_ms: 0, capabilities: [], checked_at: new Date().toISOString() };
      connectionDetailResourceId = resource.id;
      if (selectedResourceId === resource.id) connectionCheck = failedCheck;
      resourceConnectionChecks = { ...resourceConnectionChecks, [resource.id]: failedCheck };
      onError(message);
    } finally {
      connectionBusyResourceIds = connectionBusyResourceIds.filter((id) => id !== resource.id);
      connectionBusy = connectionBusyResourceIds.length > 0;
    }
  }

  async function testSelectedResourceConnection() {
    if (selectedResource && selectedResourceHasConnector) await testResourceConnection(selectedResource);
  }

  async function testResourceRowConnection(resource: Resource) {
    selectedResourceId = resource.id;
    void loadResourceDetails(resource.id);
    await testResourceConnection(resource);
  }

  async function createRelation() {
    if (!selectedResource || !relationTarget) return;
    relationBusy = true;
    onError('');
    try {
      await createResourceRelation(selectedResource.id, relationTarget, relationType);
      relationTarget = '';
      onNotice('资源关系已建立');
      await loadResourceDetails(selectedResource.id);
    } catch (error) {
      onError(describeError(error, '建立资源关系失败'));
    } finally {
      relationBusy = false;
    }
  }

  async function deleteRelation(relation: Relation) {
    if (!selectedResource) return;
    relationBusy = true;
    onError('');
    try {
      await removeResourceRelation(selectedResource.id, relation);
      onNotice('资源关系已删除');
      await loadResourceDetails(selectedResource.id);
    } catch (error) {
      onError(describeError(error, '删除资源关系失败'));
    } finally {
      relationBusy = false;
    }
  }

  async function toggleResourceEnabled(resource: Resource, enabled: boolean) {
    if (!resourceCanManage(resource, 'resource:update')) return;
    resourceActionBusy = true;
    onError('');
    try {
      const updated = await setResourceEnabled(resource, enabled);
      resources = resources.map((item) => item.id === updated.id ? updated : item);
      if (selectedResourceId === updated.id) {
        editResourceName = updated.name;
        editResourceStatus = updated.status;
        editResourceLabels = Object.entries(updated.labels ?? {}).map(([key, value]) => `${key}=${value}`).join(', ');
        editResourceConfig = JSON.stringify(updated.config ?? {}, null, 2);
      }
      onNotice(`资源“${updated.name}”已${enabled ? '启用' : '停用'}`);
    } catch (error) {
      onError(describeError(error, '更新资源状态失败'));
    } finally {
      resourceActionBusy = false;
    }
  }

  async function deleteSelectedResource() {
    const resource = selectedResource;
    if (!resource || !resourceCanManage(resource, 'resource:delete')) return;
    resourceActionBusy = true;
    onError('');
    try {
      await removeResource(resource.id);
      resources = resources.filter((item) => item.id !== resource.id);
      selectedResourceId = '';
      relations = [];
      topology = [];
      onNotice('资源已停用并从当前列表移除');
    } catch (error) {
      onError(describeError(error, '停用资源失败'));
    } finally {
      resourceActionBusy = false;
    }
  }

  let resourceCategory = '全部';
  let resourceSubtype = '全部';
  let resourceSearch = '';
  let resourceStatusFilter = 'all';
  let resourceLevelFilter = 'all';
  $: resourceCatalogItems = visibleResources.filter((resource) => {
    if (resourceCategory !== '全部' && resourceCategoryFor(resource) !== resourceCategory) return false;
    if (resourceSubtype !== '全部' && resourceSubtypeFor(resource) !== resourceSubtype) return false;
    if (resourceStatusFilter !== 'all' && resource.status !== resourceStatusFilter) return false;
    if (resourceLevelFilter !== 'all' && scopeType(resource.scope_id) !== resourceLevelFilter) return false;
    const query = resourceSearch.trim().toLowerCase();
    return !query || [resource.name, resource.kind, resource.external_uid ?? '', resourceEndpointFor(resource), Object.entries(resource.labels ?? {}).map(([key, value]) => `${key}=${value}`).join(' ')].join(' ').toLowerCase().includes(query);
  });
  function selectResourceCategory(category: string, subtype = '全部') {
    resourceCategory = category;
    resourceSubtype = subtype;
  }

  async function refreshResources() {
    if (resourceRefreshBusy) return;
    resourceRefreshBusy = true;
    try {
      await onWorkspaceReload();
    } finally {
      resourceRefreshBusy = false;
    }
  }
</script>

<section class="resources-layout">
  <ResourceCatalogRail
    resources={visibleResources}
    category={resourceCategory}
    subtype={resourceSubtype}
    onSelect={selectResourceCategory}
  />
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
            class="icon-button"
            type="button"
            disabled={busy || resourceRefreshBusy}
            title="刷新资源目录"
            aria-label="刷新资源目录"
            on:click={() => void refreshResources()}
          ><RefreshCw size={15} aria-hidden="true" /></button>
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
      <ResourceCatalogList
        resources={resourceCatalogItems}
        {selectedResourceId}
        {resourceConnectionChecks}
        {connectionDetailResourceId}
        {busy}
        {resourceActionBusy}
        {connectionBusyResourceIds}
        {resourceCanManage}
        {resourcePermissionLabel}
        {scopeType}
        {resourceScopeLabel}
        {resourceIcon}
        {providerModelsForResource}
        {providerDefaultModelForResource}
        {providerModelCapabilities}
        {providerBindingsFor}
        {providerPurposeLabel}
        {mcpServerNameFor}
        onSelect={(resource) => void loadResourceDetails(resource.id)}
        onLoadSnapshot={(resourceId) => void loadMCPSnapshots(resourceId)}
        onToggleEnabled={(resource, enabled) => void toggleResourceEnabled(resource, enabled)}
        onTestConnection={(resource) => void testResourceRowConnection(resource)}
        onEdit={openResourceEditor}
        onDelete={(resource) => void loadResourceDetails(resource.id).then(deleteSelectedResource)}
      >
        <svelte:fragment slot="details" let:resource let:resourceCheck>
          <ResourceCatalogDetails
            {resource}
            {resourceCheck}
            {operationSnapshots}
            {selectedResourceId}
            {connectionCheck}
            {formatDate}
            {resourceCanManage}
            {providerModelsForResource}
            {providerModelCapabilities}
            {providerTypeLabel}
            {providerBindingsFor}
            {providerPurposeLabel}
            {mcpServerNameFor}
          />
        </svelte:fragment>
      </ResourceCatalogList>
    </section>
    {#if resourceAddMenuOpen}
      <ResourceWorkflowPanel
        step={resourceAddStep}
        kind={resourceKind}
        category={resourceAddCategory}
        subtype={resourceAddSubtype}
        editingProvider={Boolean(editingProviderResourceId)}
        editingResource={Boolean(editingResourceId)}
        editingDocker={Boolean(editingDockerResourceId)}
        editingKubernetes={Boolean(editingKubernetesResourceId)}
        basicConfigurationComplete={resourceBasicConfigurationComplete()}
        mcpConfigurationComplete={mcpConfigurationValid()}
        dockerConfigurationComplete={dockerConfigurationComplete()}
        kubernetesConfigurationComplete={kubernetesConfigurationComplete()}
        providerModelCount={providerModels.length}
        {busy}
        scopeSelected={Boolean(selectedScopeId)}
        message={activeMessage}
        messageTone={activeMessageTone}
        stepTitle={resourceAddStepTitle(resourceAddStep, resourceKind)}
        stepDescription={resourceAddStepDescription(resourceAddStep, resourceKind)}
        validationMessage={resourceAddStepValidationMessage()}
        onCancel={() => {
          resourceAddMenuOpen = false;
          resourceAddStep = 1;
          editingProviderResourceId = '';
          editingResourceId = '';
          editingDockerResourceId = '';
          editingKubernetesResourceId = '';
        }}
        onSelectStep={(step) => { resourceAddStep = step; autoSummaryTestKey = ''; }}
        onContinueBasic={continueResourceAdd}
        onContinueProvider={continueProviderAdd}
        onContinueMcp={() => {
          mcpConfigurationAttempted = true;
          if (mcpConfigurationValid()) {
            mcpConfigurationAttempted = false;
            resourceAddStep = 3;
          }
        }}
        onContinueDocker={continueDockerAdd}
        onSubmitMcp={() => void (editingResourceId ? updateMCPFromWorkflow() : createResource())}
        onSubmitDocker={() => void (editingDockerResourceId ? updateDockerFromWorkflow() : editingKubernetesResourceId ? updateKubernetesFromWorkflow() : resourceKind === 'Kubernetes' ? createKubernetesFromWorkflow() : createDockerFromWorkflow())}
      >

          {#if resourceAddStep === 1}
            <ResourceBasicConfigStep
              bind:category={resourceAddCategory}
              bind:subtype={resourceAddSubtype}
              bind:name={resourceName}
              bind:status={resourceStatus}
              bind:labels={resourceLabels}
              categoryOptions={resourceCategoryOptions}
              subtypeOptions={resourceAddSubtypeOptions}
              typeSelectionAttempted={resourceTypeSelectionAttempted}
              basicConfigurationAttempted={resourceBasicConfigurationAttempted}
              editing={Boolean(editingProviderResourceId || editingResourceId || editingDockerResourceId || editingKubernetesResourceId)}
              scopeSummary={activeScopeSummary()}
              onSelectCategory={selectResourceAddCategory}
              onSelectSubtype={(subtype) => {
                resourceKind = resourceKindForSelection(resourceAddCategory, subtype);
                if (resourceKind === 'Kubernetes') kubernetesConnectionOverride = false;
              }}
            />
          {:else if resourceKind === 'MCPServer' && resourceAddStep === 2}
            <form id="resource-create-form" class="stack-form resource-create-form" on:submit|preventDefault={createResource}>
              <McpConnectionFields
                bind:url={mcpURL}
                bind:token={mcpToken}
                bind:requestHeaders={mcpRequestHeaders}
                bind:toolAllowlist={mcpToolAllowlist}
                bind:timeoutSeconds={mcpTimeoutSeconds}
                bind:maxResponseBytes={mcpMaxResponseBytes}
                configurationAttempted={mcpConfigurationAttempted}
              />
            </form>
          {:else if resourceKind === 'MCPServer' && resourceAddStep === 3}
            <McpReviewStep
              transport={mcpTransport}
              url={mcpURL}
              tokenConfigured={Boolean(mcpToken.trim())}
              headerCount={mcpHeaderCount()}
              toolAllowlist={mcpToolAllowlist}
              timeoutSeconds={mcpTimeoutSeconds}
              maxResponseBytes={mcpMaxResponseBytes}
              testStatus={mcpDraftTest?.result?.status ?? ''}
              testError={mcpDraftTest?.error ?? ''}
              toolCount={mcpDraftTest?.result?.tools.length ?? 0}
              latency={mcpDraftTest?.result?.latency_ms}
            />
          {:else if resourceKind === 'AIProvider' && resourceAddStep === 2}
            <ProviderConnectionStep
              bind:type={providerType}
              bind:protocol={providerProtocol}
              bind:baseURL={providerBaseURL}
              bind:apiKey={providerAPIKey}
              bind:apiKeyVisible={providerAPIKeyVisible}
              bind:timeoutSeconds={providerTimeoutSeconds}
              bind:maxConcurrency={providerMaxConcurrency}
              bind:rateLimitPerMinute={providerRateLimitPerMinute}
              bind:purposeTags={providerPurposeTags}
              typeOptions={providerTypeOptions}
              purposeOptions={providerPurposeOptions}
              configurationAttempted={providerConfigurationAttempted}
              baseURLValid={providerBaseURLValid()}
              apiKeyLoading={providerAPIKeyLoading}
              onSelectType={selectProviderType}
              onTogglePurpose={toggleProviderPurpose}
            />
          {:else if resourceKind === 'AIProvider' && resourceAddStep === 3}
            <ProviderModelStep
              bind:draft={providerModelDraft}
              models={providerModels}
              bind:defaultModel={providerDefaultModel}
              capabilityOptions={providerCapabilityOptions}
              configurationAttempted={providerModelConfigurationAttempted}
              editingModelName={editingProviderModelName}
              onToggleCapability={toggleProviderModelCapability}
              onAddModel={addProviderModel}
              onSetDefault={setProviderDefaultModel}
              onSetEnabled={setProviderModelEnabled}
              onEditModel={editProviderModel}
              onRemoveModel={removeProviderModel}
            />
          {:else if resourceKind === 'AIProvider' && resourceAddStep === 4}
            <ProviderReviewStep
              {resourceName}
              providerTypeLabel={providerTypeOptions.find((item) => item.value === providerType)?.label ?? providerType}
              providerStatus={resourceStatus}
              baseURL={providerBaseURL}
              protocol={providerProtocol}
              timeoutSeconds={providerTimeoutSeconds}
              maxConcurrency={providerMaxConcurrency}
              defaultModel={providerDefaultModel}
              models={providerModels}
              scopeSummary={activeScopeSummary()}
              labelsConfigured={Boolean(resourceLabelsText({ labels: parseLabels(resourceLabels) } as Resource))}
              purposeLabels={providerPurposeTags.map(providerPurposeLabel)}
              testBusy={providerDraftTestBusy}
              testPassed={providerDraftTestPassedState}
              testLatency={providerDraftTest?.result?.latency_ms}
              testMessage={providerDraftTest?.result?.message ?? ''}
              testError={providerDraftTest?.error ?? ''}
              capabilityLabel={(capability) => providerCapabilityOptions.find((item) => item.value === capability)?.label ?? capability}
              onSubmit={submitProviderCreate}
            />
          {:else if resourceKind === 'Docker' && resourceAddStep === 2}
            <DockerConnectionStep
              bind:accessMode={dockerAccessMode}
              bind:host={dockerHost}
              bind:timeoutSeconds={dockerTimeoutSeconds}
              bind:caBase64={dockerCABase64}
              bind:certBase64={dockerCertBase64}
              bind:keyBase64={dockerKeyBase64}
              bind:skipTLSVerify={dockerSkipTLSVerify}
              bind:connectionOverride={dockerAgentConnectionOverride}
              bind:mcpServerResourceId={dockerMCPServerResourceId}
              mcpServers={dockerMCPServers}
              configurationAttempted={dockerConfigurationAttempted}
              credentialLoading={dockerCredentialLoading}
              onConfigurationChange={resetDockerDraftTest}
            />
          {:else if resourceKind === 'Docker' && resourceAddStep === 3}
            <DockerReviewStep
              {resourceName}
              {resourceStatus}
              accessMode={dockerAccessMode}
              connectionOverride={dockerAgentConnectionOverride}
              host={dockerHost}
              skipTLSVerify={dockerSkipTLSVerify}
              mcpServerName={dockerMCPServerName()}
              credentialConfigured={Boolean(dockerCABase64.trim() || dockerCertBase64.trim() || dockerKeyBase64.trim())}
              scopeSummary={activeScopeSummary()}
              labelsConfigured={Boolean(resourceLabels.trim())}
              testBusy={dockerDraftTestBusy}
              testStatus={dockerDraftTest?.status ?? ''}
              testMessage={dockerDraftTest?.message ?? ''}
              testError={dockerDraftTest?.error ?? ''}
              testLatency={dockerDraftTest?.latency}
              testToolCount={dockerDraftTest?.toolCount ?? 0}
              onSubmit={() => void (editingDockerResourceId ? updateDockerFromWorkflow() : createDockerFromWorkflow())}
            />
          {:else if resourceKind === 'Kubernetes' && resourceAddStep === 2}
            <KubernetesConnectionStep isAgent={kubernetesIsAgent()} bind:connectionOverride={kubernetesConnectionOverride} bind:connectionMode={kubernetesConnectionMode} bind:server={kubernetesServer} bind:caBase64={kubernetesCABase64} bind:token={kubernetesToken} bind:certBase64={kubernetesCertBase64} bind:keyBase64={kubernetesKeyBase64} bind:kubeconfig={kubernetesKubeconfig} bind:skipTLSVerify={kubernetesSkipTLSVerify} bind:mcpServerResourceId={kubernetesMCPServerResourceId} mcpServers={kubernetesMCPServers} configurationAttempted={kubernetesConfigurationAttempted} credentialLoading={kubernetesCredentialLoading} onConfigurationChange={() => kubernetesDraftTest=null} />
          {:else if resourceKind === 'Kubernetes' && resourceAddStep === 3}
            <KubernetesReviewStep resourceName={resourceName} resourceStatus={resourceStatus} isAgent={kubernetesIsAgent()} connectionOverride={kubernetesConnectionOverride} connectionMode={kubernetesConnectionMode} server={kubernetesServer} mcpServerName={resources.find((r) => r.id === kubernetesMCPServerResourceId)?.name ?? ''} credentialConfigured={Boolean(kubernetesKubeconfig.trim() || kubernetesToken.trim() || kubernetesCABase64.trim() || kubernetesCertBase64.trim() || kubernetesKeyBase64.trim())} scopeSummary={activeScopeSummary()} labelsConfigured={Boolean(resourceLabels.trim())} testBusy={kubernetesDraftTestBusy} testStatus={kubernetesDraftTest?.status ?? ''} testMessage={kubernetesDraftTest?.message ?? ''} testError={kubernetesDraftTest?.error ?? ''} testLatency={kubernetesDraftTest?.latency} onSubmit={() => void (editingKubernetesResourceId ? updateKubernetesFromWorkflow() : createKubernetesFromWorkflow())} />
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
              ><McpConnectionFields
                bind:url={mcpURL}
                bind:token={mcpToken}
                bind:requestHeaders={mcpRequestHeaders}
                bind:toolAllowlist={mcpToolAllowlist}
                bind:timeoutSeconds={mcpTimeoutSeconds}
                bind:maxResponseBytes={mcpMaxResponseBytes}
                tokenPlaceholder="留空保持原凭据"
              />
            </div>
          {:else}
            <form
              id="resource-create-form"
              class="stack-form resource-create-form"
              on:submit|preventDefault={createResource}
            >
              <ResourceSchemaFields
                schema={createSchema}
                showTimeout={resourceSupportsEndpointTimeout(resourceKind)}
                bind:timeoutSeconds={genericTimeoutSeconds}
                bind:values={resourceConfigValues}
                bind:sensitiveValues={resourceSensitiveValues}
                bind:rawConfig={resourceConfig}
                isRequired={resourceSchemaFieldRequired}
                configurationAttempted={resourceBasicConfigurationAttempted}
              />
            </form>
          {/if}
      </ResourceWorkflowPanel>
    {/if}
    <ResourceDetailPanel
      {selectedResource}
      {selectedSchema}
      {resourceEditorOpen}
      {selectedResourceCanDelete}
      {selectedResourceHasConnector}
      {selectedResourceCanUpdate}
      {busy}
      {resourceActionBusy}
      {connectionBusy}
      {connectionCheck}
      {relationBusy}
      {relations}
      {topology}
      {resources}
      bind:relationTarget
      bind:relationType
      bind:providerType
      bind:providerProtocol
      bind:providerBaseURL
      bind:providerTimeoutSeconds
      bind:providerMaxConcurrency
      bind:providerRateLimitPerMinute
      bind:providerPurposeTags
      bind:providerModels
      bind:providerModelDraft
      bind:editingProviderModelName
      bind:providerDefaultModel
      {providerTypeOptions}
      {providerPurposeOptions}
      {providerCapabilityOptions}
      bind:mcpURL
      bind:mcpToken
      bind:mcpRequestHeaders
      bind:mcpToolAllowlist
      bind:mcpTimeoutSeconds
      bind:mcpMaxResponseBytes
      bind:editResourceName
      bind:editResourceStatus
      bind:editResourceLabels
      bind:editResourceConfig
      bind:resourceConfigValues
      bind:editResourceSensitiveValues
      bind:genericTimeoutSeconds
      {capabilityName}
      {formatDate}
      {scopeName}
      {resourceCategoryFor}
      {resourceSubtypeFor}
      {resourceSchemaFieldRequired}
      onDelete={deleteSelectedResource}
      onTestConnection={testSelectedResourceConnection}
      onUpdate={updateSelectedResource}
      onTogglePurpose={toggleProviderPurpose}
      onToggleCapability={toggleProviderModelCapability}
      onAddModel={addProviderModel}
      onSetDefault={setProviderDefaultModel}
      onEditModel={editProviderModel}
      onRemoveModel={removeProviderModel}
      onCreateRelation={createRelation}
      onDeleteRelation={deleteRelation}
    />
  </section>
</section>
