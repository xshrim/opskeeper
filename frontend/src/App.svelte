<script lang="ts">
import { onMount } from 'svelte';
  import {
    Boxes,
    Bot,
    Building2,
    ChevronLeft,
    ChevronRight,
    ChevronDown,
    ClipboardCheck,
    CloudDownload,
    Copy,
    Eye,
    EyeOff,
    FolderKanban,
    LayoutDashboard,
    Link2,
    LogOut,
    Monitor,
    Moon,
    PanelLeftClose,
    PanelLeftOpen,
    Paperclip,
    Pencil,
    PlugZap,
    Plus,
    RefreshCw,
    ScanSearch,
    Search,
    Send,
    ShieldCheck,
    Sparkles,
    Square,
    Stethoscope,
    Sun,
    Trash2,
    Upload,
    UserRound,
    UsersRound,
    icons as lucideIcons
  } from 'lucide-svelte';
  import { fetchHealth, toStatusRows, type HealthReport } from './lib/health';
  import BrandIcon from './lib/BrandIcon.svelte';
  import MessageBanner from './lib/MessageBanner.svelte';
  import {
    api,
    ApiError,
    type ConnectionCheck,
    type ConnectorCapability,
    type DiagnosisEvidence,
    type DiagnosisMessage,
    type DiagnosisSession,
    type DiagnosisSnapshot,
    type DiagnosisStatus,
    type DiscoveryItem,
    type DiscoveryProjectMapping,
    type DiscoveryRun,
    type Group,
    type Platform,
    type Project,
    type Relation,
    type Resource,
    type ResourceRoleBinding,
    type ResourceRoleDefinition,
    type ResourceSchema,
    type RoleBinding,
    type RoleDefinition,
    type Team,
    type AIConnectionResult,
    type AIProviderAvailability,
    type InspectionPolicy,
    type InspectionRun,
    type InspectionFinding,
    type NotificationChannel,
    type OperationRequest,
    type MCPSnapshot,
    type SkillVersion,
    type AgentProfileVersion,
    type TopologyNode,
    type User,
    type UserPreferences
  } from './lib/api';

  type View =
    | 'overview'
    | 'organization'
    | 'discovery'
    | 'resources'
    | 'skill'
    | 'agent'
    | 'operations'
    | 'diagnosis'
    | 'inspection'
    | 'access'
    | 'profile';
  type Theme = UserPreferences['theme'];
  type SidebarMode = UserPreferences['sidebar_mode'];
  type AccessTab = 'teams' | 'users' | 'roles';
  type DisableTarget = { kind: 'team' | 'user'; ids: string[] };
  type ProjectMappingDraft = DiscoveryProjectMapping & {
    mode: 'existing' | 'create' | 'ignore';
  };
  type ScopeChoice = {
    id: string;
    type: string;
    name: string;
    parentId?: string;
  };
  type AIProviderBindingSummary = {
    scope_id: string;
    provider_resource_id: string;
    tag: string;
  };
  type NewUserResourceGrant = {
    resourceID: string;
    roleID: string;
  };
  type NewUserGrant = {
    scopeType: 'platform' | 'team' | 'project';
    scopeID: string;
    roleID: string;
    resourceGrants: NewUserResourceGrant[];
  };
  type TeamIconOption = {
    value: string;
    label: string;
    keywords: string;
  };
  type SkillToolOption = {
    name: string;
    title: string;
    description: string;
    inputSchema: Record<string, unknown>;
  };
  type ProviderModelDraft = {
    name: string;
    contextWindowTokens: number;
    maxOutputTokens: number;
    temperature: number;
    temperatureMutable: boolean;
    capabilities: string[];
    enabled: boolean;
    priority: number;
  };
  const commonTeamIconLabels: Record<string, string> = {
    Boxes: '平台',
    UsersRound: '团队',
    FolderKanban: '项目',
    AppWindow: '应用',
    Waypoints: '接口',
    Building2: '组织',
    Cloud: '云服务',
    Network: '访问入口',
    ServerCog: '中间件',
    Database: 'PostgreSQL',
    DatabaseZap: 'Redis',
    Radio: 'Kafka',
    ChartNoAxesCombined: '指标',
    FileText: '日志',
    GitBranch: '链路',
    ScanSearch: '可观测',
    Bell: '通知',
    Clock: '计划',
    Search: '检索',
    BookOpen: '手册',
    Sparkles: 'Skill',
    BrainCircuit: 'AI 接入',
    Bot: 'MCP',
    HardDrive: '存储',
    KeyRound: '凭据',
    Package: '资源'
  };

  const teamIconOptions: TeamIconOption[] = Object.keys(lucideIcons)
    .sort((left, right) => left.localeCompare(right))
    .map((value) => ({
      value,
      label: commonTeamIconLabels[value] ?? formatIconName(value),
      keywords: `${value} ${formatIconName(value)} ${commonTeamIconLabels[value] ?? ''}`
    }));

  const providerTypeOptions = [
    { value: 'openai_compatible', label: 'OpenAI 兼容', baseURL: '' },
    { value: 'openai', label: 'OpenAI', baseURL: 'https://api.openai.com/v1' },
    {
      value: 'anthropic',
      label: 'Anthropic',
      baseURL: 'https://api.anthropic.com/v1'
    },
    {
      value: 'gemini',
      label: 'Gemini',
      baseURL: 'https://generativelanguage.googleapis.com/v1beta/openai'
    },
    { value: 'grok', label: 'Grok', baseURL: 'https://api.x.ai/v1' },
    {
      value: 'deepseek',
      label: 'DeepSeek',
      baseURL: 'https://api.deepseek.com/v1'
    },
    {
      value: 'qwen',
      label: 'Qwen',
      baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1'
    },
    { value: 'kimi', label: 'Kimi', baseURL: 'https://api.moonshot.cn/v1' },
    {
      value: 'glm',
      label: 'GLM',
      baseURL: 'https://open.bigmodel.cn/api/paas/v4'
    },
    {
      value: 'minimax',
      label: 'MiniMax',
      baseURL: 'https://api.minimaxi.com/v1'
    },
    { value: 'mimo', label: 'MiMo', baseURL: 'https://api.xiaomimimo.com/v1' },
    { value: 'longcat', label: 'LongCat', baseURL: '' },
    {
      value: 'doubao',
      label: 'Doubao',
      baseURL: 'https://ark.cn-beijing.volces.com/api/v3'
    },
    {
      value: 'openrouter',
      label: 'OpenRouter',
      baseURL: 'https://openrouter.ai/api/v1'
    },
    {
      value: 'siliconflow',
      label: 'SiliconFlow',
      baseURL: 'https://api.siliconflow.cn/v1'
    },
    { value: 'ollama', label: 'Ollama', baseURL: 'http://localhost:11434/v1' }
  ];
  const providerCapabilityOptions = [
    { value: 'text', label: '文本' },
    { value: 'vision', label: '视觉' },
    { value: 'audio', label: '音频' },
    { value: 'tool_calling', label: '工具调用' },
    { value: 'structured_output', label: '结构化输出' },
    { value: 'stream', label: '流式输出' },
    { value: 'deep_thinking', label: '深度思考' }
  ];
  const providerPurposeOptions = [
    { value: 'default', label: '默认', requiredCapabilities: ['text'] },
    {
      value: 'diagnosis',
      label: '诊断',
      requiredCapabilities: ['text', 'tool_calling', 'stream']
    },
    {
      value: 'inspection',
      label: '巡检',
      requiredCapabilities: ['text', 'tool_calling', 'structured_output']
    },
    {
      value: 'workflow',
      label: '工作流',
      requiredCapabilities: ['text', 'tool_calling', 'structured_output']
    }
  ];

  const skillToolOptions: SkillToolOption[] = [
    {
      name: 'connector_kubernetes_read',
      title: 'Kubernetes 只读查询',
      description: '查询目标 Kubernetes 资源；不会修改集群。',
      inputSchema: {
        type: 'object',
        required: ['resource'],
        properties: {
          target_resource_id: { type: 'string' },
          resource: { type: 'string' },
          namespace: { type: 'string' },
          name: { type: 'string' },
          label_selector: { type: 'string' },
          limit: { type: 'integer' }
        },
        additionalProperties: false
      }
    },
    {
      name: 'connector_metrics_query',
      title: '指标查询',
      description: '通过已关联的 Prometheus 查询时间序列指标。',
      inputSchema: {
        type: 'object',
        required: ['query', 'start', 'end', 'step_seconds'],
        properties: {
          target_resource_id: { type: 'string' },
          query: { type: 'string' },
          start: { type: 'string', format: 'date-time' },
          end: { type: 'string', format: 'date-time' },
          step_seconds: { type: 'integer' }
        },
        additionalProperties: false
      }
    },
    {
      name: 'connector_logs_query',
      title: '日志查询',
      description: '通过已关联的 Loki 查询限定范围内的日志。',
      inputSchema: {
        type: 'object',
        required: ['query', 'start', 'end', 'limit'],
        properties: {
          target_resource_id: { type: 'string' },
          query: { type: 'string' },
          start: { type: 'string', format: 'date-time' },
          end: { type: 'string', format: 'date-time' },
          limit: { type: 'integer' }
        },
        additionalProperties: false
      }
    },
    {
      name: 'connector_traces_query',
      title: '链路查询',
      description: '通过已关联的追踪平台查询服务调用链。',
      inputSchema: {
        type: 'object',
        required: ['service', 'start', 'end', 'limit'],
        properties: {
          target_resource_id: { type: 'string' },
          service: { type: 'string' },
          operation: { type: 'string' },
          start: { type: 'string', format: 'date-time' },
          end: { type: 'string', format: 'date-time' },
          limit: { type: 'integer' }
        },
        additionalProperties: false
      }
    },
    {
      name: 'connector_alerts_get',
      title: '告警查询',
      description: '查询目标关联监控平台中的当前告警。',
      inputSchema: {
        type: 'object',
        properties: {
          target_resource_id: { type: 'string' },
          active_only: { type: 'boolean' }
        },
        additionalProperties: false
      }
    }
  ];

  let authState: 'loading' | 'login' | 'ready' = 'loading';
  let currentUser: User | null = null;
  let loginIdentifier = '';
  let password = '';
  let passwordVisible = false;
  let loginError = '';
  let loginErrorTimer: number | null = null;
  let notice = '';
  let noticeTimer: number | null = null;
  let errorMessage = '';
  let errorTimer: number | null = null;
  let activeMessage = '';
  let activeMessageTone: 'success' | 'error' = 'success';
  let messageInChildSurface = false;
  let busy = false;
  let view: View = 'overview';
  let preferences: UserPreferences = {
    theme: 'auto',
    sidebar_mode: 'fixed',
    sidebar_collapsed: false
  };
  let sidebarHovered = false;
  let previousSidebarCompact = false;
  let userMenuOpen = false;
  let teamMenuOpen = false;
  let accessMenuOpen = false;
  let isPlatformAdmin = false;
  let hasPlatformRole = false;
  let selectedTeamId = '';
  let selectedProjectId = '';
  let profileDisplayName = '';
  let profileEmail = '';
  let profilePhone = '';
  let profileCurrentPassword = '';
  let profileNewPassword = '';
  let profileConfirmPassword = '';
  let requiredNewPassword = '';
  let requiredConfirmPassword = '';
  let requiredNewPasswordVisible = false;
  let requiredConfirmPasswordVisible = false;
  let copiedControl:
    'created-password' | 'reset-username' | 'reset-credentials' | null = null;
  let copiedControlTimer: number | null = null;
  let avatarBusy = false;
  let platform: Platform | null = null;
  let teams: Team[] = [];
  let projects: Project[] = [];
  let resources: Resource[] = [];
  let contextResources: Resource[] = [];
  let aiProviderBindings: AIProviderBindingSummary[] = [];
  let schemas: ResourceSchema[] = [];
  let health: HealthReport | null = null;
  let healthController: AbortController | null = null;
  let healthInterval: number | null = null;
  let selectedScopeId = '';
  let selectedResourceId = '';
  let relations: Relation[] = [];
  let topology: TopologyNode[] = [];
  let connectionCheck: ConnectionCheck | null = null;
  let connectionBusy = false;
  let resourceConnectionChecks: Record<string, ConnectionCheck | null> = {};
  let users: User[] = [];
  let groups: Group[] = [];
  let groupMembers: Record<string, string[]> = {};
  let roles: RoleDefinition[] = [];
  let bindings: RoleBinding[] = [];
  let resourceRoles: ResourceRoleDefinition[] = [];
  let resourceBindings: ResourceRoleBinding[] = [];
  let aiLoaded = false;
  let selectedProviderId = '';
  let selectedSkillId = '';
  let selectedSkillVersionId = '';
  let llmModelName = '';
  let skillVersions: SkillVersion[] = [];
  let skillInstruction = '';
  let skillTargetKinds = 'Application';
  let skillInputSchema = '{"type":"object","additionalProperties":true}';
  let skillOutputSchema = '{"type":"object","additionalProperties":true}';
  let skillInput = '{}';
  let selectedSkillToolNames: string[] = [];
  let agentProfileResources: Resource[] = [];
  let selectedAgentProfileId = '';
  let selectedAgentProfileVersionId = '';
  let agentProfileVersions: AgentProfileVersion[] = [];
  let agentProfileName = '';
  let agentProfileInstruction = '';
  let agentProfileCapabilities = 'text, tool_calling, stream';
  let agentProfileAllowedTools = '';
  let agentProfileTargetKinds = 'Application';
  let agentProfileInputSchema = '{"type":"object","additionalProperties":true}';
  let agentProfileOutputSchema =
    '{"type":"object","additionalProperties":true}';
  let diagnosisLoaded = false;
  let diagnosisSessions: DiagnosisSession[] = [];
  let diagnosisAvailableProviders: AIProviderAvailability[] = [];
  let selectedDiagnosisId = '';
  let diagnosisSnapshot: DiagnosisSnapshot | null = null;
  let diagnosisQuestion = '';
  let diagnosisComposerText = '';
  let diagnosisTargetIds: string[] = [];
  let diagnosisFollowup = '';
  let selectedEvidence: DiagnosisEvidence | null = null;
  let diagnosisHistoryWidth = 232;
  let diagnosisContextWidth = 275;
  let diagnosisHistoryCollapsed = false;
  let diagnosisContextCollapsed = false;
  let diagnosisContextTab: 'context' | 'evidence' = 'context';
  let diagnosisSessionSearch = '';
  let diagnosisEditingMessageId = '';
  let diagnosisEditDraft = '';
  let diagnosisInterruptedReason = '';
  let diagnosisGenerating = false;
  let diagnosisStreamingText = '';
  let diagnosisStreamingPending = '';
  let diagnosisStreamingFrame = 0;
  let diagnosisStreamingStartedAt = 0;
  let diagnosisActionExpanded: Record<string, boolean> = {};
  let diagnosisActionChildren: Record<string, boolean> = {};
  let diagnosisHiddenMessageIds: string[] = [];
  let diagnosisEvents: EventSource | null = null;
  let diagnosisRefreshToken = 0;
  let inspectionLoaded = false;
  let inspectionPolicies: InspectionPolicy[] = [];
  let inspectionRuns: InspectionRun[] = [];
  let inspectionFindings: InspectionFinding[] = [];
  let notificationChannels: NotificationChannel[] = [];
  let inspectionPolicyName = '';
  let inspectionCron = '0 * * * *';
  let inspectionTimezone =
    Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  let inspectionTargetIds: string[] = [];
  let inspectionAgentProfileId = '';
  let inspectionTargetLabels = '{}';
  let inspectionTimeoutSeconds = 120;
  let inspectionRetries = 1;
  let inspectionMaxConcurrent = 2;
  let inspectionMaxToolCalls = 12;
  let inspectionMaxTokens = 20000;
  let channelName = '';
  let channelWebhookURL = '';
  let channelRateLimit = 30;
  let operationsLoaded = false;
  let operationRequests: OperationRequest[] = [];
  let operationTargetId = '';
  let operationName = 'kubernetes.restart_workload';
  let operationRisk: 'low' | 'medium' | 'high' = 'medium';
  let operationParameters = '{\n  "namespace": "default",\n  "workload": ""\n}';
  let operationImpact = '';
  let operationRollback = '';
  let operationSnapshots: Record<string, MCPSnapshot[]> = {};
  let diagnosisEventCursor = 0;
  let accessLoaded = false;
  let discoveryLoaded = false;
  let selectedClusterId = '';
  let discoveryRuns: DiscoveryRun[] = [];
  let activeDiscovery: DiscoveryRun | null = null;
  let discoveryItems: DiscoveryItem[] = [];
  let selectedDiscoveryItems: Record<string, boolean> = {};
  let projectMappingDrafts: Record<string, ProjectMappingDraft> = {};

  let teamName = '';
  let teamCode = '';
  let teamIcon = 'UsersRound';
  let projectTeamId = '';
  let projectName = '';
  let projectCode = '';
  let projectIcon = 'FolderKanban';
  let resourceKind = '';
  let resourceCategory = '全部';
  let resourceSubtype = '全部';
  let resourceSearch = '';
  let resourceStatusFilter = 'all';
  let resourceLevelFilter = 'all';
  let expandedResourceCategory = '';
  let resourceEditorOpen = false;
  let resourceAddMenuOpen = false;
  let resourceAddStep = 1;
  let resourceAddCategory = '';
  let resourceAddSubtype = '';
  let providerType = 'openai_compatible';
  let providerProtocol = 'chat_completions';
  let providerBaseURL = '';
  let providerAPIKey = '';
  let providerAPIKeyVisible = false;
  let providerAPIKeyLoading = false;
  let providerTimeoutSeconds = 60;
  let providerMaxConcurrency = 5;
  let providerRateLimitPerMinute = 0;
  let providerModels: ProviderModelDraft[] = [];
  let providerModelDraft: ProviderModelDraft = emptyProviderModelDraft();
  let editingProviderModelName = '';
  let providerDefaultModel = '';
  let providerPurposeTags: string[] = [];
  let editingProviderResourceId = '';
  let mcpTransport = 'streamable_http';
  let mcpURL = '';
  let mcpToolAllowlist = '';
  let mcpTimeoutSeconds = 10;
  let mcpMaxResponseBytes = 1048576;
  let providerConfigurationAttempted = false;
  let providerModelConfigurationAttempted = false;
  let providerModelValidationMessage = '';
  let providerSummaryAttempted = false;
  let resourceTypeSelectionAttempted = false;
  let resourceBasicConfigurationAttempted = false;
  let providerDraftTest: {
    signature: string;
    result?: AIConnectionResult;
    error?: string;
  } | null = null;
  let providerDraftTestBusy = false;
  let providerDraftTestPassedState = false;
  let providerDraftCurrentSignature = '';
  $: providerDraftCurrentSignature = JSON.stringify({
    scope: selectedScopeId,
    providerType,
    baseURL: providerBaseURL.trim(),
    apiKey: providerAPIKey,
    model: (() => {
      const defaultModel = providerModels.find(
        (model) => model.name === providerDefaultModel
      );
      return defaultModel
        ? {
            name: defaultModel.name,
            contextWindowTokens: defaultModel.contextWindowTokens,
            temperature: defaultModel.temperature,
            capabilities: defaultModel.capabilities
          }
        : null;
    })()
  });
  $: providerDraftTestPassedState = Boolean(
    providerDraftTest?.signature === providerDraftCurrentSignature &&
      providerDraftTest.result?.status === 'succeeded'
  );
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
  let relationTarget = '';
  let relationType = 'depends_on';
  let groupScopeId = '';
  let groupName = '';
  let groupDescription = '';
  let bindingSubjectType = 'user';
  let bindingSubjectId = '';
  let bindingRoleId = '';
  let bindingScopeId = '';
  let resourceBindingSubjectType = 'user';
  let resourceBindingSubjectId = '';
  let resourceBindingRoleId = '';
  let resourceBindingResourceId = '';
  let accessSearch = '';
  let accessTab: AccessTab = 'teams';
  let accessLoading = false;
  let accessLoadError = '';
  let teamAccessExpanded: Record<string, boolean> = {};
  let selectedAccessTeamIds: string[] = [];
  let selectedAccessUserIds: string[] = [];
  let teamDialogOpen = false;
  let iconPickerTarget: 'create' | 'edit' | null = null;
  let teamIconSearch = '';
  let userDialogOpen = false;
  let editingTeam: Team | null = null;
  let editTeamName = '';
  let editTeamIcon = '';
  let editTeamStatus = 'active';
  let editingUser: User | null = null;
  let editUserDisplayName = '';
  let editUserScopeId = '';
  let editUserRoleIds: string[] = [];
  let editUserResourceRoleId = '';
  let editUserResourceId = '';
  let disableTarget: DisableTarget | null = null;
  let newUserUsername = '';
  let newUserEmail = '';
  let newUserPhone = '';
  let newUserDisplayName = '';
  let newUserPassword = '';
  let newUserPasswordMode: 'manual' | 'generated' = 'generated';
  let createdUserCredentials: { username: string; password: string } | null =
    null;
  let passwordResetCredentials: { username: string; password: string } | null =
    null;
  let newUserGrants: NewUserGrant[] = [];

  $: scopeChoices = buildScopeChoices(platform, teams, projects);
  $: activeScope =
    scopeChoices.find((scope) => scope.id === selectedScopeId) ??
    scopeChoices[0];
  $: selectedTeam = teams.find((team) => team.id === selectedTeamId) ?? null;
  $: selectedTeamProjects = projects.filter(
    (project) => project.team_id === selectedTeamId
  );
  $: selectedProject =
    selectedTeamProjects.find((project) => project.id === selectedProjectId) ??
    null;
  $: workspaceProjects = selectedTeamId
    ? selectedTeamProjects
    : hasPlatformRole
      ? projects
      : [];
  $: visibleProjects = selectedScopeId
    ? activeScope?.type === 'platform'
      ? projects
      : activeScope?.type === 'team'
        ? selectedTeamProjects
        : projects.filter((project) => project.scope.id === selectedScopeId)
    : projects;
  $: visibleResources = selectedScopeId
    ? resources.filter((resource) => resourceInActiveWorkspace(resource))
    : resources;
  $: resourceCatalogItems = visibleResources.filter((resource) => {
    if (
      resourceCategory !== '全部' &&
      resourceCategoryFor(resource) !== resourceCategory
    )
      return false;
    if (
      resourceSubtype !== '全部' &&
      resourceSubtypeFor(resource) !== resourceSubtype
    )
      return false;
    if (
      resourceStatusFilter !== 'all' &&
      resource.status !== resourceStatusFilter
    )
      return false;
    if (
      resourceLevelFilter !== 'all' &&
      scopeType(resource.scope_id) !== resourceLevelFilter
    )
      return false;
    const query = resourceSearch.trim().toLowerCase();
    return (
      !query ||
      `${resource.name} ${resource.kind} ${resourceSchemaName(resource.kind)} ${scopeName(resource.scope_id)} ${resourceLabelsText(resource)}`
        .toLowerCase()
        .includes(query)
    );
  });
  $: selectedResource =
    resources.find((resource) => resource.id === selectedResourceId) ?? null;
  $: selectedResourceCanUpdate = selectedResource
    ? resourceCanManage(selectedResource, 'resource:update')
    : false;
  $: selectedResourceCanDelete = selectedResource
    ? resourceCanManage(selectedResource, 'resource:delete')
    : false;
  $: rows = toStatusRows(health);
  $: selectedSchema = schemas.find(
    (schema) => schema.kind === selectedResource?.kind
  );
  $: selectedResourceHasConnector = Boolean(
    selectedResource &&
    ['AIProvider', 'Kubernetes', 'Prometheus', 'Loki'].includes(
      selectedResource.kind
    )
  );
  $: createSchema = schemas.find((schema) => schema.kind === resourceKind);
  $: resourceAddSubtypeOptions = resourceAddCategory
    ? (resourceCategoryOptions[resourceAddCategory] ?? [])
    : [];
  $: kubernetesClusters = resources.filter(
    (resource) => resource.kind === 'Kubernetes'
  );
  $: namespaceCandidates = discoveryItems.filter(
    (item) => item.kind === 'Project'
  );
  $: applicationCandidates = discoveryItems.filter(
    (item) => item.kind === 'Application'
  );
  $: if (view === 'diagnosis' && !llmModelName && selectedProviderId) {
    const available = diagnosisAvailableProviders.find(
      (item) => item.provider_resource_id === selectedProviderId
    );
    llmModelName = String(available?.models[0]?.name ?? '');
  }
  $: diagnosisProviderModels = (() => {
    const available = diagnosisAvailableProviders.find(
      (item) => item.provider_resource_id === selectedProviderId
    );
    if (available) return available.models;
    return [];
  })();
  $: skillResources = resources.filter((item) => item.kind === 'Skill');
  $: agentProfileResources = resources.filter(
    (item) => item.kind === 'AgentProfile'
  );
  $: executableTargets = visibleResources.filter(
    (item) => item.kind !== 'AIProvider' && item.kind !== 'Skill'
  );
  $: diagnosisTargets = contextResources.filter(
    (item) => item.status === 'active' && resourceInActiveWorkspace(item)
  );
  $: sidebarCompact =
    preferences.sidebar_mode === 'hover'
      ? !sidebarHovered
      : preferences.sidebar_collapsed;
  $: if (sidebarCompact && !previousSidebarCompact) {
    accessMenuOpen = false;
  }
  $: previousSidebarCompact = sidebarCompact;
  $: avatarURL = preferences.avatar_updated_at
    ? api.avatarURL(preferences.avatar_updated_at)
    : '';
  $: visibleAccessUsers = users.filter((user) => {
    const query = accessSearch.trim().toLowerCase();
    if (!query) return true;
    return [user.display_name, user.username, user.email, user.phone]
      .filter(Boolean)
      .some((value) => value.toLowerCase().includes(query));
  });
  $: visibleAccessTeams = teams.filter((team) => {
    const query = accessSearch.trim().toLowerCase();
    if (!query) return true;
    return [team.name, team.code, team.status].some((value) =>
      value.toLowerCase().includes(query)
    );
  });
  $: filteredTeamIconOptions = teamIconOptions.filter((option) => {
    const query = teamIconSearch.trim().toLowerCase();
    return (
      !query ||
      `${option.label} ${option.keywords}`.toLowerCase().includes(query)
    );
  });
  $: manageableScopeChoices = isPlatformAdmin
    ? scopeChoices
    : scopeChoices.filter((scope) =>
        actorPermissionsAtScope(scope.id).includes('member:grant')
      );
  $: availableEditUserRoles = grantableRolesForScope(editUserScopeId);
  $: editingScopeViewer = Boolean(
    editingUser &&
    editUserRoleIds.some(
      (roleID) =>
        roles.find((role) => role.id === roleID)?.name ===
        resourceGrantViewerRole(scopeType(editUserScopeId))
    )
  );
  $: scopeViewerResources = resources.filter(
    (resource) =>
      resourceVisibleToScope(editUserScopeId, resource.scope_id) &&
      resource.status === 'active'
  );
  $: availableScopeViewerResourceRoles = resourceRoles.filter(
    (resourceRole) =>
      viewerResourceRoleAllowed(resourceRole) &&
      resourceRole.permissions.every((permission) =>
        actorPermissionsAtScope(editUserScopeId).includes(String(permission))
      )
  );
  $: scopeViewerResourceBindings = editingUser
    ? resourceBindings.filter(
        (binding) =>
          binding.subject_type === 'user' &&
          binding.subject_id === editingUser?.id &&
          binding.scope_id === editUserScopeId
      )
    : [];
  $: accessCanManageUsers = manageableScopeChoices.length > 0;
  $: accessCanCreateUser =
    isPlatformAdmin ||
    accessCanManageUsers ||
    userRoleBindings(currentUser?.id ?? '').some((binding) =>
      ['PlatformAdmin', 'TeamAdmin', 'ProjectAdmin'].includes(binding.role_name)
    );
  $: accessCanCreateTeam = isPlatformAdmin;
  $: accessTeamUsers = Object.fromEntries(
    teams.map((team) => {
      const teamScopeIDs = new Set([
        team.scope.id,
        ...projects
          .filter((project) => project.team_id === team.id)
          .map((project) => project.scope.id)
      ]);
      const memberIDs = groups
        .filter((group) => teamScopeIDs.has(group.scope_id))
        .flatMap((group) => groupMembers[group.id] ?? []);
      const roleIDs = bindings
        .filter(
          (binding) =>
            teamScopeIDs.has(binding.scope_id) &&
            binding.subject_type === 'user'
        )
        .map((binding) => binding.subject_id);
      const visibleIDs = new Set([...memberIDs, ...roleIDs]);
      return [team.id, users.filter((user) => visibleIDs.has(user.id))];
    })
  ) as Record<string, User[]>;

  onMount(() => {
    void bootstrap();
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const refreshTheme = () => applyTheme();
    media.addEventListener('change', refreshTheme);
    document.addEventListener('pointerdown', handleDocumentPointerDown);
    return () => {
      if (noticeTimer !== null) window.clearTimeout(noticeTimer);
      if (errorTimer !== null) window.clearTimeout(errorTimer);
      if (loginErrorTimer !== null) window.clearTimeout(loginErrorTimer);
      stopHealthPolling();
      closeDiagnosisEvents();
      media.removeEventListener('change', refreshTheme);
      document.removeEventListener('pointerdown', handleDocumentPointerDown);
    };
  });

  $: if (notice) {
    if (noticeTimer !== null) window.clearTimeout(noticeTimer);
    noticeTimer = window.setTimeout(() => {
      notice = '';
      noticeTimer = null;
    }, 5_000);
  }

  $: if (errorMessage) {
    if (errorTimer !== null) window.clearTimeout(errorTimer);
    errorTimer = window.setTimeout(() => {
      errorMessage = '';
      errorTimer = null;
    }, 5_000);
  }

  $: if (loginError) {
    if (loginErrorTimer !== null) window.clearTimeout(loginErrorTimer);
    loginErrorTimer = window.setTimeout(() => {
      loginError = '';
      loginErrorTimer = null;
    }, 5_000);
  }

  $: activeMessage = errorMessage || notice;
  $: activeMessageTone = errorMessage ? 'error' : 'success';
  $: messageInChildSurface = resourceAddMenuOpen || teamDialogOpen || userDialogOpen || Boolean(editingTeam) || Boolean(editingUser) || Boolean(iconPickerTarget) || Boolean(disableTarget);

  function startHealthPolling() {
    if (healthInterval !== null) return;
    const controller = new AbortController();
    healthController = controller;
    const checkHealth = async () => {
      try {
        health = await fetchHealth(controller.signal);
      } catch {
        if (healthController === controller) health = null;
      }
    };
    void checkHealth();
    healthInterval = window.setInterval(checkHealth, 15_000);
  }

  function stopHealthPolling() {
    healthController?.abort();
    healthController = null;
    if (healthInterval !== null) {
      window.clearInterval(healthInterval);
      healthInterval = null;
    }
    health = null;
  }

  async function bootstrap() {
    try {
      const [user, sessionContext] = await Promise.all([
        api.me(),
        api.sessionContext()
      ]);
      currentUser = user;
      if (user.must_change_password) {
        authState = 'ready';
        return;
      }
      isPlatformAdmin = sessionContext.platform_admin;
      hasPlatformRole = sessionContext.platform_role;
      authState = 'ready';
      await loadPreferences();
      startHealthPolling();
      await loadWorkspace();
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        authState = 'login';
      } else {
        authState = 'login';
        loginError = '无法恢复会话，请重新登录。';
      }
    }
  }

  async function loadWorkspace() {
    errorMessage = '';
    try {
      const [loadedPlatform, teamPage, loadedSchemas, resourcePage] =
        await Promise.all([
          api.platform(),
          api.teams(),
          api.schemas(),
          api.resources()
        ]);
      platform = loadedPlatform;
      teams = teamPage.items;
      schemas = loadedSchemas;
      resources = resourcePage.items;
      const contextResourceResult = await Promise.allSettled([
        api.contextResources()
      ]);
      const contextPage = contextResourceResult[0];
      contextResources =
        contextPage.status === 'fulfilled' ? contextPage.value.items : [];
      await loadResourceConnectionChecks(resources);
      const projectPages = await Promise.all(
        teams.map((team) => api.projects(team.id))
      );
      projects = projectPages.flatMap((page) => page.items);
      const scopeIDs = [
        loadedPlatform.scope.id,
        ...teams.map((team) => team.scope.id),
        ...projects.map((project) => project.scope.id)
      ];
      const bindingPages = await Promise.allSettled(
        [...new Set(scopeIDs)].map((scopeID) =>
          api.aiProviderBindings(scopeID)
        )
      );
      aiProviderBindings = bindingPages.flatMap((result) =>
        result.status === 'fulfilled' ? result.value : []
      );
      const defaultTeam = hasPlatformRole ? undefined : teams[0];
      selectedTeamId = defaultTeam?.id ?? '';
      projectTeamId = defaultTeam?.id ?? '';
      selectedProjectId = '';
      selectedScopeId = defaultTeam?.scope.id ?? platform.scope.id;
    } catch (error) {
      errorMessage = describeError(error, '工作区数据加载失败');
    }
  }

  async function login() {
    busy = true;
    loginError = '';
    try {
      currentUser = await api.login(loginIdentifier.trim(), password);
      if (currentUser.must_change_password) {
        password = '';
        passwordVisible = false;
        authState = 'ready';
        return;
      }
      const sessionContext = await api.sessionContext();
      isPlatformAdmin = sessionContext.platform_admin;
      hasPlatformRole = sessionContext.platform_role;
      password = '';
      passwordVisible = false;
      authState = 'ready';
      await loadPreferences();
      startHealthPolling();
      await loadWorkspace();
    } catch (error) {
      loginError = describeError(
        error,
        '登录失败，请检查用户名、邮箱、手机号和密码'
      );
    } finally {
      busy = false;
    }
  }

  async function logout() {
    busy = true;
    try {
      await api.logout();
    } finally {
      currentUser = null;
      isPlatformAdmin = false;
      hasPlatformRole = false;
      selectedTeamId = '';
      selectedProjectId = '';
      authState = 'login';
      stopHealthPolling();
      view = 'overview';
      userMenuOpen = false;
      busy = false;
    }
  }

  function chooseView(nextView: View) {
    view = nextView;
    notice = '';
    errorMessage = '';
    userMenuOpen = false;
    teamMenuOpen = false;
    if (nextView === 'access' && !accessLoaded) void loadAccess();
    if (nextView === 'discovery' && !discoveryLoaded) void loadDiscovery();
    if (nextView === 'skill' && !aiLoaded) void loadAI();
    if (nextView === 'agent' && !aiLoaded) void loadAI();
    if (nextView === 'diagnosis' && !diagnosisLoaded) void loadDiagnosis();
    if (nextView === 'inspection' && !inspectionLoaded) void loadInspection();
    if (nextView === 'operations' && !operationsLoaded) void loadOperations();
    if (nextView !== 'diagnosis') closeDiagnosisEvents();
  }

  function chooseAccessTab(tab: AccessTab) {
    accessTab = tab;
    accessSearch = '';
    accessMenuOpen = true;
    chooseView('access');
  }

  async function loadPreferences() {
    try {
      preferences = await api.preferences();
      applyTheme();
    } catch (error) {
      errorMessage = describeError(error, '个人偏好加载失败');
    }
  }

  function applyTheme() {
    if (typeof window === 'undefined') return;
    const isDark =
      preferences.theme === 'dark' ||
      (preferences.theme === 'auto' &&
        window.matchMedia('(prefers-color-scheme: dark)').matches);
    document.documentElement.dataset.theme = isDark ? 'dark' : 'light';
  }

  function openProfile() {
    profileDisplayName = currentUser?.display_name ?? '';
    profileEmail = currentUser?.email ?? '';
    profilePhone = currentUser?.phone ?? '';
    chooseView('profile');
  }

  async function saveProfile() {
    await action(async () => {
      currentUser = await api.updateProfile({
        display_name: profileDisplayName,
        email: profileEmail,
        phone: profilePhone
      });
      preferences = await api.updatePreferences({
        theme: preferences.theme,
        sidebar_mode: preferences.sidebar_mode,
        sidebar_collapsed: preferences.sidebar_collapsed
      });
      applyTheme();
      notice = '个人中心配置已保存';
    });
  }

  async function changeOwnPassword(required = false) {
    const currentPassword = profileCurrentPassword;
    const newPassword = required ? requiredNewPassword : profileNewPassword;
    const confirmPassword = required
      ? requiredConfirmPassword
      : profileConfirmPassword;
    if (newPassword !== confirmPassword) {
      errorMessage = '两次输入的新密码不一致。';
      return;
    }
    await action(async () => {
      currentUser = await api.changePassword({
        ...(required ? {} : { current_password: currentPassword }),
        new_password: newPassword
      });
      requiredNewPassword = '';
      requiredConfirmPassword = '';
      requiredNewPasswordVisible = false;
      requiredConfirmPasswordVisible = false;
      profileCurrentPassword = '';
      profileNewPassword = '';
      profileConfirmPassword = '';
      notice = '密码已更新';
      if (!required) return;
      const sessionContext = await api.sessionContext();
      isPlatformAdmin = sessionContext.platform_admin;
      hasPlatformRole = sessionContext.platform_role;
      await loadPreferences();
      startHealthPolling();
      await loadWorkspace();
    });
  }

  async function copyOneTimePassword() {
    if (!createdUserCredentials) return;
    try {
      await navigator.clipboard.writeText(createdUserCredentials.password);
      markCopySuccess('created-password');
      notice = '一次性密码已复制';
    } catch {
      errorMessage = '无法访问剪贴板，请手动复制一次性密码。';
    }
  }

  async function copyPasswordResetCredentials(includePassword: boolean) {
    if (!passwordResetCredentials) return;
    const value = includePassword
      ? `用户名：${passwordResetCredentials.username}\n一次性密码：${passwordResetCredentials.password}`
      : passwordResetCredentials.username;
    try {
      await navigator.clipboard.writeText(value);
      markCopySuccess(includePassword ? 'reset-credentials' : 'reset-username');
      notice = includePassword ? '用户名和一次性密码已复制' : '用户名已复制';
    } catch {
      errorMessage = '无法访问剪贴板，请手动复制凭据。';
    }
  }

  function markCopySuccess(control: NonNullable<typeof copiedControl>) {
    copiedControl = control;
    if (copiedControlTimer !== null) window.clearTimeout(copiedControlTimer);
    copiedControlTimer = window.setTimeout(() => {
      copiedControl = null;
      copiedControlTimer = null;
    }, 3000);
  }

  async function uploadAvatar(event: Event) {
    const file = (event.currentTarget as HTMLInputElement).files?.[0];
    if (!file) return;
    if (
      !['image/jpeg', 'image/png'].includes(file.type) ||
      file.size > 1 << 20
    ) {
      errorMessage = '头像仅支持不超过 1 MiB 的 PNG 或 JPEG 图片。';
      return;
    }
    avatarBusy = true;
    errorMessage = '';
    try {
      preferences = await api.updateAvatar(file);
      notice = '头像已更新';
    } catch (error) {
      errorMessage = describeError(error, '头像上传失败');
    } finally {
      avatarBusy = false;
      (event.currentTarget as HTMLInputElement).value = '';
    }
  }

  async function toggleSidebar() {
    if (preferences.sidebar_mode === 'hover') {
      preferences = {
        ...preferences,
        sidebar_mode: 'fixed',
        sidebar_collapsed: true
      };
    } else {
      preferences = {
        ...preferences,
        sidebar_collapsed: !preferences.sidebar_collapsed
      };
    }
    try {
      preferences = await api.updatePreferences({
        theme: preferences.theme,
        sidebar_mode: preferences.sidebar_mode,
        sidebar_collapsed: preferences.sidebar_collapsed
      });
    } catch (error) {
      errorMessage = describeError(error, '侧边导航设置保存失败');
    }
  }

  function handleGlobalKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      userMenuOpen = false;
      teamMenuOpen = false;
      accessMenuOpen = false;
      resourceAddMenuOpen = false;
      resourceEditorOpen = false;
    }
  }

  function handleDocumentPointerDown(event: PointerEvent) {
    const target = event.target;
    if (!(target instanceof Element)) return;
    if (userMenuOpen && !target.closest('.sidebar-user-menu')) {
      userMenuOpen = false;
    }
    if (teamMenuOpen && !target.closest('.workspace-team-wrap')) {
      teamMenuOpen = false;
    }
    if (accessMenuOpen && view !== 'access' && !target.closest('.nav-group')) {
      accessMenuOpen = false;
    }
  }

  function chooseTeam(teamID: string) {
    if (!teamID && hasPlatformRole) {
      selectedTeamId = '';
      projectTeamId = '';
      selectedProjectId = '';
      selectedScopeId = platform?.scope.id ?? '';
      selectedResourceId = '';
      return;
    }
    const team = teams.find((item) => item.id === teamID);
    if (!team) return;
    selectedTeamId = team.id;
    projectTeamId = team.id;
    selectedProjectId = '';
    selectedScopeId = team.scope.id;
    selectedResourceId = '';
    teamMenuOpen = false;
  }

  function chooseProject(projectID: string) {
    if (!projectID) {
      selectedProjectId = '';
      selectedScopeId = selectedTeam?.scope.id ?? platform?.scope.id ?? '';
      selectedResourceId = '';
      return;
    }
    const project = projects.find((item) => item.id === projectID);
    if (project && project.team_id !== selectedTeamId) {
      selectedTeamId = project.team_id;
      projectTeamId = project.team_id;
    }
    selectedProjectId = project?.id ?? '';
    selectedScopeId = project?.scope.id ?? selectedTeam?.scope.id ?? '';
    selectedResourceId = '';
  }

  function resourceInActiveWorkspace(resource: Resource) {
    if (activeScope?.type === 'platform') return true;
    const selectedTeamScopeID = selectedTeam?.scope.id;
    if (activeScope?.type === 'team') {
      return (
        resource.scope_id === platform?.scope.id ||
        resource.scope_id === selectedTeamScopeID ||
        selectedTeamProjects.some(
          (project) => project.scope.id === resource.scope_id
        )
      );
    }
    return (
      resource.scope_id === platform?.scope.id ||
      resource.scope_id === selectedTeamScopeID ||
      resource.scope_id === selectedProject?.scope.id
    );
  }

  async function loadInspection() {
    if (!selectedScopeId) return;
    inspectionLoaded = true;
    try {
      [
        inspectionPolicies,
        inspectionRuns,
        inspectionFindings,
        notificationChannels
      ] = await Promise.all([
        api.inspectionPolicies(selectedScopeId),
        api.inspectionRuns(selectedScopeId),
        api.inspectionFindings(selectedScopeId),
        api.notificationChannels(selectedScopeId)
      ]);
    } catch (error) {
      errorMessage = describeError(error, '巡检数据加载失败');
    }
  }
  async function rerunInspection(policyID: string) {
    busy = true;
    try {
      await api.startInspectionRun(policyID, selectedScopeId);
      notice = '已创建手动巡检任务。';
      await loadInspection();
    } catch (error) {
      errorMessage = describeError(error, '创建巡检任务失败');
    } finally {
      busy = false;
    }
  }
  async function setInspectionPolicyStatus(policyID: string, status: string) {
    busy = true;
    try {
      await api.setInspectionPolicyStatus(policyID, selectedScopeId, status);
      notice = status === 'disabled' ? '已停止周期巡检。' : '已恢复周期巡检。';
      await loadInspection();
    } catch (error) {
      errorMessage = describeError(error, '更新巡检策略失败');
    } finally {
      busy = false;
    }
  }

  function toggleInspectionSelection(list: string[], id: string) {
    return list.includes(id)
      ? list.filter((item) => item !== id)
      : [...list, id];
  }

  async function createInspectionPolicy() {
    await action(async () => {
      const created = await api.createInspectionPolicy({
        scope_id: selectedScopeId,
        name: inspectionPolicyName,
        cron: inspectionCron,
        timezone: inspectionTimezone,
        target_resource_ids: inspectionTargetIds,
        target_labels: JSON.parse(inspectionTargetLabels),
        agent_profile_resource_id: inspectionAgentProfileId || undefined,
        timeout_seconds: inspectionTimeoutSeconds,
        retries: inspectionRetries,
        max_concurrent: inspectionMaxConcurrent,
        max_tool_calls: inspectionMaxToolCalls,
        max_tokens: inspectionMaxTokens,
        maintenance: []
      });
      inspectionPolicies = [created, ...inspectionPolicies];
      inspectionPolicyName = '';
      notice = '巡检策略已创建。';
    });
  }

  async function createNotificationChannel() {
    await action(async () => {
      const created = await api.createNotificationChannel({
        scope_id: selectedScopeId,
        name: channelName,
        webhook_url: channelWebhookURL,
        status: 'active',
        rate_limit_per_minute: channelRateLimit
      });
      notificationChannels = [created, ...notificationChannels];
      channelName = '';
      channelWebhookURL = '';
      notice = 'Webhook 通知渠道已创建。';
    });
  }

  async function loadOperations() {
    if (!selectedScopeId) return;
    operationsLoaded = true;
    try {
      operationRequests = await api.operationRequests(selectedScopeId);
      const mcpServers = resources.filter(
        (item) => item.kind === 'MCPServer' && item.scope_id === selectedScopeId
      );
      operationSnapshots = Object.fromEntries(
        await Promise.all(
          mcpServers.map(async (server) => [
            server.id,
            await api.mcpSnapshots(server.id)
          ])
        )
      );
    } catch (error) {
      errorMessage = describeError(error, '受控操作数据加载失败');
    }
  }
  async function createOperationRequest() {
    await action(async () => {
      const created = await api.createOperationRequest({
        scope_id: selectedScopeId,
        target_resource_id: operationTargetId,
        operation_name: operationName,
        risk_level: operationRisk,
        parameters: JSON.parse(operationParameters),
        impact_summary: operationImpact,
        rollback_summary: operationRollback,
        dry_run: { requested: true },
        idempotency_key: crypto.randomUUID()
      });
      operationRequests = [created, ...operationRequests];
      notice = '已创建操作请求；中高风险操作等待另一位有权限的审批人处理。';
    });
  }
  async function approveOperation(
    item: OperationRequest,
    decision: 'approved' | 'rejected'
  ) {
    await action(async () => {
      const updated = await api.approveOperation(item.id, {
        decision,
        parameters_hash: item.parameters_hash
      });
      operationRequests = operationRequests.map((current) =>
        current.id === updated.id ? updated : current
      );
      notice =
        decision === 'approved' ? '操作请求已批准。' : '操作请求已拒绝。';
    });
  }
  async function startOperation(item: OperationRequest) {
    await action(async () => {
      await api.startOperation(item.id, crypto.randomUUID());
      operationRequests = await api.operationRequests(selectedScopeId);
      notice = '执行已排队；实际变更只能由受限 Kubernetes Job 完成。';
    });
  }
  async function discoverMCP(resourceId: string) {
    await action(async () => {
      const snapshot = await api.discoverMCP(resourceId);
      operationSnapshots = {
        ...operationSnapshots,
        [resourceId]: [snapshot, ...(operationSnapshots[resourceId] ?? [])]
      };
      notice =
        snapshot.status === 'succeeded'
          ? '已发现并保存 MCP 工具快照。'
          : 'MCP 健康检查失败，已保存受限错误信息。';
    });
  }

  async function loadDiagnosis() {
    diagnosisLoaded = true;
    if (selectedScopeId) {
      try {
        diagnosisAvailableProviders = await api.availableAIProviders(
          selectedScopeId,
          'diagnosis'
        );
        if (
          diagnosisAvailableProviders.length > 0 &&
          (!selectedProviderId ||
            !diagnosisAvailableProviders.some(
              (item) => item.provider_resource_id === selectedProviderId
            ))
        ) {
          selectedProviderId =
            diagnosisAvailableProviders[0].provider_resource_id;
          llmModelName = diagnosisAvailableProviders[0].models[0]?.name ?? '';
        }
      } catch (error) {
        diagnosisAvailableProviders = [];
        errorMessage = describeError(error, '可用模型服务商加载失败');
      }
    }
    await loadDiagnosisSessions();
  }

  async function loadDiagnosisSessions() {
    if (!selectedScopeId) return;
    try {
      diagnosisSessions = await api.diagnosisSessions(selectedScopeId);
      if (!selectedDiagnosisId && diagnosisSessions[0]) {
        await openDiagnosis(diagnosisSessions[0].id);
      }
    } catch (error) {
      errorMessage = describeError(error, '诊断会话历史加载失败');
    }
  }

  async function openDiagnosis(id: string) {
    closeDiagnosisEvents();
    selectedDiagnosisId = id;
    selectedEvidence = null;
    diagnosisEditingMessageId = '';
    diagnosisEditDraft = '';
    diagnosisInterruptedReason = '';
    diagnosisStreamingText = '';
    diagnosisStreamingPending = '';
    if (diagnosisStreamingFrame) cancelAnimationFrame(diagnosisStreamingFrame);
    diagnosisStreamingFrame = 0;
    diagnosisStreamingStartedAt = 0;
    try {
      diagnosisSnapshot = await api.diagnosisSession(id);
      diagnosisSnapshot = {
        ...diagnosisSnapshot,
        messages: diagnosisSnapshot.messages.filter(
          (message) => !diagnosisHiddenMessageIds.includes(message.id)
        )
      };
      diagnosisTargetIds = diagnosisSnapshot.targets.map(
        (target) => target.resource_id
      );
      if (diagnosisSnapshot.session.ai_provider_resource_id) {
        selectedProviderId = diagnosisSnapshot.session.ai_provider_resource_id;
      }
      if (diagnosisSnapshot.session.model_name) {
        llmModelName = diagnosisSnapshot.session.model_name;
      }
      diagnosisGenerating = isDiagnosisRunning(
        diagnosisSnapshot.session.status
      );
      diagnosisEventCursor = 0;
      openDiagnosisEvents(id);
    } catch (error) {
      errorMessage = describeError(error, '诊断详情加载失败');
    }
  }

  function openDiagnosisEvents(id: string) {
    closeDiagnosisEvents();
    const stream = new EventSource(
      api.diagnosisEventsURL(id, diagnosisEventCursor)
    );
    diagnosisEvents = stream;
    stream.onmessage = (event) => handleDiagnosisEvent(id, event);
    for (const type of [
      'session.created',
      'phase.changed',
      'plan.created',
      'execution.started',
      'execution.completed',
      'execution.cancelled',
      'execution.failed',
      'assistant.delta',
      'assistant.completed',
      'tool.requested',
      'tool.started',
      'tool.completed',
      'tool.failed',
      'evidence.collected',
      'report.ready',
      'diagnosis.failed',
      'message.created',
      'target.added'
    ]) {
      stream.addEventListener(type, (event) => {
        handleDiagnosisEvent(id, event as MessageEvent);
      });
    }
    stream.onerror = () => {
      // Native EventSource reconnects with the last received event id.
    };
  }

  function handleDiagnosisEvent(id: string, event: MessageEvent) {
    if (id !== selectedDiagnosisId) return;
    diagnosisEventCursor = Number(event.lastEventId) || diagnosisEventCursor;
    let payload: Record<string, unknown> = {};
    try {
      payload = event.data ? JSON.parse(event.data) : {};
    } catch {
      payload = {};
    }
    const eventType = event.type || 'message';
    appendDiagnosisEvent(eventType, payload, diagnosisEventCursor);
    if (eventType === 'assistant.delta') {
      const text = String(payload.text ?? '');
      if (text) {
        diagnosisStreamingPending += text;
        if (!diagnosisStreamingFrame) {
          diagnosisStreamingFrame = requestAnimationFrame(() => {
            diagnosisStreamingText += diagnosisStreamingPending;
            diagnosisStreamingPending = '';
            diagnosisStreamingFrame = 0;
          });
        }
        diagnosisStreamingStartedAt ||= Date.now();
      }
      diagnosisGenerating = true;
      return;
    }
    if (eventType === 'assistant.completed') {
      if (!diagnosisStreamingText && payload.text) {
        diagnosisStreamingText = String(payload.text);
      }
      diagnosisGenerating = true;
      void refreshDiagnosis(id);
      return;
    }
    if (eventType === 'execution.started' || eventType === 'tool.requested' || eventType === 'tool.started' || eventType === 'phase.changed') {
      diagnosisGenerating = true;
      diagnosisStreamingStartedAt ||= Date.now();
      void refreshDiagnosis(id);
      return;
    }
    if (eventType === 'execution.failed' || eventType === 'execution.cancelled' || eventType === 'diagnosis.failed') {
      const reason = String(payload.error ?? payload.message ?? payload.error_message ?? '').trim();
      diagnosisInterruptedReason = reason || (eventType === 'execution.cancelled' ? '回答被取消。' : '回答生成失败。');
      diagnosisGenerating = false;
      void refreshDiagnosis(id);
      return;
    }
    if (eventType === 'execution.completed' || eventType === 'report.ready') {
      void refreshDiagnosis(id).finally(() => {
        if (diagnosisStreamingPending) {
          diagnosisStreamingText += diagnosisStreamingPending;
          diagnosisStreamingPending = '';
        }
        if (diagnosisStreamingFrame) cancelAnimationFrame(diagnosisStreamingFrame);
        diagnosisStreamingFrame = 0;
        diagnosisGenerating = false;
        diagnosisStreamingText = '';
        diagnosisStreamingStartedAt = 0;
      });
      return;
    }
    void refreshDiagnosis(id);
  }

  function appendDiagnosisEvent(type: string, payload: Record<string, unknown>, id: number) {
    if (!diagnosisSnapshot || !id) return;
    const current = diagnosisSnapshot.events ?? [];
    if (current.some((item) => item.id === id)) return;
    const item = {
      id,
      session_id: diagnosisSnapshot.session.id,
      type,
      payload,
      created_at: new Date().toISOString()
    };
    diagnosisSnapshot = {
      ...diagnosisSnapshot,
      events: [...current, item].sort((a, b) => a.id - b.id)
    };
  }

  function closeDiagnosisEvents() {
    diagnosisEvents?.close();
    diagnosisEvents = null;
    if (diagnosisStreamingFrame) cancelAnimationFrame(diagnosisStreamingFrame);
    diagnosisStreamingFrame = 0;
  }

  async function refreshDiagnosis(id = selectedDiagnosisId) {
    if (!id || id !== selectedDiagnosisId) return;
    const refreshToken = ++diagnosisRefreshToken;
    try {
      const snapshot = await api.diagnosisSession(id);
      if (refreshToken !== diagnosisRefreshToken || id !== selectedDiagnosisId) return;
      diagnosisSnapshot = {
        ...snapshot,
        messages: snapshot.messages.filter(
          (message) => !diagnosisHiddenMessageIds.includes(message.id)
        )
      };
      diagnosisGenerating = isDiagnosisRunning(snapshot.session.status);
      diagnosisSessions = diagnosisSessions.map((item) =>
        item.id === diagnosisSnapshot?.session.id
          ? diagnosisSnapshot.session
          : item
      );
      if (
        diagnosisSnapshot.session.status === 'succeeded' ||
        diagnosisSnapshot.session.status === 'failed' ||
        diagnosisSnapshot.session.status === 'cancelled'
      ) {
        closeDiagnosisEvents();
        diagnosisGenerating = false;
      }
    } catch (error) {
      errorMessage = describeError(error, '诊断状态刷新失败');
    }
  }

  function toggleDiagnosisTarget(resourceID: string) {
    diagnosisTargetIds = diagnosisTargetIds.includes(resourceID)
      ? diagnosisTargetIds.filter((id) => id !== resourceID)
      : [...diagnosisTargetIds, resourceID];
  }

  async function startDiagnosis() {
    if (!selectedScopeId) {
      errorMessage = '请先选择一个可用级别。';
      return;
    }
    await action(async () => {
      const session = await api.startDiagnosis({
        scope_id: selectedScopeId,
        question: diagnosisQuestion,
        target_resource_ids: diagnosisTargetIds,
        ai_provider_resource_id: selectedProviderId || undefined,
        model_name: llmModelName || undefined
      });
      diagnosisSessions = [session, ...diagnosisSessions];
      diagnosisQuestion = '';
      diagnosisTargetIds = [];
      diagnosisFollowup = '';
      diagnosisInterruptedReason = '';
      await openDiagnosis(session.id);
    });
  }

  async function submitDiagnosisMessage() {
    const content = diagnosisComposerText.trim();
    if (!content) return;
    if (!selectedDiagnosisId) {
      diagnosisQuestion = content;
      await startDiagnosis();
      diagnosisComposerText = '';
      return;
    }
    diagnosisFollowup = content;
    await sendDiagnosisFollowup();
    diagnosisComposerText = '';
  }

  function handleDiagnosisComposerKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      void submitDiagnosisMessage();
    }
  }

  function isLastDiagnosisUser(index: number) {
    const messages = diagnosisSnapshot?.messages ?? [];
    return index === messages.map((item) => item.role).lastIndexOf('user');
  }

  function copyDiagnosisAnswer(content: string) {
    void navigator.clipboard?.writeText(content).then(
      () => (notice = '回答已复制。'),
      () => (notice = '当前浏览器不允许直接复制，请手动选择文本。')
    );
  }

  function toggleDiagnosisContext(resourceID: string) {
    if (diagnosisTargetIds.includes(resourceID)) {
      diagnosisTargetIds = diagnosisTargetIds.filter((id) => id !== resourceID);
      return;
    }
    if (diagnosisTargetIds.length >= 20) {
      errorMessage = '一次诊断最多加载 20 个上下文资源。';
      return;
    }
    diagnosisTargetIds = [...diagnosisTargetIds, resourceID];
    if (selectedDiagnosisId) {
      void action(async () => {
        await api.addDiagnosisTarget(selectedDiagnosisId, resourceID);
        await refreshDiagnosis();
      });
    }
  }

  function startDiagnosisResize(
    side: 'history' | 'context',
    event: PointerEvent
  ) {
    if (window.innerWidth <= 850) return;
    const startX = event.clientX;
    const startWidth =
      side === 'history' ? diagnosisHistoryWidth : diagnosisContextWidth;
    const move = (moveEvent: PointerEvent) => {
      const delta = moveEvent.clientX - startX;
      if (side === 'history') {
        diagnosisHistoryWidth = Math.max(
          180,
          Math.min(360, startWidth + delta)
        );
      } else {
        diagnosisContextWidth = Math.max(
          220,
          Math.min(380, startWidth - delta)
        );
      }
    };
    const stop = () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', stop);
    };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', stop, { once: true });
  }

  async function sendDiagnosisFollowup() {
    if (!selectedDiagnosisId || !diagnosisFollowup.trim()) return;
    await action(async () => {
      await api.askDiagnosis(selectedDiagnosisId, diagnosisFollowup);
      diagnosisFollowup = '';
      diagnosisInterruptedReason = '';
      diagnosisGenerating = true;
      await refreshDiagnosis();
      openDiagnosisEvents(selectedDiagnosisId);
    });
  }

  function isDiagnosisRunning(status: DiagnosisStatus | string) {
    return ['queued', 'planning', 'collecting', 'analyzing'].includes(status);
  }

  function newDiagnosisSession() {
    closeDiagnosisEvents();
    selectedDiagnosisId = '';
    diagnosisSnapshot = null;
    selectedEvidence = null;
    diagnosisQuestion = '';
    diagnosisFollowup = '';
    diagnosisComposerText = '';
    diagnosisTargetIds = [];
    diagnosisEditingMessageId = '';
    diagnosisEditDraft = '';
    diagnosisInterruptedReason = '';
    diagnosisGenerating = false;
    diagnosisStreamingText = '';
    diagnosisStreamingStartedAt = 0;
  }

  async function clearDiagnosisHistory() {
    const sessions = [...diagnosisSessions];
    await action(async () => {
      await Promise.all(sessions.map((session) => api.deleteDiagnosis(session.id)));
      closeDiagnosisEvents();
      diagnosisSessions = [];
      newDiagnosisSession();
    });
  }

  function renameDiagnosisSession(session: DiagnosisSession) {
    const title = window.prompt(
      '重命名诊断会话',
      session.title || '未命名诊断'
    );
    if (!title?.trim()) return;
    diagnosisSessions = diagnosisSessions.map((item) =>
      item.id === session.id ? { ...item, title: title.trim() } : item
    );
    if (diagnosisSnapshot?.session.id === session.id) {
      diagnosisSnapshot = {
        ...diagnosisSnapshot,
        session: { ...diagnosisSnapshot.session, title: title.trim() }
      };
    }
  }

  async function deleteDiagnosisSession(session: DiagnosisSession) {
    if (!window.confirm(`删除“${session.title || '未命名诊断'}”的列表记录？`))
      return;
    await action(async () => {
      await api.deleteDiagnosis(session.id);
      diagnosisSessions = diagnosisSessions.filter((item) => item.id !== session.id);
      if (selectedDiagnosisId === session.id) newDiagnosisSession();
    });
  }

  function beginDiagnosisEdit(message: DiagnosisMessage) {
    diagnosisEditingMessageId = message.id;
    diagnosisEditDraft = message.content;
  }

  async function saveDiagnosisEdit() {
    const content = diagnosisEditDraft.trim();
    if (!content || !selectedDiagnosisId || !diagnosisSnapshot) return;
    const originalID = diagnosisEditingMessageId;
    await action(async () => {
      const created = await api.askDiagnosis(selectedDiagnosisId, content);
      diagnosisHiddenMessageIds = [...diagnosisHiddenMessageIds, originalID];
      diagnosisSnapshot = {
        ...diagnosisSnapshot!,
        messages: [
          ...diagnosisSnapshot!.messages.filter(
            (message) => message.id !== originalID
          ),
          created
        ]
      };
      diagnosisEditingMessageId = '';
      diagnosisEditDraft = '';
      diagnosisFollowup = '';
      diagnosisGenerating = true;
      openDiagnosisEvents(selectedDiagnosisId);
    });
  }

  function stopDiagnosisGeneration() {
    if (!selectedDiagnosisId || !diagnosisGenerating) return;
    closeDiagnosisEvents();
    diagnosisGenerating = false;
    diagnosisInterruptedReason = '用户手动停止了当前回答。';
    void api.cancelDiagnosis(selectedDiagnosisId).catch((error) => {
      diagnosisInterruptedReason = `停止请求失败：${describeError(error, '无法停止后台执行')}`;
    });
  }

  function diagnosisStatusLabel(status: string) {
    const labels: Record<string, string> = {
      queued: '排队中',
      planning: '规划中',
      collecting: '采集中',
      analyzing: '分析中',
      succeeded: '已完成',
      failed: '失败',
      cancelled: '已取消'
      ,skipped: '已跳过'
      ,warning: '需核验'
    };
    return labels[status] ?? status;
  }

  function diagnosisDuration(start: string, end?: string) {
    const begin = new Date(start).getTime();
    const finish = end ? new Date(end).getTime() : Date.now();
    if (!Number.isFinite(begin) || !Number.isFinite(finish)) return '—';
    const seconds = Math.max(0, Math.round((finish - begin) / 1000));
    return seconds < 60
      ? `${seconds}s`
      : `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  }

  function diagnosisActionData(snapshot: DiagnosisSnapshot): any[] {
    const events = [...(snapshot.events ?? [])].sort((a, b) => a.id - b.id);
    type ToolAction = {
      id: string;
      icon: 'tool';
      title: string;
      status: string;
      duration: string;
      input: string;
      output: string;
      created_at: string;
      updated_at: string;
      tool: string;
      resourceID: string;
    };
    type ToolGroup = {
      id: string;
      title: string;
      status: string;
      duration: string;
      children: ToolAction[];
    };
    const titleForTool = (name: string) => {
      const titles: Record<string, string> = {
        'connector.query_metrics': '查询监控指标',
        'connector.get_alerts': '查询告警',
        'connector.query_logs': '查询日志',
        'connector.inspect_postgresql': '检查 PostgreSQL',
        'connector.read_kubernetes': '查询 Kubernetes'
      };
      return titles[name] ?? `调用 ${name}`;
    };
    const groups: ToolGroup[] = [];
    let currentGroup: ToolGroup | null = null;

    for (const event of events) {
      // Text is the only user-visible boundary: tool calls on either side
      // belong to different Codex-style collapsed action groups.
      if (event.type === 'assistant.delta') {
        if (String(event.payload?.text ?? '').trim()) currentGroup = null;
        continue;
      }
      if (!event.type.startsWith('tool.')) continue;
      const payload = event.payload ?? {};
      const tool = String(payload.tool ?? '');
      const resourceID = String(payload.resource_id ?? '');
      // Evidence bookkeeping also emits tool.completed, but it is not an
      // AIEngine invocation and therefore must not appear in this trace.
      if (!tool || !resourceID) continue;
      if (!currentGroup) {
        currentGroup = {
          id: `tool-group-${event.id}`,
          title: '工具调用',
          status: '进行中',
          duration: '—',
          children: []
        };
        groups.push(currentGroup);
      }
      let action = [...currentGroup.children]
        .reverse()
        .find((item) => item.tool === tool && item.resourceID === resourceID && item.status === '进行中');
      if (!action) {
        action = {
          id: `tool-${event.id}`,
          icon: 'tool',
          title: titleForTool(tool),
          status: '进行中',
          duration: '—',
          input: JSON.stringify({ tool, resource_id: resourceID }, null, 2),
          output: '等待工具执行结果…',
          created_at: event.created_at,
          updated_at: event.created_at,
          tool,
          resourceID
        };
        currentGroup.children.push(action);
      }
      action.updated_at = event.created_at;
      if (event.type === 'tool.completed') {
        action.status = '已完成';
        const duration = Number(payload.duration_ms ?? 0);
        action.duration = duration > 0 ? `${Math.max(1, Math.round(duration / 1000))}s` : diagnosisDuration(action.created_at, event.created_at);
        action.output = JSON.stringify({ status: 'succeeded', duration_ms: payload.duration_ms ?? 0 }, null, 2);
      } else if (event.type === 'tool.failed') {
        action.status = '失败';
        const duration = Number(payload.duration_ms ?? 0);
        action.duration = duration > 0 ? `${Math.max(1, Math.round(duration / 1000))}s` : diagnosisDuration(action.created_at, event.created_at);
        action.output = JSON.stringify({ status: 'failed', error: payload.error ?? '工具调用失败' }, null, 2);
      }
      const completed = currentGroup.children.every((item) => item.status !== '进行中');
      currentGroup.status = completed ? (currentGroup.children.some((item) => item.status === '失败') ? '失败' : '已完成') : '进行中';
      currentGroup.duration = diagnosisDuration(currentGroup.children[0].created_at, action.updated_at);
    }
    return groups;
  }

  function renderDiagnosisMarkdown(value: string) {
    const escape = (text: string) =>
      text
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
    const escaped = escape(value ?? '');
    const lines = escaped.split(/\r?\n/);
    let html = '';
    let inList = false;
    const inline = (line: string) =>
      line
        .replace(/`([^`]+)`/g, '<code>$1</code>')
        .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
        .replace(/__([^_]+)__/g, '<strong>$1</strong>')
        .replace(/\*([^*]+)\*/g, '<em>$1</em>')
        .replace(/_([^_]+)_/g, '<em>$1</em>')
        .replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>');
    const closeList = () => {
      if (inList) {
        html += '</ul>';
        inList = false;
      }
    };
    for (const rawLine of lines) {
      const line = rawLine.trimEnd();
      const listItem = line.match(/^\s*[-*]\s+(.+)$/);
      if (listItem) {
        if (!inList) {
          html += '<ul>';
          inList = true;
        }
        html += `<li>${inline(listItem[1])}</li>`;
        continue;
      }
      closeList();
      if (!line.trim()) continue;
      const heading = line.match(/^\s*(#{1,3})\s+(.+)$/);
      if (heading) {
        const level = heading[1].length;
        html += `<h${level}>${inline(heading[2])}</h${level}>`;
      } else {
        html += `<p>${inline(line)}</p>`;
      }
    }
    closeList();
    return html;
  }

  async function loadAI() {
    aiLoaded = true;
    selectedSkillId = selectedSkillId || skillResources[0]?.id || '';
    selectedAgentProfileId =
      selectedAgentProfileId || agentProfileResources[0]?.id || '';
    if (selectedSkillId) await loadSkillVersions();
    if (selectedAgentProfileId) await loadAgentProfileVersions();
  }

  async function loadAgentProfileVersions() {
    if (!selectedAgentProfileId) return;
    try {
      agentProfileVersions = await api.agentProfileVersions(
        selectedAgentProfileId
      );
      selectedAgentProfileVersionId =
        agentProfileVersions.find((item) => item.status === 'published')?.id ||
        agentProfileVersions[0]?.id ||
        '';
    } catch (error) {
      errorMessage = describeError(error, 'AgentProfile 版本加载失败');
    }
  }

  async function createAgentProfile() {
    if (
      !selectedScopeId ||
      !agentProfileName.trim() ||
      !agentProfileInstruction.trim()
    )
      return;
    await action(async () => {
      const config = {
        version: 1,
        instruction: agentProfileInstruction.trim(),
        capabilities: agentProfileCapabilities
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean),
        allowed_tools: agentProfileAllowedTools
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean),
        target_kinds: agentProfileTargetKinds
          .split(',')
          .map((item) => item.trim())
          .filter(Boolean),
        input_schema: JSON.parse(agentProfileInputSchema),
        output_schema: JSON.parse(agentProfileOutputSchema),
        enabled: true
      };
      const resource = await api.createResource({
        scope_id: selectedScopeId,
        kind: 'AgentProfile',
        schema_version: 1,
        name: agentProfileName.trim(),
        labels: {},
        config,
        status: 'active'
      });
      const version = await api.createAgentProfileVersion(resource.id, config);
      await api.publishAgentProfileVersion(resource.id, version.id);
      resources = [resource, ...resources];
      selectedAgentProfileId = resource.id;
      agentProfileVersions = [version];
      agentProfileName = '';
      agentProfileInstruction = '';
      notice = 'AgentProfile 已创建并发布 v1';
    });
  }

  async function publishAgentProfileVersion() {
    if (!selectedAgentProfileId || !selectedAgentProfileVersionId) return;
    await action(async () => {
      await api.publishAgentProfileVersion(
        selectedAgentProfileId,
        selectedAgentProfileVersionId
      );
      await loadAgentProfileVersions();
      notice = 'AgentProfile 版本已发布';
    });
  }

  async function loadSkillVersions() {
    if (!selectedSkillId) return;
    try {
      skillVersions = await api.skillVersions(selectedSkillId);
      selectedSkillVersionId =
        skillVersions.find((item) => item.status === 'published')?.id ||
        skillVersions[0]?.id ||
        '';
    } catch (error) {
      errorMessage = describeError(error, 'Skill 版本加载失败');
    }
  }

  async function createSkillVersion() {
    if (!selectedSkillId) return;
    await action(async () => {
      const version = await api.createSkillVersion(selectedSkillId, {
        manifest: {
          name:
            skillResources.find((item) => item.id === selectedSkillId)?.name ||
            'Skill',
          description: '由 OpsKeeper 管理的声明式 Skill',
          instruction: skillInstruction,
          target_kinds: skillTargetKinds
            .split(',')
            .map((item) => item.trim())
            .filter(Boolean)
        },
        input_schema: JSON.parse(skillInputSchema),
        output_schema: JSON.parse(skillOutputSchema),
        tools: skillToolOptions
          .filter((tool) => selectedSkillToolNames.includes(tool.name))
          .map((tool) => ({
            name: tool.name,
            description: tool.description,
            input_schema: tool.inputSchema
          })),
        risk_level: 'read_only'
      });
      skillVersions = [version, ...skillVersions];
      selectedSkillVersionId = version.id;
      notice = `Skill v${version.version} 草稿已创建`;
    });
  }

  function toggleSkillTool(name: string) {
    selectedSkillToolNames = selectedSkillToolNames.includes(name)
      ? selectedSkillToolNames.filter((item) => item !== name)
      : [...selectedSkillToolNames, name];
  }

  async function publishSkillVersion() {
    if (!selectedSkillId || !selectedSkillVersionId) return;
    await action(async () => {
      await api.publishSkillVersion(selectedSkillId, selectedSkillVersionId);
      await loadSkillVersions();
      notice = 'Skill 版本已发布';
    });
  }

  async function setSkillDefault() {
    if (!selectedScopeId || !selectedSkillId || !selectedSkillVersionId) return;
    await action(async () => {
      await api.setSkillDefault({
        scope_id: selectedScopeId,
        skill_resource_id: selectedSkillId,
        skill_version_id: selectedSkillVersionId
      });
      notice = '默认 Skill 已更新';
    });
  }

  async function loadDiscovery() {
    discoveryLoaded = true;
    selectedClusterId = selectedClusterId || kubernetesClusters[0]?.id || '';
    if (!selectedClusterId) return;
    await loadDiscoveryRuns();
  }

  async function selectDiscoveryCluster() {
    activeDiscovery = null;
    discoveryItems = [];
    projectMappingDrafts = {};
    selectedDiscoveryItems = {};
    await loadDiscoveryRuns();
  }

  async function loadDiscoveryRuns() {
    if (!selectedClusterId) return;
    try {
      discoveryRuns = await api.discoveryRuns(selectedClusterId);
      if (discoveryRuns[0]) await openDiscovery(discoveryRuns[0]);
    } catch (error) {
      errorMessage = describeError(error, '集群同步历史加载失败');
    }
  }

  async function startDiscovery() {
    if (!selectedClusterId) return;
    await action(async () => {
      activeDiscovery = await api.startDiscovery(selectedClusterId);
      discoveryRuns = [activeDiscovery, ...discoveryRuns];
      discoveryItems = [];
      notice = '集群扫描已开始';
      void pollDiscovery(activeDiscovery.id);
    });
  }

  async function pollDiscovery(id: string) {
    for (let attempt = 0; attempt < 120; attempt += 1) {
      await new Promise((resolve) => window.setTimeout(resolve, 1000));
      if (activeDiscovery?.id !== id) return;
      try {
        const run = await api.discovery(id);
        activeDiscovery = run;
        discoveryRuns = discoveryRuns.map((item) =>
          item.id === run.id ? run : item
        );
        if (run.status === 'succeeded') {
          await loadDiscoveryItems(id);
          notice = `扫描完成，共发现 ${run.item_count} 个项目和应用候选`;
          return;
        }
        if (run.status === 'failed' || run.status === 'cancelled') {
          errorMessage = run.error_message || '集群扫描失败';
          return;
        }
      } catch (error) {
        errorMessage = describeError(error, '扫描状态刷新失败');
        return;
      }
    }
    errorMessage = '扫描仍在运行，可稍后从同步历史重新打开。';
  }

  async function openDiscovery(run: DiscoveryRun) {
    activeDiscovery = run;
    if (run.status === 'succeeded') await loadDiscoveryItems(run.id);
    else if (run.status === 'queued' || run.status === 'running')
      void pollDiscovery(run.id);
  }

  async function loadDiscoveryItems(id: string) {
    discoveryItems = await api.discoveryItems(id);
    selectedDiscoveryItems = Object.fromEntries(
      discoveryItems
        .filter((item) => item.kind === 'Application')
        .map((item) => [item.id, item.status !== 'ignored'])
    );
    projectMappingDrafts = Object.fromEntries(
      discoveryItems
        .filter((item) => item.kind === 'Project')
        .map((item) => [
          item.namespace || item.name,
          defaultProjectMapping(item)
        ])
    );
  }

  function defaultProjectMapping(item: DiscoveryItem): ProjectMappingDraft {
    const namespace = item.namespace || item.name;
    if (['kube-system', 'kube-public', 'kube-node-lease'].includes(namespace))
      return { mode: 'ignore', ignore: true };
    const mapped = projects.find(
      (project) =>
        project.source_resource_id === selectedClusterId &&
        project.external_uid === item.external_uid
    );
    if (mapped) return { mode: 'existing', project_id: mapped.id };
    const onlyProject = allowedProjectsForCluster()[0];
    const cluster = kubernetesClusters.find(
      (resource) => resource.id === selectedClusterId
    );
    if (
      cluster &&
      scopeChoices.find((scope) => scope.id === cluster.scope_id)?.type ===
        'project' &&
      onlyProject
    )
      return { mode: 'existing', project_id: onlyProject.id };
    const allowedTeam = allowedTeamsForCluster()[0];
    return {
      mode: 'create',
      team_id: allowedTeam?.id || '',
      name: item.name,
      code: namespaceCode(item.namespace || item.name)
    };
  }

  function allowedTeamsForCluster() {
    const cluster = kubernetesClusters.find(
      (resource) => resource.id === selectedClusterId
    );
    if (!cluster) return [];
    const scope = scopeChoices.find((item) => item.id === cluster.scope_id);
    if (scope?.type === 'platform') return teams;
    if (scope?.type === 'team')
      return teams.filter((team) => team.scope.id === cluster.scope_id);
    const project = projects.find((item) => item.scope.id === cluster.scope_id);
    return teams.filter((team) => team.id === project?.team_id);
  }

  function allowedProjectsForCluster() {
    const cluster = kubernetesClusters.find(
      (resource) => resource.id === selectedClusterId
    );
    if (!cluster) return [];
    const scope = scopeChoices.find((item) => item.id === cluster.scope_id);
    if (scope?.type === 'platform') return projects;
    if (scope?.type === 'team') {
      const team = teams.find((item) => item.scope.id === cluster.scope_id);
      return projects.filter((project) => project.team_id === team?.id);
    }
    return projects.filter((project) => project.scope.id === cluster.scope_id);
  }

  async function importDiscovery() {
    if (!activeDiscovery) return;
    const discoveryID = activeDiscovery.id;
    await action(async () => {
      const projectMappings = Object.fromEntries(
        Object.entries(projectMappingDrafts).map(([namespace, draft]) => {
          if (draft.mode === 'ignore') return [namespace, { ignore: true }];
          if (draft.mode === 'existing')
            return [namespace, { project_id: draft.project_id }];
          return [
            namespace,
            {
              team_id: draft.team_id,
              name: draft.name,
              code: draft.code
            }
          ];
        })
      );
      const result = await api.importDiscovery(discoveryID, {
        item_ids: Object.entries(selectedDiscoveryItems)
          .filter(([, selected]) => selected)
          .map(([id]) => id),
        project_mappings: projectMappings
      });
      activeDiscovery = result.run;
      await Promise.all([loadWorkspace(), loadDiscoveryItems(result.run.id)]);
      notice = `已映射 ${result.imported.filter((item) => item.kind === 'Project').length} 个项目并导入 ${result.imported.filter((item) => item.kind === 'Application').length} 个应用`;
    });
  }

  function namespaceCode(value: string) {
    return (
      value
        .trim()
        .toLowerCase()
        .replace(/[^a-z0-9-]+/g, '-')
        .replace(/^-+|-+$/g, '') || 'kubernetes-project'
    );
  }

  function payloadCount(item: DiscoveryItem, key: string) {
    const value = item.payload[key];
    return Array.isArray(value) ? value.length : 0;
  }

  async function loadAccess() {
    accessLoaded = true;
    accessLoading = true;
    accessLoadError = '';
    try {
      const result = await Promise.allSettled([
        api.users(),
        api.groups(),
        api.roles(),
        api.bindings(),
        api.resourceRoles(),
        api.resourceBindings()
      ]);
      users = result[0].status === 'fulfilled' ? result[0].value : [];
      groups = result[1].status === 'fulfilled' ? result[1].value : [];
      roles = result[2].status === 'fulfilled' ? result[2].value : [];
      bindings = result[3].status === 'fulfilled' ? result[3].value : [];
      resourceRoles = result[4].status === 'fulfilled' ? result[4].value : [];
      resourceBindings =
        result[5].status === 'fulfilled' ? result[5].value : [];
      const memberResults = await Promise.allSettled(
        groups.map((group) => api.groupMembers(group.id))
      );
      groupMembers = Object.fromEntries(
        groups.map((group, index) => [
          group.id,
          memberResults[index]?.status === 'fulfilled'
            ? memberResults[index].value.map((member) => member.user_id)
            : []
        ])
      );
      const rejectedItems = result
        .map((item, index) => (item.status === 'rejected' ? index : -1))
        .filter((index) => index >= 0);
      if (rejectedItems.length > 0) {
        const names = [
          '用户',
          '成员组',
          '角色',
          '角色授权',
          '资源角色',
          '资源授权'
        ];
        accessLoadError = `管理数据加载不完整：${rejectedItems.map((index) => names[index]).join('、')}。`;
      }
      if (newUserGrants.length === 0) resetUserDialog();
    } catch {
      accessLoadError = '成员和角色数据加载失败，请重试。';
    } finally {
      accessLoading = false;
    }
  }

  async function createTeam() {
    await action(async () => {
      const created = await api.createTeam({
        name: teamName,
        code: teamCode,
        icon: teamIcon,
        labels: {}
      });
      teams = [...teams, created];
      teamName = '';
      teamCode = '';
      teamIcon = 'UsersRound';
      notice = `团队“${created.name}”已创建`;
      teamDialogOpen = false;
    });
  }

  function openTeamDialog() {
    teamName = '';
    teamCode = '';
    teamIcon = randomTeamIcon();
    teamDialogOpen = true;
  }

  function randomTeamIcon() {
    return (
      teamIconOptions[Math.floor(Math.random() * teamIconOptions.length)]
        ?.value ?? 'team'
    );
  }

  function openTeamIconPicker(target: 'create' | 'edit') {
    teamIconSearch = '';
    iconPickerTarget = target;
  }

  function selectTeamIcon(icon: string) {
    if (iconPickerTarget === 'create') teamIcon = icon;
    if (iconPickerTarget === 'edit') editTeamIcon = icon;
    iconPickerTarget = null;
    teamIconSearch = '';
  }

  function updateNewUserUsername(value: string) {
    if (!newUserDisplayName || newUserDisplayName === newUserUsername) {
      newUserDisplayName = value;
    }
    newUserUsername = value;
  }

  async function createUser() {
    await action(async () => {
      const result = await api.createUser({
        username: newUserUsername,
        email: newUserEmail,
        phone: newUserPhone,
        display_name: newUserDisplayName,
        password: newUserPassword,
        password_mode: newUserPasswordMode,
        grants: newUserGrants.map((grant) => ({
          scope_id: grant.scopeID,
          role_id: grant.roleID,
          resource_grants: grant.resourceGrants.map((resourceGrant) => ({
            resource_id: resourceGrant.resourceID,
            role_id: resourceGrant.roleID
          }))
        }))
      });
      users = [...users, result.user];
      bindings = [...bindings, ...result.bindings];
      resetUserDialog();
      createdUserCredentials = {
        username: result.user.username,
        password: result.one_time_password
      };
      notice = `用户“${result.user.display_name || result.user.username}”已创建并完成授权`;
    });
  }

  async function createProject() {
    await action(async () => {
      const created = await api.createProject(projectTeamId, {
        name: projectName,
        code: projectCode,
        icon: projectIcon,
        labels: {}
      });
      projects = [...projects, created];
      projectName = '';
      projectCode = '';
      projectIcon = 'FolderKanban';
      notice = `项目“${created.name}”已创建`;
    });
  }

  async function createResource() {
    await action(async () => {
      if (!resourceAddCategory || !resourceAddSubtype || !resourceName.trim()) {
        throw new Error('请先完成基础配置中的资源类型、资源子类型和资源名称。');
      }
      const isProvider = resourceKind === 'AIProvider';
      if (isProvider && providerNameDuplicate()) {
        throw new Error('当前级别已存在同名 AI Provider，请更换名称。');
      }
      if (isProvider && !providerDraftTestPassed()) {
        throw new Error('请先完成默认 Model 的连接测试并确认测试通过。');
      }
      if (resourceKind === 'MCPServer' && !mcpConfigurationValid()) {
        throw new Error('请填写 HTTPS 服务地址并至少配置一个允许的工具。');
      }
      const config = isProvider
        ? providerConfigForCreate()
        : resourceKind === 'MCPServer'
          ? mcpConfigForSave()
        : buildSchemaConfig(createSchema, resourceConfigValues, resourceConfig);
      const credentialId = isProvider
        ? await createProviderCredential()
        : await createResourceCredential(createSchema, resourceSensitiveValues);
      const created = await api.createResource({
        scope_id: selectedScopeId,
        kind: resourceKind,
        subtype:
          resourceAddSubtype ||
          resourceSubtypeFor({ kind: resourceKind, config }),
        name: resourceName,
        status: resourceStatus,
        labels: parseLabels(resourceLabels),
        config,
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      if (isProvider && providerPurposeTags.length > 0) {
        await Promise.all(
          providerPurposeTags.map((purpose) =>
            api.setAIProviderBinding(selectedScopeId, purpose, created.id)
          )
        );
        const currentScopeBindings = await api.aiProviderBindings(selectedScopeId);
        aiProviderBindings = [
          ...aiProviderBindings.filter(
            (binding) => binding.scope_id !== selectedScopeId
          ),
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
      notice = `资源“${created.name}”已创建`;
      resourceEditorOpen = false;
      resourceAddMenuOpen = false;
      resourceAddStep = 1;
      await loadResourceDetails(created.id);
    });
  }

  async function updateProviderFromWorkflow() {
    const provider = resources.find(
      (resource) => resource.id === editingProviderResourceId
    );
    if (!provider) return;
    await action(async () => {
      if (providerNameDuplicate()) {
        throw new Error('当前级别已存在同名 AI Provider，请更换名称。');
      }
      const credentialId = await createProviderCredential(resourceName);
      const updated = await api.updateResource(provider.id, {
        name: resourceName.trim(),
        subtype: resourceSubtypeFor({
          kind: 'AIProvider',
          config: providerConfigForCreate()
        }),
        status: resourceStatus,
        labels: parseLabels(resourceLabels),
        config: providerConfigForCreate(),
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      const existingTags = aiProviderBindings
        .filter(
          (binding) =>
            binding.scope_id === selectedScopeId &&
            binding.provider_resource_id === provider.id
        )
        .map((binding) => binding.tag);
      await Promise.all([
        ...existingTags
          .filter((tag) => !providerPurposeTags.includes(tag))
          .map((tag) => api.removeAIProviderBinding(selectedScopeId, tag)),
        ...providerPurposeTags.map((tag) =>
          api.setAIProviderBinding(selectedScopeId, tag, provider.id)
        )
      ]);
      const currentScopeBindings = await api.aiProviderBindings(selectedScopeId);
      aiProviderBindings = [
        ...aiProviderBindings.filter(
          (binding) => binding.scope_id !== selectedScopeId
        ),
        ...currentScopeBindings
      ];
      resources = resources.map((resource) =>
        resource.id === updated.id ? updated : resource
      );
      selectedResourceId = updated.id;
      editingProviderResourceId = '';
      resourceAddMenuOpen = false;
      resourceAddStep = 1;
      notice = `Provider“${updated.name}”已更新`;
      await loadResourceDetails(updated.id);
    });
  }

  async function deleteSelectedResource() {
    if (!selectedResource) return;
    await action(async () => {
      await api.deleteResource(selectedResource.id);
      resources = resources.filter(
        (resource) => resource.id !== selectedResource.id
      );
      selectedResourceId = '';
      relations = [];
      topology = [];
      notice = '资源已停用并从当前列表移除';
    });
  }

  async function loadResourceDetails(id: string) {
    selectedResourceId = id;
    connectionCheck = null;
    const resource = resources.find((item) => item.id === id);
    if (resource) syncResourceEditor(resource);
    try {
      const [loadedRelations, loadedTopology] = await Promise.all([
        api.relations(id),
        api.topology(id)
      ]);
      relations = loadedRelations;
      topology = loadedTopology.items;
    } catch (error) {
      errorMessage = describeError(error, '资源关系加载失败');
    }
    await loadConnectionCheck(id);
  }

  async function loadConnectionCheck(id: string) {
    const current = resources.find((item) => item.id === id);
    // AIProvider checks use the provider-specific test endpoint and do not
    // have entries in the generic connector check history API. Preserve a
    // just-completed provider result instead of clearing it here.
    if (!current || !resourceHasConnector(current)) {
      resourceConnectionChecks = { ...resourceConnectionChecks, [id]: null };
      return;
    }
    if (current.kind === 'AIProvider') return;
    try {
      const check = await api.latestResourceConnectionCheck(id);
      resourceConnectionChecks = { ...resourceConnectionChecks, [id]: check };
      if (selectedResourceId === id) connectionCheck = check;
    } catch (error) {
      if (error instanceof ApiError && error.status === 404) {
        resourceConnectionChecks = { ...resourceConnectionChecks, [id]: null };
        return;
      }
      if (selectedResourceId === id)
        errorMessage = describeError(error, '连接状态加载失败');
    }
  }

  async function loadResourceConnectionChecks(items: Resource[]) {
    const connectorItems = items.filter(
      (resource) => resource.kind !== 'AIProvider' && resourceHasConnector(resource)
    );
    if (!connectorItems.length) return;
    const checks = await Promise.all(
      connectorItems.map(async (resource) => {
        try {
          return [
            resource.id,
            await api.latestResourceConnectionCheck(resource.id)
          ] as const;
        } catch (error) {
          if (error instanceof ApiError && error.status === 404)
            return [resource.id, null] as const;
          return [resource.id, null] as const;
        }
      })
    );
    resourceConnectionChecks = {
      ...resourceConnectionChecks,
      ...Object.fromEntries(checks)
    };
  }

  async function testResourceConnectionFor(resource: Resource) {
    if (!resourceHasConnector(resource)) return;
    connectionBusy = true;
    errorMessage = '';
    try {
      let check: ConnectionCheck;
      if (resource.kind === 'AIProvider') {
        const config = resource.config ?? {};
        const models = Array.isArray(config.models)
          ? (config.models as Array<Record<string, unknown>>)
          : [];
        const configuredDefault = String(config.default_model ?? '').trim();
        const defaultModel =
          models.find((model) => String(model.name ?? '').trim() === configuredDefault) ??
          models.find((model) => Boolean(model.enabled ?? true));
        const modelName = String(defaultModel?.name ?? '').trim();
        if (!modelName) {
          throw new Error('该 AI Provider 尚未配置可用的默认 Model。');
        }
        const result = await api.testAIProvider(resource.id, {
          scope_id: selectedScopeId || resource.scope_id,
          model_name: modelName,
          stream: Array.isArray(defaultModel?.capabilities) &&
            (defaultModel?.capabilities as unknown[]).includes('stream')
        });
        check = {
          id: `ai-provider-${resource.id}`,
          resource_id: resource.id,
          status: result.status === 'succeeded' ? 'succeeded' : 'failed',
          message: result.message,
          latency_ms: result.latency_ms,
          capabilities: [],
          checked_at: new Date().toISOString()
        };
        // Provider checks are intentionally reflected in the catalog only;
        // the detail panel does not show a separate result notification.
      } else {
        check = await api.testResourceConnection(resource.id);
      }
      if (selectedResourceId === resource.id) connectionCheck = check;
      resourceConnectionChecks = {
        ...resourceConnectionChecks,
        [resource.id]: check
      };
      if (resource.kind !== 'AIProvider') {
        notice =
          check.status === 'succeeded'
            ? `资源“${resource.name}”连接测试通过`
            : `资源“${resource.name}”连接测试失败`;
      }
    } catch (error) {
      const message = describeError(error, '连接测试失败');
      if (resource.kind === 'AIProvider') {
        const failedCheck: ConnectionCheck = {
          id: `ai-provider-${resource.id}`,
          resource_id: resource.id,
          status: 'failed',
          message,
          latency_ms: 0,
          capabilities: [],
          checked_at: new Date().toISOString()
        };
        if (selectedResourceId === resource.id) connectionCheck = failedCheck;
        resourceConnectionChecks = {
          ...resourceConnectionChecks,
          [resource.id]: failedCheck
        };
      } else {
        errorMessage = message;
      }
    } finally {
      connectionBusy = false;
    }
  }

  async function testSelectedResourceConnection() {
    if (!selectedResource || !selectedResourceHasConnector) return;
    await testResourceConnectionFor(selectedResource);
  }

  async function testResourceRowConnection(resource: Resource) {
    // Select the row synchronously so the result is immediately reflected in
    // the row/detail state. Relationship, topology, and historical check
    // loading are supplementary and must not block the actual Provider test.
    selectedResourceId = resource.id;
    syncResourceEditor(resource);
    void loadResourceDetails(resource.id);
    // Use the clicked row's resource directly. The selectedResource reactive
    // value may still refer to the previous row while Svelte flushes updates.
    await testResourceConnectionFor(resource);
  }

  function resourceHasConnector(resource: Resource) {
    return ['AIProvider', 'Kubernetes', 'Prometheus', 'Loki'].includes(
      resource.kind
    );
  }

  function resourceEndpointFor(resource: Resource) {
    if (resource.kind === 'AIProvider')
      return String(resource.config?.base_url ?? '未设置服务地址');
    return String(
      resource.config?.url ??
        resource.config?.endpoint ??
        resource.config?.host ??
        '未设置端点'
    );
  }

  function resourceConnectionLabel(resource: Resource) {
    const check = resourceConnectionChecks[resource.id];
    if (check) {
      return `${check.status === 'succeeded' ? '正常' : '失败'}·${check.latency_ms}ms`;
    }
    if (resource.status === 'active') return '正常';
    if (resource.status === 'disabled') return '已停用';
    return '未知';
  }

  function resourceConnectionClass(resource: Resource) {
    const check = resourceConnectionChecks[resource.id];
    if (check) return check.status === 'succeeded' ? 'active' : 'unknown';
    return resource.status;
  }

  function resourceScopeLabel(resource: Resource) {
    const labels: Record<string, string> = {
      platform: '平台',
      team: '团队',
      project: '项目'
    };
    return labels[scopeType(resource.scope_id)] ?? '资源';
  }

  function resourceLabelsText(resource: Resource) {
    return Object.entries(resource.labels ?? {})
      .map(([key, value]) => (value ? `${key}=${value}` : key))
      .join(', ');
  }

  function providerPurposeLabel(tag: string) {
    const labels: Record<string, string> = {
      default: '默认',
      diagnosis: '诊断',
      inspection: '巡检',
      workflow: '工作流'
    };
    return labels[tag] ?? tag;
  }

  function providerBindingsFor(resource: Resource) {
    return aiProviderBindings
      .filter((binding) => binding.provider_resource_id === resource.id)
      .sort((left, right) => {
        const scopeOrder = { platform: 0, team: 1, project: 2 } as Record<
          string,
          number
        >;
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
    const type = activeScope?.type ?? 'platform';
    return `${labels[type] ?? '当前级别'} · ${activeScope?.name ?? '平台'}`;
  }

  function openResourceEditor(resource: Resource) {
    if (resource.kind === 'AIProvider') {
      openProviderWorkflowForEdit(resource);
      return;
    }
    // Select and hydrate the editor immediately. Relationship and topology
    // requests are supplementary and must not make the edit action appear
    // unresponsive.
    selectedResourceId = resource.id;
    syncResourceEditor(resource);
    resourceEditorOpen = true;
    void loadResourceDetails(resource.id);
  }

  async function updateSelectedResource() {
    if (!selectedResource) return;
    await action(async () => {
      const isProvider = selectedResource.kind === 'AIProvider';
      if (selectedResource.kind === 'MCPServer' && !mcpConfigurationValid()) {
        throw new Error('请填写 HTTPS 服务地址并至少配置一个允许的工具。');
      }
      const config = isProvider
        ? providerConfigForCreate()
        : selectedResource.kind === 'MCPServer'
          ? mcpConfigForSave()
        : buildSchemaConfig(selectedSchema, resourceConfigValues, editResourceConfig);
      const credentialId = isProvider
        ? await createProviderCredential()
        : await createResourceCredential(selectedSchema, editResourceSensitiveValues);
      const updated = await api.updateResource(selectedResource.id, {
        name: editResourceName,
        subtype: resourceSubtypeFor({ kind: selectedResource.kind, config }),
        status: editResourceStatus,
        labels: parseLabels(editResourceLabels),
        config,
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      resources = resources.map((resource) =>
        resource.id === updated.id ? updated : resource
      );
      syncResourceEditor(updated);
      notice = `资源“${updated.name}”已更新`;
    });
  }

  async function toggleResourceEnabled(resource: Resource, enabled: boolean) {
    if (!resourceCanManage(resource, 'resource:update')) return;
    await action(async () => {
      const updated = await api.updateResource(resource.id, {
        status: enabled ? 'active' : 'disabled',
        ...(resource.kind === 'AIProvider'
          ? { config: { ...(resource.config ?? {}), enabled } }
          : {})
      });
      resources = resources.map((item) => (item.id === updated.id ? updated : item));
      if (selectedResourceId === updated.id) syncResourceEditor(updated);
      notice = `资源“${updated.name}”已${enabled ? '启用' : '停用'}`;
    });
  }

  function syncResourceEditor(resource: Resource) {
    editResourceName = resource.name;
    editResourceStatus = resource.status;
    editResourceLabels = Object.entries(resource.labels ?? {})
      .map(([key, value]) => `${key}=${value}`)
      .join(', ');
    editResourceConfig = JSON.stringify(resource.config ?? {}, null, 2);
    resourceConfigValues = Object.fromEntries(
      Object.entries(resource.config ?? {}).map(([key, value]) => [
        key,
        String(value)
      ])
    );
    editResourceSensitiveValues = {};
    if (resource.kind === 'MCPServer') {
      const config = resource.config ?? {};
      mcpTransport = String(config.transport ?? 'streamable_http');
      mcpURL = String(config.url ?? '');
      mcpToolAllowlist = Array.isArray(config.tool_allowlist)
        ? config.tool_allowlist.map(String).join('\n')
        : String(config.tool_allowlist ?? '');
      mcpTimeoutSeconds = Number(config.timeout_seconds ?? 10);
      mcpMaxResponseBytes = Number(config.max_response_bytes ?? 1048576);
    }
    if (resource.kind === 'AIProvider') syncProviderEditor(resource);
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
        maxOutputTokens: Number(model.max_output_tokens ?? 8192),
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

  function resetResourceConfig() {
    resourceConfigValues = {};
    resourceSensitiveValues = {};
    resourceConfig = '{}';
    mcpTransport = 'streamable_http';
    mcpURL = '';
    mcpToolAllowlist = '';
    mcpTimeoutSeconds = 10;
    mcpMaxResponseBytes = 1048576;
  }

  function emptyProviderModelDraft(): ProviderModelDraft {
    return {
      name: '',
      contextWindowTokens: 128000,
      maxOutputTokens: 8192,
      temperature: 0.7,
      temperatureMutable: true,
      capabilities: ['text', 'tool_calling', 'structured_output', 'stream'],
      enabled: true,
      priority: 0
    };
  }

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

  function providerConfigForCreate(): Record<string, unknown> {
    return {
      provider_type: providerType,
      protocol: providerProtocol,
      base_url: providerBaseURL.trim(),
      timeout_seconds: providerTimeoutSeconds,
      max_concurrency: providerMaxConcurrency,
      rate_limit_per_minute: providerRateLimitPerMinute,
      // Resource status is the single source of truth for whether a Provider
      // can be used. Keep the legacy config field in sync for older readers.
      enabled: resourceStatus === 'active',
      default_model: providerDefaultModel,
      models: providerModels.map((model) => ({
        name: model.name.trim(),
        context_window_tokens: model.contextWindowTokens,
        max_output_tokens: model.maxOutputTokens,
        temperature: model.temperature,
        temperature_mutable: model.temperatureMutable,
        capabilities: model.capabilities,
        enabled: model.enabled,
        priority: model.priority
      }))
    };
  }

  function mcpConfigForSave(): Record<string, unknown> {
    return {
      transport: mcpTransport,
      url: mcpURL.trim(),
      tool_allowlist: mcpToolAllowlist
        .split(/[\n,]/)
        .map((item) => item.trim())
        .filter(Boolean),
      timeout_seconds: mcpTimeoutSeconds,
      max_response_bytes: mcpMaxResponseBytes
    };
  }

  function mcpConfigurationValid() {
    if (mcpTransport !== 'streamable_http' || !mcpURL.trim()) return false;
    try {
      const url = new URL(mcpURL.trim());
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return false;
    } catch {
      return false;
    }
    const tools = mcpConfigForSave().tool_allowlist;
    return Array.isArray(tools) && tools.length > 0;
  }

  function providerTypeLabel(type: unknown) {
    return providerTypeOptions.find((option) => option.value === String(type))?.label ?? String(type || 'Provider');
  }

  function providerModelsForResource(resource: Resource) {
    const models = Array.isArray(resource.config?.models) ? resource.config.models : [];
    return models as Array<Record<string, unknown>>;
  }

  function providerDefaultModelForResource(resource: Resource) {
    const models = providerModelsForResource(resource);
    const configured = String(resource.config?.default_model ?? '').trim();
    return (
      models.find((model) => String(model.name ?? '').trim() === configured) ??
      models.find((model) => model.enabled !== false) ??
      models[0]
    );
  }

  function providerModelCapabilities(model: Record<string, unknown> | undefined) {
    if (!model || !Array.isArray(model.capabilities)) return [];
    return (model.capabilities as unknown[]).map((capability) =>
      providerCapabilityOptions.find((item) => item.value === String(capability))?.label ?? String(capability)
    );
  }

  function providerDefaultModelDraft() {
    return providerModels.find((model) => model.name === providerDefaultModel);
  }

  function providerPurposeMissingCapabilities(purpose: string) {
    const option = providerPurposeOptions.find((item) => item.value === purpose);
    const defaultModel = providerDefaultModelDraft();
    if (!option || !defaultModel) return option?.requiredCapabilities ?? [];
    return option.requiredCapabilities.filter(
      (capability) => !defaultModel.capabilities.includes(capability)
    );
  }

  function providerPurposeAvailable(purpose: string) {
    return Boolean(providerDefaultModelDraft()) &&
      providerPurposeMissingCapabilities(purpose).length === 0;
  }

  function providerPurposeUnavailableReason(purpose: string) {
    if (!providerDefaultModelDraft()) return '尚未选择默认 Model。';
    const missing = providerPurposeMissingCapabilities(purpose)
      .map(
        (capability) =>
          providerCapabilityOptions.find((item) => item.value === capability)
            ?.label ?? capability
      )
      .join('、');
    return missing ? `当前默认 Model 缺失：${missing}` : '';
  }

  function toggleProviderPurpose(purpose: string) {
    if (!providerPurposeAvailable(purpose)) return;
    providerPurposeTags = providerPurposeTags.includes(purpose)
      ? providerPurposeTags.filter((tag) => tag !== purpose)
      : [...providerPurposeTags, purpose];
  }

  function providerDraftSignature() {
    return providerDraftCurrentSignature;
  }

  function providerDraftTestPassed() {
    return Boolean(
      providerDraftTest?.signature === providerDraftSignature() &&
        providerDraftTest.result?.status === 'succeeded'
    );
  }

  async function testProviderDraftConnection() {
    providerDraftTestBusy = true;
    // Keep a visible in-flight result so the summary never falls back to
    // "尚未核验" while the request is being processed.
    providerDraftTest = {
      signature: providerDraftSignature(),
      error: '正在测试默认 Model，请稍候…'
    };
    const defaultModel = providerModels.find(
      (model) => model.name === providerDefaultModel
    );
    if (!selectedScopeId) {
      providerDraftTest = {
        signature: providerDraftSignature(),
        error: '未选择资源归属级别，无法执行连接测试。'
      };
      providerDraftTestBusy = false;
      return;
    }
    if (!providerAPIKey.trim()) {
      providerDraftTest = {
        signature: providerDraftSignature(),
        error: 'API Key 为必填项，请先填写后再进行连接测试。'
      };
      providerDraftTestBusy = false;
      return;
    }
    if (!defaultModel) {
      providerDraftTest = {
        signature: providerDraftSignature(),
        error: '尚未选择默认 Model，请先在 Model 配置步骤中选择。'
      };
      providerDraftTestBusy = false;
      return;
    }
    if (!providerBaseURLValid()) {
      providerDraftTest = {
        signature: providerDraftSignature(),
        error: '服务地址无效，请返回 Provider 配置检查地址。'
      };
      providerDraftTestBusy = false;
      return;
    }
    const signature = providerDraftSignature();
    try {
      const result = await api.testDraftAIProvider({
        scope_id: selectedScopeId,
        provider_type: providerType,
        base_url: providerBaseURL.trim(),
        model_name: defaultModel.name,
        api_key: providerAPIKey,
        context_window: defaultModel.contextWindowTokens,
        temperature: defaultModel.temperature,
        capabilities: defaultModel.capabilities,
        stream: defaultModel.capabilities.includes('stream')
      });
      providerDraftTest = { signature, result };
    } catch (error) {
      providerDraftTest = {
        signature,
        error: describeError(error, 'Provider 连接测试失败')
      };
    } finally {
      providerDraftTestBusy = false;
    }
  }

  async function createProviderCredential(name = resourceName) {
    if (!providerAPIKey.trim() || !selectedScopeId) {
      throw new Error('API Key 为必填项，请填写后再继续。');
    }
    const credential = await api.createCredential({
      scope_id: selectedScopeId,
      name: `${name || 'AI Provider'} API Key`,
      purpose: 'AI Provider 访问凭据',
      secret: providerAPIKey.trim()
    });
    return credential.id;
  }

  function addProviderModel() {
    providerModelConfigurationAttempted = true;
    if (!providerModelDraftComplete()) {
      providerModelValidationMessage =
        '请补全带 * 的 Model 字段，并至少选择一项能力。';
      return;
    }
    const model = {
      ...providerModelDraft,
      name: providerModelDraft.name.trim(),
      capabilities: [...providerModelDraft.capabilities]
    };
    if (
      providerModels.some(
        (item) =>
          item.name === model.name && item.name !== editingProviderModelName
      )
    ) {
      providerModelValidationMessage = `Model “${model.name}”已存在，请使用其他名称。`;
      return;
    }
    if (editingProviderModelName) {
      const previousName = editingProviderModelName;
      providerModels = providerModels.map((item) =>
        item.name === previousName ? model : item
      );
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

  function editProviderModel(model: ProviderModelDraft) {
    editingProviderModelName = model.name;
    providerModelDraft = {
      ...model,
      capabilities: [...model.capabilities]
    };
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

  function setProviderDefaultModel(name: string) {
    providerDefaultModel = name;
    providerModels = providerModels.map((model) =>
      model.name === name ? { ...model, enabled: true } : model
    );
  }

  function setProviderModelEnabled(name: string, enabled: boolean) {
    if (name === providerDefaultModel && !enabled) return;
    providerModels = providerModels.map((model) =>
      model.name === name ? { ...model, enabled } : model
    );
  }

  function toggleProviderModelCapability(capability: string) {
    providerModelDraft = {
      ...providerModelDraft,
      capabilities: providerModelDraft.capabilities.includes(capability)
        ? providerModelDraft.capabilities.filter((item) => item !== capability)
        : [...providerModelDraft.capabilities, capability]
    };
  }

  function selectProviderType(type: string) {
    providerType = type;
    const preset = providerTypeOptions.find((item) => item.value === type);
    if (preset?.baseURL) providerBaseURL = preset.baseURL;
  }

  function resourceSchemaForSelection(category: string, subtype: string) {
    return (
      schemas.find(
        (schema) =>
          resourceCategoryFor(schema as unknown as { kind: string }) ===
            category &&
          resourceSubtypeFor(schema as unknown as { kind: string }) === subtype
      ) ??
      schemas.find(
        (schema) =>
          resourceCategoryFor(schema as unknown as { kind: string }) ===
          category
      ) ??
      schemas[0]
    );
  }

  function toggleResourceAddMenu() {
    resourceAddMenuOpen = !resourceAddMenuOpen;
    resourceEditorOpen = false;
    resourceAddStep = 1;
    resourceTypeSelectionAttempted = false;
    resourceBasicConfigurationAttempted = false;
    if (resourceAddMenuOpen) {
      resourceStatus = 'active';
      resourceAddCategory = resourceCategory === '全部' ? '' : resourceCategory;
      resourceAddSubtype = resourceSubtype === '全部' ? '' : resourceSubtype;
    }
  }

  function chooseResourceAddSubtype(
    category: string,
    subtype: string,
    resetDraft = true
  ) {
    const schema = resourceSchemaForSelection(category, subtype);
    resourceAddCategory = category;
    resourceAddSubtype = subtype;
    resourceKind =
      category === 'LLM' && subtype === 'Provider'
        ? 'AIProvider'
        : (schema?.kind ?? '');
    resourceCategory = category;
    resourceSubtype = subtype;
    if (resetDraft) {
      resetResourceConfig();
      resetProviderDraft();
    }
    resourceAddStep = 1;
    resourceTypeSelectionAttempted = false;
    resourceBasicConfigurationAttempted = false;
    resourceAddMenuOpen = true;
    resourceEditorOpen = false;
  }

  function selectResourceAddCategory(category: string) {
    resourceAddCategory = category;
    resourceAddSubtype = '';
    resourceKind = '';
  }

  function continueResourceAdd() {
    resourceTypeSelectionAttempted = true;
    resourceBasicConfigurationAttempted = true;
    if (!resourceBasicConfigurationComplete()) return;
    resourceTypeSelectionAttempted = false;
    resourceBasicConfigurationAttempted = false;
    chooseResourceAddSubtype(
      resourceAddCategory,
      resourceAddSubtype,
      !editingProviderResourceId
    );
    resourceAddStep = 2;
  }

  function resourceBasicConfigurationComplete() {
    return Boolean(
      resourceAddCategory &&
        resourceAddSubtype &&
        resourceName.trim() &&
        selectedScopeId
    );
  }

  function providerConfigurationComplete() {
    return Boolean(
      !providerNameDuplicate() &&
        providerType &&
        providerBaseURLValid() &&
        providerAPIKey.trim()
    );
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

  function openProviderWorkflowForEdit(resource: Resource) {
    const project = projects.find((item) => item.scope.id === resource.scope_id);
    const team = teams.find((item) => item.scope.id === resource.scope_id);
    if (project) {
      selectedTeamId = project.team_id;
      projectTeamId = project.team_id;
      selectedProjectId = project.id;
    } else if (team) {
      selectedTeamId = team.id;
      projectTeamId = team.id;
      selectedProjectId = '';
    } else {
      selectedTeamId = '';
      projectTeamId = '';
      selectedProjectId = '';
    }
    selectedScopeId = resource.scope_id;
    selectedResourceId = resource.id;
    resourceKind = 'AIProvider';
    resourceAddCategory = 'LLM';
    resourceAddSubtype = 'Provider';
    resourceCategory = 'LLM';
    resourceSubtype = 'Provider';
    editingProviderResourceId = resource.id;
    syncProviderEditor(resource);
    providerPurposeTags = aiProviderBindings
      .filter(
        (binding) =>
          binding.scope_id === resource.scope_id &&
          binding.provider_resource_id === resource.id
      )
      .map((binding) => binding.tag);
    providerDraftTest = null;
    resourceAddStep = 1;
    resourceBasicConfigurationAttempted = false;
    resourceEditorOpen = false;
    resourceAddMenuOpen = true;
    if (resource.credential_id) {
      providerAPIKeyLoading = true;
      void api.credentialSecret(resource.credential_id).then(
        (credential) => {
          // Keep the existing credential in the editor so editing a Provider
          // does not silently invalidate its required connection secret.
          if (editingProviderResourceId === resource.id) {
            providerAPIKey = credential.secret;
            providerAPIKeyLoading = false;
          }
        },
        (error) => {
          if (editingProviderResourceId === resource.id) {
            providerAPIKey = '';
            providerAPIKeyLoading = false;
            errorMessage = describeError(error, '无法读取 Provider API Key');
          }
        }
      );
    } else {
      providerAPIKeyLoading = false;
    }
  }

  function providerBaseURLValid() {
    try {
      const url = new URL(providerBaseURL.trim());
      return url.protocol === 'http:' || url.protocol === 'https:';
    } catch {
      return false;
    }
  }

  function providerConfigurationIssues() {
    const issues: string[] = [];
    if (providerNameDuplicate()) issues.push('资源名称已存在');
    if (!providerType) issues.push('Provider 类型');
    if (!providerBaseURL.trim()) issues.push('服务地址');
    else if (!providerBaseURLValid()) issues.push('服务地址格式');
    if (!providerAPIKey.trim()) issues.push('API Key');
    return issues;
  }

  function providerModelDraftComplete() {
    return Boolean(
      providerModelDraft.name.trim() &&
      providerModelDraft.contextWindowTokens > 0 &&
      providerModelDraft.capabilities.length > 0
    );
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
        resourceAddStep = 4;
      }
      return;
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
      return '当前选择的用途与默认 Model 的能力不匹配，请调整用途标记或 Model 能力。';
    if (
      resourceKind === 'AIProvider' &&
      resourceAddStep === 4 &&
      providerDraftTest?.error
    )
      return `连接测试失败：${providerDraftTest.error}`;
    if (
      resourceKind === 'AIProvider' &&
      resourceAddStep === 4 &&
      providerSummaryAttempted &&
      !providerDraftTestPassed()
    )
      return '请先完成默认 Model 的连接测试并确认测试通过。';
    return '';
  }

  async function submitProviderCreate() {
    providerSummaryAttempted = true;
    if (providerAPIKeyLoading) {
      return;
    }
    if (!providerPurposeConfigurationValid()) return;
    if (!providerDraftTestPassed()) return;
    if (editingProviderResourceId) {
      await updateProviderFromWorkflow();
      return;
    }
    await createResource();
  }

  function providerPurposeConfigurationValid() {
    return providerPurposeTags.every((purpose) => providerPurposeAvailable(purpose));
  }

  function resourceAddStepTitle(step: number, kind: string) {
    if (step === 1) return '基础配置';
    if (kind !== 'AIProvider') return '配置资源';
    return ['Provider 配置', 'Model 配置', '总结核验'][step - 2] ?? '配置资源';
  }

  function resourceSchemaFieldRequired(key: string) {
    return createSchema?.schema.required?.includes(key) ?? false;
  }

  function buildSchemaConfig(
    schema: ResourceSchema | undefined,
    values: Record<string, string>,
    raw: string
  ) {
    if (!schema?.schema.properties)
      return JSON.parse(raw) as Record<string, unknown>;
    return Object.fromEntries(
      Object.entries(schema.schema.properties)
        .filter(
          ([key, field]) =>
            !field.sensitive && (values[key] ?? '').trim() !== ''
        )
        .map(([key, field]) => [
          key,
          normalizeFieldValue(values[key], field.type)
        ])
    );
  }

  function normalizeFieldValue(value: string, type?: string): unknown {
    if (type === 'integer' || type === 'number') return Number(value);
    if (type === 'boolean') return value === 'true';
    if (type === 'array') {
      try {
        const parsed: unknown = JSON.parse(value);
        if (Array.isArray(parsed)) return parsed;
      } catch {
        // Comma-separated values remain convenient for simple string arrays.
      }
      return value
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean);
    }
    return value;
  }

  async function createResourceCredential(
    schema: ResourceSchema | undefined,
    values: Record<string, string>
  ) {
    const secret = Object.fromEntries(
      Object.entries(values).filter(([, value]) => value.trim() !== '')
    );
    const scopeId = selectedResource?.scope_id ?? selectedScopeId;
    if (!scopeId || !schema || Object.keys(secret).length === 0) return '';
    const credential = await api.createCredential({
      scope_id: scopeId,
      name: `${resourceName || selectedResource?.name || schema.display_name} connection`,
      purpose: `${schema.display_name} 敏感连接信息`,
      secret: JSON.stringify(secret)
    });
    return credential.id;
  }

  async function createRelation() {
    if (!selectedResource) return;
    await action(async () => {
      await api.createRelation(selectedResource.id, {
        target_resource_id: relationTarget,
        relation_type: relationType,
        attributes: {},
        confirmed: true
      });
      relationTarget = '';
      notice = '资源关系已建立';
      await loadResourceDetails(selectedResource.id);
    });
  }

  async function deleteRelation(relation: Relation) {
    if (!selectedResource) return;
    await action(async () => {
      await api.deleteRelation(selectedResource.id, relation.id);
      await loadResourceDetails(selectedResource.id);
      notice = '资源关系已删除';
    });
  }

  async function createGroup() {
    await action(async () => {
      const created = await api.createGroup({
        scope_id: groupScopeId,
        name: groupName,
        description: groupDescription
      });
      groups = [...groups, created];
      groupName = '';
      groupDescription = '';
      notice = `成员组“${created.name}”已创建`;
    });
  }

  async function createBinding() {
    await action(async () => {
      const created = await api.createBinding({
        subject_type: bindingSubjectType,
        subject_id: bindingSubjectId,
        role_id: bindingRoleId,
        scope_id: bindingScopeId
      });
      bindings = [...bindings, created];
      notice = '角色绑定已创建';
    });
  }

  async function deleteBinding(binding: RoleBinding) {
    await action(async () => {
      await api.deleteBinding(binding.id);
      bindings = bindings.filter((item) => item.id !== binding.id);
      notice = '角色绑定已删除';
    });
  }

  async function createResourceBinding() {
    await action(async () => {
      const created = await api.createResourceBinding({
        subject_type: resourceBindingSubjectType,
        subject_id: resourceBindingSubjectId,
        role_id: resourceBindingRoleId,
        resource_id: resourceBindingResourceId
      });
      resourceBindings = [...resourceBindings, created];
      notice = '资源角色绑定已创建';
    });
  }

  async function deleteResourceBinding(binding: ResourceRoleBinding) {
    await action(async () => {
      await api.deleteResourceBinding(binding.id);
      resourceBindings = resourceBindings.filter(
        (item) => item.id !== binding.id
      );
      notice = '资源角色绑定已删除';
    });
  }

  async function action(operation: () => Promise<void>) {
    busy = true;
    errorMessage = '';
    try {
      await operation();
    } catch (error) {
      errorMessage = describeError(error, '操作失败');
    } finally {
      busy = false;
    }
  }

  function describeError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 403) return '当前账号没有执行此操作的权限。';
      if (error.status === 401) return '会话已过期，请重新登录。';
      if (
        error.status === 409 &&
        error.message === 'Resource conflicts with existing data'
      ) {
        return '当前级别下已有同名 AI 接入或凭据冲突，请更换名称后重试。';
      }
      return error.message || fallback;
    }
    if (error instanceof SyntaxError) return '配置必须是有效的 JSON 对象。';
    if (error instanceof Error) return error.message || fallback;
    return fallback;
  }

  function buildScopeChoices(
    currentPlatform: Platform | null,
    currentTeams: Team[],
    currentProjects: Project[]
  ): ScopeChoice[] {
    const choices: ScopeChoice[] = currentPlatform
      ? [
          {
            id: currentPlatform.scope.id,
            type: 'platform',
            name: currentPlatform.name
          }
        ]
      : [];
    for (const team of currentTeams)
      choices.push({
        id: team.scope.id,
        type: 'team',
        name: team.name,
        parentId: currentPlatform?.scope.id
      });
    for (const project of currentProjects)
      choices.push({
        id: project.scope.id,
        type: 'project',
        name: project.name,
        parentId: currentTeams.find((team) => team.id === project.team_id)
          ?.scope.id
      });
    return choices;
  }

  function scopeName(id: string) {
    return (
      scopeChoices.find((scope) => scope.id === id)?.name ?? id.slice(0, 8)
    );
  }

  function userRoleBindings(userID: string) {
    const groupIDs = groups
      .filter((group) => groupMembers[group.id]?.includes(userID))
      .map((group) => group.id);
    return bindings.filter(
      (binding) =>
        (binding.subject_type === 'user' && binding.subject_id === userID) ||
        (binding.subject_type === 'group' &&
          groupIDs.includes(binding.subject_id))
    );
  }

  function userRoles(userID: string) {
    const names = userRoleBindings(userID).map((binding) => binding.role_name);
    return [...new Set(names)];
  }

  function roleLabel(name: string) {
    const labels: Record<string, string> = {
      PlatformAdmin: '平台管理员',
      PlatformOperator: '平台操作员',
      PlatformViewer: '平台观察员',
      TeamAdmin: '团队管理员',
      TeamOperator: '团队操作员',
      TeamViewer: '团队观察员',
      ProjectAdmin: '项目管理员',
      ProjectOperator: '项目操作员',
      ProjectViewer: '项目观察员',
      ResourceAdmin: '资源管理员',
      ResourceOperator: '资源操作员',
      ResourceViewer: '资源观察员'
    };
    return labels[name] ?? name;
  }

  function grantRoleLabel(name: string) {
    const labels: Record<string, string> = {
      PlatformAdmin: '管理员',
      PlatformOperator: '操作员',
      PlatformViewer: '观察员',
      TeamAdmin: '管理员',
      TeamOperator: '操作员',
      TeamViewer: '观察员',
      ProjectAdmin: '管理员',
      ProjectOperator: '操作员',
      ProjectViewer: '观察员'
    };
    return labels[name] ?? roleLabel(name);
  }

  function grantScopeLabel(scopeType: NewUserGrant['scopeType']) {
    const labels: Record<NewUserGrant['scopeType'], string> = {
      platform: '平台',
      team: '团队',
      project: '项目'
    };
    return labels[scopeType];
  }

  function resourceGrantViewerRole(scopeType: string) {
    const rolesByScope: Record<string, string> = {
      platform: 'PlatformViewer',
      team: 'TeamViewer',
      project: 'ProjectViewer'
    };
    return rolesByScope[scopeType] ?? '';
  }

  function roleScopeLabel(scopeType: string) {
    const labels: Record<string, string> = {
      platform: '平台级',
      team: '团队级',
      project: '项目级',
      resource: '资源级'
    };
    return labels[scopeType] ?? scopeType;
  }

  function userPermissions(userID: string) {
    const roleIDs = new Set(
      userRoleBindings(userID).map((binding) => binding.role_id)
    );
    return [
      ...new Set(
        roles
          .filter((role) => roleIDs.has(role.id))
          .flatMap((role) => role.permissions.map(String))
      )
    ];
  }

  const permissionDescriptions: Record<string, string> = {
    'organization:read': '查看组织、平台和级别信息',
    'team:manage': '创建、编辑和停用团队',
    'project:manage': '创建、编辑和停用项目',
    'member:grant': '管理用户、用户组和角色授权',
    'resource:read': '查看资源列表、配置和详情',
    'resource:create': '创建资源',
    'resource:update': '编辑资源配置',
    'resource:delete': '删除或停用资源',
    'resource:use': '使用资源执行连接测试或业务调用',
    'engine:manage': '管理 AI 引擎及其级别内的默认 AIProvider',
    'credential:manage': '管理凭据及其关联配置',
    'credential:test': '测试凭据连接',
    'relation:manage': '管理资源之间的关联关系',
    'discovery:run': '启动集群或资源发现',
    'discovery:import': '导入发现结果',
    'diagnosis:start': '启动 AI 诊断',
    'diagnosis:read': '查看诊断记录和结果',
    'inspection:manage': '管理自动巡检策略',
    'inspection:execute': '执行自动巡检',
    'operation:approve': '审批受控操作',
    'audit:read': '查看审计日志'
  };

  function permissionDescription(permission: string) {
    return permissionDescriptions[permission] ?? '暂无权限说明';
  }

  function scopeContains(ancestorID: string, scopeID: string) {
    let current = scopeChoices.find((scope) => scope.id === scopeID);
    while (current) {
      if (current.id === ancestorID) return true;
      current = current.parentId
        ? scopeChoices.find((scope) => scope.id === current?.parentId)
        : undefined;
    }
    return false;
  }

  function actorPermissionsAtScope(scopeID: string) {
    if (!currentUser || !scopeID) return [];
    if (isPlatformAdmin) {
      return [
        ...new Set(roles.flatMap((role) => role.permissions.map(String)))
      ];
    }
    const roleIDs = new Set(
      userRoleBindings(currentUser.id)
        .filter((binding) => scopeContains(binding.scope_id, scopeID))
        .map((binding) => binding.role_id)
    );
    return [
      ...new Set(
        roles
          .filter((role) => roleIDs.has(role.id))
          .flatMap((role) => role.permissions.map(String))
      )
    ];
  }

  function grantableRolesForScope(scopeID: string) {
    if (!scopeID) return [];
    const permissions = new Set(actorPermissionsAtScope(scopeID));
    return roles.filter(
      (role) =>
        role.scope_type === scopeType(scopeID) &&
        role.permissions.every((permission) => permissions.has(permission))
    );
  }

  function canManageTeam(_team: Team) {
    return isPlatformAdmin;
  }

  function canManageUser(user: User) {
    return (
      user.can_manage ?? (accessCanManageUsers && user.id !== currentUser?.id)
    );
  }

  function userScopeNames(userID: string) {
    return [
      ...new Set(
        userRoleBindings(userID).map((binding) => scopeName(binding.scope_id))
      )
    ];
  }

  function toggleTeamAccess(teamID: string) {
    teamAccessExpanded = {
      ...teamAccessExpanded,
      [teamID]: !teamAccessExpanded[teamID]
    };
  }

  function resetUserDialog() {
    newUserUsername = '';
    newUserEmail = '';
    newUserPhone = '';
    newUserDisplayName = '';
    newUserPassword = '';
    newUserPasswordMode = 'generated';
    createdUserCredentials = null;
    const preferredScopeID = selectedTeam?.scope.id ?? platform?.scope.id ?? '';
    const preferredScope =
      manageableScopeChoices.find((scope) => scope.id === preferredScopeID) ??
      manageableScopeChoices[0];
    newUserGrants = preferredScope
      ? [
          {
            scopeType: preferredScope.type as NewUserGrant['scopeType'],
            scopeID: preferredScope.id,
            roleID: '',
            resourceGrants: []
          }
        ]
      : [];
  }

  function addNewUserGrant() {
    const scope = manageableScopeChoices[0];
    if (!scope) return;
    newUserGrants = [
      ...newUserGrants,
      {
        scopeType: scope.type as NewUserGrant['scopeType'],
        scopeID: scope.id,
        roleID: '',
        resourceGrants: []
      }
    ];
  }

  function updateNewUserGrant(index: number, updates: Partial<NewUserGrant>) {
    newUserGrants = newUserGrants.map((grant, grantIndex) =>
      grantIndex === index ? { ...grant, ...updates } : grant
    );
  }

  function chooseNewUserGrantType(
    index: number,
    type: NewUserGrant['scopeType']
  ) {
    const scope = manageableScopeChoices.find((item) => item.type === type);
    updateNewUserGrant(index, {
      scopeType: type,
      scopeID: scope?.id ?? '',
      roleID: '',
      resourceGrants: []
    });
  }

  function removeNewUserGrant(index: number) {
    newUserGrants = newUserGrants.filter(
      (_, grantIndex) => grantIndex !== index
    );
  }

  function newUserGrantScopes(type: NewUserGrant['scopeType']) {
    return manageableScopeChoices.filter((scope) => scope.type === type);
  }

  function newUserGrantRoles(grant: NewUserGrant) {
    return grantableRolesForScope(grant.scopeID);
  }

  function newUserGrantIsScopeViewer(grant: NewUserGrant) {
    return (
      roles.find((role) => role.id === grant.roleID)?.name ===
      resourceGrantViewerRole(grant.scopeType)
    );
  }

  function newUserGrantResources(grant: NewUserGrant) {
    return resources.filter(
      (resource) =>
        resourceVisibleToScope(grant.scopeID, resource.scope_id) &&
        resource.status === 'active'
    );
  }

  function resourceVisibleToScope(
    viewerScopeID: string,
    resourceScopeID: string
  ) {
    if (!viewerScopeID || !resourceScopeID) return false;
    if (viewerScopeID === resourceScopeID) return true;
    const parentByID = new Map(
      scopeChoices.map((scope) => [scope.id, scope.parentId ?? ''])
    );
    const reaches = (start: string, target: string) => {
      let current = start;
      const visited = new Set<string>();
      while (current && !visited.has(current)) {
        if (current === target) return true;
        visited.add(current);
        current = parentByID.get(current) ?? '';
      }
      return false;
    };
    return (
      reaches(viewerScopeID, resourceScopeID) ||
      reaches(resourceScopeID, viewerScopeID)
    );
  }

  function newUserGrantResourceRoles(grant: NewUserGrant) {
    return resourceRoles.filter(
      (resourceRole) =>
        viewerResourceRoleAllowed(resourceRole) &&
        resourceRole.permissions.every((permission) =>
          actorPermissionsAtScope(grant.scopeID).includes(String(permission))
        )
    );
  }

  function viewerResourceRoleAllowed(resourceRole: ResourceRoleDefinition) {
    return !resourceRole.permissions.some((permission) =>
      ['resource:create', 'resource:update', 'resource:delete'].includes(
        String(permission)
      )
    );
  }

  function addNewUserResourceGrant(index: number) {
    const grant = newUserGrants[index];
    if (!grant) return;
    updateNewUserGrant(index, {
      resourceGrants: [...grant.resourceGrants, { resourceID: '', roleID: '' }]
    });
  }

  function updateNewUserResourceGrant(
    grantIndex: number,
    resourceIndex: number,
    updates: Partial<NewUserResourceGrant>
  ) {
    const grant = newUserGrants[grantIndex];
    if (!grant) return;
    updateNewUserGrant(grantIndex, {
      resourceGrants: grant.resourceGrants.map((resourceGrant, index) =>
        index === resourceIndex
          ? { ...resourceGrant, ...updates }
          : resourceGrant
      )
    });
  }

  function removeNewUserResourceGrant(
    grantIndex: number,
    resourceIndex: number
  ) {
    const grant = newUserGrants[grantIndex];
    if (!grant) return;
    updateNewUserGrant(grantIndex, {
      resourceGrants: grant.resourceGrants.filter(
        (_, index) => index !== resourceIndex
      )
    });
  }

  function openEditTeam(team: Team) {
    editingTeam = team;
    editTeamName = team.name;
    editTeamIcon = team.icon;
    editTeamStatus = team.status;
  }

  async function saveTeam() {
    if (!editingTeam) return;
    const teamID = editingTeam.id;
    await action(async () => {
      const updated = await api.updateTeam(teamID, {
        name: editTeamName,
        icon: editTeamIcon,
        status: editTeamStatus
      });
      teams = teams.map((team) => (team.id === teamID ? updated : team));
      editingTeam = null;
      notice = `团队“${updated.name}”已更新`;
    });
  }

  function openEditUser(user: User) {
    editingUser = user;
    editUserDisplayName = user.display_name || user.username;
    passwordResetCredentials = null;
    editUserResourceRoleId = '';
    editUserResourceId = '';
    const directBindings = bindings.filter(
      (binding) =>
        binding.subject_type === 'user' && binding.subject_id === user.id
    );
    editUserScopeId =
      directBindings.find((binding) =>
        manageableScopeChoices.some((scope) => scope.id === binding.scope_id)
      )?.scope_id ??
      manageableScopeChoices[0]?.id ??
      '';
    editUserRoleIds = directBindings
      .filter((binding) => binding.scope_id === editUserScopeId)
      .map((binding) => binding.role_id);
  }

  function chooseEditUserScope(scopeID: string) {
    editUserScopeId = scopeID;
    editUserRoleIds = editingUser
      ? bindings
          .filter(
            (binding) =>
              binding.subject_type === 'user' &&
              binding.subject_id === editingUser?.id &&
              binding.scope_id === scopeID
          )
          .map((binding) => binding.role_id)
      : [];
    editUserResourceRoleId = '';
    editUserResourceId = '';
  }

  async function saveUser() {
    if (!editingUser || !editUserScopeId) return;
    const user = editingUser;
    const userID = user.id;
    await action(async () => {
      if (editUserDisplayName.trim() !== user.display_name) {
        const updatedUser = await api.updateUser(userID, {
          display_name: editUserDisplayName.trim()
        });
        users = users.map((user) => (user.id === userID ? updatedUser : user));
        editingUser = updatedUser;
      }
      const existing = bindings.filter(
        (binding) =>
          binding.subject_type === 'user' &&
          binding.subject_id === userID &&
          binding.scope_id === editUserScopeId
      );
      const desired = new Set(editUserRoleIds);
      const created: RoleBinding[] = [];
      for (const roleID of editUserRoleIds) {
        if (!existing.some((binding) => binding.role_id === roleID)) {
          created.push(
            await api.createBinding({
              subject_type: 'user',
              subject_id: userID,
              role_id: roleID,
              scope_id: editUserScopeId
            })
          );
        }
      }
      const removedIDs: string[] = [];
      for (const binding of existing) {
        if (!desired.has(binding.role_id)) {
          await api.deleteBinding(binding.id);
          removedIDs.push(binding.id);
        }
      }
      bindings = [
        ...bindings.filter((binding) => !removedIDs.includes(binding.id)),
        ...created
      ];
      editingUser = null;
      notice = '用户授权已更新';
    });
  }

  async function resetManagedUserPassword() {
    if (!editingUser) return;
    const user = editingUser;
    await action(async () => {
      const result = await api.resetUserPassword(user.id);
      passwordResetCredentials = {
        username: user.username,
        password: result.one_time_password
      };
      notice = `已为“${user.display_name || user.username}”生成一次性密码`;
    });
  }

  async function grantScopeViewerResource() {
    if (!editingUser || !editUserResourceRoleId || !editUserResourceId) return;
    const userID = editingUser.id;
    await action(async () => {
      const binding = await api.createResourceBinding({
        subject_type: 'user',
        subject_id: userID,
        role_id: editUserResourceRoleId,
        resource_id: editUserResourceId
      });
      resourceBindings = [...resourceBindings, binding];
      editUserResourceRoleId = '';
      editUserResourceId = '';
      notice = '观察员资源权限已添加';
    });
  }

  async function revokeScopeViewerResource(binding: ResourceRoleBinding) {
    await action(async () => {
      await api.deleteResourceBinding(binding.id);
      resourceBindings = resourceBindings.filter(
        (item) => item.id !== binding.id
      );
      notice = '观察员资源权限已移除';
    });
  }

  function requestDisable(kind: DisableTarget['kind'], ids: string[]) {
    if (ids.length > 0) disableTarget = { kind, ids: [...ids] };
  }

  async function confirmDisable() {
    if (!disableTarget) return;
    const target = disableTarget;
    await action(async () => {
      if (target.kind === 'team') {
        const updated = await Promise.all(
          target.ids.map((id) => api.updateTeam(id, { status: 'disabled' }))
        );
        const byID = new Map(updated.map((team) => [team.id, team]));
        teams = teams.map((team) => byID.get(team.id) ?? team);
        selectedAccessTeamIds = [];
      } else {
        const updated = await Promise.all(
          target.ids.map((id) => api.updateUser(id, { status: 'disabled' }))
        );
        const byID = new Map(updated.map((user) => [user.id, user]));
        users = users.map((user) => byID.get(user.id) ?? user);
        selectedAccessUserIds = [];
      }
      disableTarget = null;
      notice = `${target.ids.length} 个${target.kind === 'team' ? '团队' : '用户'}已禁用`;
    });
  }

  function viewTitle(currentView: View) {
    if (currentView === 'access') {
      return accessTab === 'teams'
        ? '团队管理'
        : accessTab === 'users'
          ? '用户管理'
          : '角色管理';
    }
    const titles: Record<View, string> = {
      overview: '平台总览',
      organization: '项目管理',
      discovery: '集群项目与应用导入',
      resources: '资源目录',
      skill: 'Skill',
      agent: 'Agent 专家',
      operations: '受控操作与 MCP',
      diagnosis: 'AI 诊断工作台',
      inspection: '自动巡检与健康',
      access: '成员',
      profile: '个人中心'
    };
    return titles[currentView];
  }

  function viewBreadcrumb(currentView: View) {
    if (currentView === 'access') return `成员 / ${viewTitle(currentView)}`;
    const titles: Record<View, string> = {
      overview: '总览',
      organization: '项目',
      discovery: '集群导入',
      resources: '资源',
      skill: 'Skill',
      agent: 'Agent 专家',
      operations: '受控操作',
      diagnosis: 'AI 诊断',
      inspection: '自动巡检',
      access: '成员',
      profile: '个人中心'
    };
    return titles[currentView];
  }

  function scopeType(id: string) {
    return scopeChoices.find((scope) => scope.id === id)?.type ?? 'scope';
  }

  function scopeLevelLabel(type: string) {
    return (
      (
        {
          platform: '平台',
          team: '团队',
          project: '项目'
        } as Record<string, string>
      )[type] ?? type
    );
  }

  function resourceCanManage(resource: Resource, permission: string) {
    return (
      isPlatformAdmin ||
      (resource.scope_id === selectedScopeId &&
        actorPermissionsAtScope(resource.scope_id).includes(permission))
    );
  }

  const resourceCategoryOptions: Record<string, string[]> = {
    全部: [],
    应用: ['虚拟机', '容器化', '云原生'],
    中间件: ['Redis', 'TongRDS', 'Kafka', 'RabbitMQ', 'ElasticSearch'],
    数据库: ['OceanBase', 'Oracle', 'MySQL', 'PostgreSQL'],
    制品库: ['Generic', 'Docker', 'Helm'],
    代码库: ['Git', 'Bundle'],
    Docker: ['API', 'Agent'],
    Kubernetes: ['API', 'Agent'],
    MCPServer: ['StreamHTTP', 'SSE'],
    Skill: ['诊断', '监控', '优化', '维护'],
    LLM: ['Provider'],
    监控: ['指标', '日志', '链路', '告警']
  };

  function resourceCategoryFor(resource: {
    kind: string;
    config?: Record<string, unknown>;
    subtype?: string;
  }) {
    if (resource.kind === 'AIProvider') return 'LLM';
    if (resource.kind === 'MCPServer') return 'MCPServer';
    if (resource.kind === 'GenericAPI') return 'Docker';
    if (resource.kind === 'Application') return '应用';
    if (
      [
        'Redis',
        'Kafka',
        'Elasticsearch',
        'GenericMiddleware',
        'RabbitMQ',
        'TongRDS'
      ].includes(resource.kind)
    )
      return '中间件';
    if (
      ['OceanBase', 'Oracle', 'MySQL', 'PostgreSQL', 'Database'].includes(
        resource.kind
      )
    )
      return '数据库';
    if (['ArtifactRepository', 'ArtifactStore'].includes(resource.kind))
      return '制品库';
    if (['CodeRepository', 'Repository'].includes(resource.kind))
      return '代码库';
    if (resource.kind === 'Docker') return 'Docker';
    if (resource.kind === 'Kubernetes') return 'Kubernetes';
    if (resource.kind === 'Skill') return 'Skill';
    if (
      [
        'Prometheus',
        'Loki',
        'Tempo',
        'Jaeger',
        'Elastic',
        'Datadog',
        'Alertmanager'
      ].includes(resource.kind)
    )
      return '监控';
    return resourceSchemaName(resource.kind);
  }

  function resourceSubtypeFor(resource: {
    kind: string;
    config?: Record<string, unknown>;
    subtype?: string;
  }) {
    const category = resourceCategoryFor(resource);
    const labelMap: Record<string, string> = {
      Application: '虚拟机',
      Kubernetes: 'API',
      KubernetesCluster: 'API',
      Middleware: 'Redis',
      GenericMiddleware: 'Redis',
      Database: 'PostgreSQL',
      ArtifactRepository: 'Generic',
      ArtifactStore: 'Generic',
      CodeRepository: 'Git',
      Repository: 'Git',
      MCPServer: 'StreamHTTP',
      GenericAPI: 'API',
      AIProvider: 'OpenAI',
      Prometheus: '指标',
      Loki: '日志',
      Tempo: '链路',
      Alertmanager: '告警'
    };
    const explicit = String(resource.subtype || resource.config?.subtype || '');
    if (resource.kind === 'AIProvider') return 'Provider';
    return String(
      explicit ||
        resource.config?.provider ||
        labelMap[resource.kind] ||
        resource.kind
    );
  }

  function resourceSubtypeOptionsFor(resource: {
    kind: string;
    config?: Record<string, unknown>;
    subtype?: string;
  }) {
    const current = resourceSubtypeFor(resource);
    const options = resourceCategoryOptions[resourceCategoryFor(resource)] ?? [];
    return options.includes(current) ? options : [current, ...options];
  }

  function resourceCategoryIcon(category: string) {
    const icons: Record<string, string> = {
      全部: '◇',
      应用: '⌘',
      中间件: '◒',
      数据库: '◉',
      制品库: '▤',
      代码库: '⌘',
      Docker: '◈',
      Kubernetes: '⬡',
      MCPServer: '⌁',
      Skill: '✧',
      LLM: '✦',
      监控: '◌'
    };
    return icons[category] ?? '◇';
  }

  function selectResourceCategory(category: string, subtype = '全部') {
    resourceCategory = category;
    resourceSubtype = subtype;
    resourceKind = '';
    expandedResourceCategory = category === '全部' ? '' : category;
  }

  function parseLabels(value: string): Record<string, string> {
    return Object.fromEntries(
      value
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean)
        .map((item) => {
          const [key, ...rest] = item.split('=');
          return [key.trim(), rest.join('=').trim()];
        })
    );
  }

  function formatDate(value: string) {
    return new Date(value).toLocaleString('zh-CN', {
      dateStyle: 'medium',
      timeStyle: 'short'
    });
  }

  function capabilityName(capability: ConnectorCapability) {
    const names: Record<ConnectorCapability, string> = {
      kubernetes_read: '读取 Kubernetes',
      query_metrics: '查询指标',
      query_logs: '查询日志',
      query_traces: '查询链路',
      get_alerts: '读取告警'
    };
    return names[capability] ?? capability;
  }

  function formatIconName(name: string) {
    return name
      .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
      .replace(/([A-Z])([A-Z][a-z])/g, '$1 $2');
  }

  function teamIconComponent(icon: string | undefined): any {
    const key = icon ?? 'UsersRound';
    return (
      lucideIcons[key as keyof typeof lucideIcons] ?? lucideIcons.UsersRound
    );
  }

  function iconGlyph(icon: string | undefined) {
    const glyphs: Record<string, string> = {
      platform: '▣',
      team: '♟',
      building: '▦',
      cloud: '☁',
      project: '▰',
      kubernetes: '☸',
      application: '⌘',
      endpoint: '↗',
      schedule: '◷',
      postgresql: '◉',
      redis: '◒',
      kafka: '◫',
      search: '⌕',
      middleware: '◇',
      llm: '✦',
      mcp: '⌁',
      skill: '✧',
      metrics: '▥',
      logs: '≋',
      traces: '⌁',
      observability: '◌',
      api: '⇄',
      notification: '♢',
      runbook: '☷',
      storage: '▤',
      credential: '⚿',
      resource: '◇'
    };
    return glyphs[icon ?? 'resource'] ?? icon?.slice(0, 1).toUpperCase() ?? '◇';
  }

  function schemaName(schema: ResourceSchema) {
    return schema.display_name || schema.kind;
  }

  function resourceSchemaName(kind: string) {
    if (kind === 'AIProvider') return 'Provider';
    const schema = schemas.find((item) => item.kind === kind);
    return schema ? schemaName(schema) : kind;
  }

  function resourceIcon(kind: string) {
    return iconGlyph(schemas.find((item) => item.kind === kind)?.icon);
  }

  function brandNameFor(resource: {
    kind: string;
    config?: Record<string, unknown>;
    subtype?: string;
  }) {
    const subtype = String(
      resource.subtype ||
        resource.config?.subtype ||
        resource.config?.provider ||
        resource.config?.provider_type ||
        ''
    );
    const normalizedSubtype = subtype.toLowerCase().replace(/[^a-z0-9]/g, '');
    const names: Record<string, string> = {
      Redis: 'Redis',
      Kafka: 'Kafka',
      RabbitMQ: 'RabbitMQ',
      ElasticSearch: 'ElasticSearch',
      Elasticsearch: 'ElasticSearch',
      PostgreSQL: 'PostgreSQL',
      MySQL: 'MySQL',
      Docker: 'Docker',
      Kubernetes: 'Kubernetes',
      Git: 'Git',
      GitHub: 'GitHub',
      GitLab: 'GitLab',
      Helm: 'Helm',
      Prometheus: 'Prometheus',
      Grafana: 'Grafana',
      OpenAI: 'OpenAI',
      Anthropic: 'Anthropic',
      DeepSeek: 'DeepSeek',
      Qwen: 'Qwen',
      Ollama: 'Ollama'
    };
    if (names[subtype]) return names[subtype];
    const normalizedNames: Record<string, string> = {
      openai: 'OpenAI',
      openaicompatible: 'OpenAI',
      anthropic: 'Anthropic',
      deepseek: 'DeepSeek',
      qwen: 'Qwen',
      ollama: 'Ollama',
      postgresql: 'PostgreSQL',
      mysql: 'MySQL',
      elasticsearch: 'ElasticSearch',
      rabbitmq: 'RabbitMQ',
      apachekafka: 'Kafka'
    };
    if (normalizedNames[normalizedSubtype])
      return normalizedNames[normalizedSubtype];
    if (resource.kind === 'Redis') return 'Redis';
    if (resource.kind === 'Kafka') return 'Kafka';
    if (resource.kind === 'RabbitMQ') return 'RabbitMQ';
    if (resource.kind === 'PostgreSQL') return 'PostgreSQL';
    if (resource.kind === 'MySQL') return 'MySQL';
    if (resource.kind === 'Docker') return 'Docker';
    if (resource.kind === 'Kubernetes') return 'Kubernetes';
    if (resource.kind === 'Prometheus') return 'Prometheus';
    return '';
  }

  function focusOnMount(node: HTMLInputElement) {
    node.focus();
  }
</script>

<svelte:head>
  <meta name="description" content="OpsKeeper platform control plane" />
</svelte:head>

<svelte:window on:keydown={handleGlobalKeydown} />

{#if authState === 'loading'}
  <div class="loading-screen">
    <div class="loading-state">
      <span class="spinner"></span>
      <p>正在恢复工作区会话…</p>
    </div>
  </div>
{:else if authState === 'login'}
  <main class="login-shell">
    <div class="login-brand" aria-label="OpsKeeper 智能值守平台">
      <span class="login-logo" aria-hidden="true">O</span>
      <span class="login-brand-copy">
        <strong>OpsKeeper</strong>
        <small>智能值守平台</small>
      </span>
    </div>
    <section class="login-panel" aria-labelledby="login-heading">
      <header class="login-panel-header">
        <p class="login-kicker">账号登录</p>
        <h1 id="login-heading">欢迎回来</h1>
        <p class="login-intro">使用平台账号继续访问 OpsKeeper。</p>
        {#if loginError}<MessageBanner message={loginError} tone="error" />{/if}
      </header>
      <form class="stack-form login-form" on:submit|preventDefault={login}>
        <div class="login-field">
          <label for="login-identifier">账号</label>
          <input
            id="login-identifier"
            type="text"
            bind:value={loginIdentifier}
            autocomplete="username"
            required
            use:focusOnMount
            placeholder="用户名、邮箱或手机号"
          />
        </div>
        <div class="login-field">
          <label for="login-password">密码</label>
          <span class="password-control">
            <input
              id="login-password"
              type={passwordVisible ? 'text' : 'password'}
              bind:value={password}
              autocomplete="current-password"
              required
              placeholder="请输入登录密码"
            />
            <button
              class="password-toggle"
              type="button"
              aria-label={passwordVisible ? '隐藏密码' : '显示密码'}
              aria-pressed={passwordVisible}
              data-tooltip={passwordVisible ? '隐藏密码' : '显示密码'}
              on:click={() => (passwordVisible = !passwordVisible)}
              >{#if passwordVisible}<EyeOff
                  size={18}
                  strokeWidth={1.8}
                  aria-hidden="true"
                />{:else}<Eye
                  size={18}
                  strokeWidth={1.8}
                  aria-hidden="true"
                />{/if}</button
            >
          </span>
        </div>
        <span
          class="login-submit-wrap"
          data-tooltip={!loginIdentifier.trim() || !password
            ? '请先填写账号和密码'
            : undefined}
        >
          <button
            class="login-submit"
            type="submit"
            disabled={busy || !loginIdentifier.trim() || !password}
            aria-busy={busy}
          >
            {#if busy}<span class="button-spinner" aria-hidden="true"
              ></span>{/if}
            <span>{busy ? '正在登录' : '登录'}</span>
          </button>
        </span>
      </form>
      <p class="login-footnote">账号权限由平台管理员统一配置</p>
    </section>
  </main>
{:else if currentUser?.must_change_password}
  <main class="login-shell">
    <section class="login-panel" aria-labelledby="required-password-heading">
      <header class="login-panel-header">
        <p class="login-kicker">安全验证</p>
        <h1 id="required-password-heading">请修改一次性密码</h1>
        <p class="login-intro">为保护账号安全，完成修改前无法访问平台内容。</p>
        {#if errorMessage}<MessageBanner message={errorMessage} tone="error" />{/if}
      </header>
      <form
        class="stack-form login-form"
        on:submit|preventDefault={() => changeOwnPassword(true)}
      >
        <label class="login-field"
          >新密码<span class="password-control"
            ><input
              type={requiredNewPasswordVisible ? 'text' : 'password'}
              bind:value={requiredNewPassword}
              required
              minlength="8"
              autocomplete="new-password"
              placeholder="至少 8 位"
            /><button
              class="password-toggle"
              type="button"
              aria-label={requiredNewPasswordVisible
                ? '隐藏新密码'
                : '显示新密码'}
              aria-pressed={requiredNewPasswordVisible}
              data-tooltip={requiredNewPasswordVisible
                ? '隐藏新密码'
                : '显示新密码'}
              on:click={() =>
                (requiredNewPasswordVisible = !requiredNewPasswordVisible)}
              >{#if requiredNewPasswordVisible}<EyeOff
                  size={18}
                  strokeWidth={1.8}
                  aria-hidden="true"
                />{:else}<Eye
                  size={18}
                  strokeWidth={1.8}
                  aria-hidden="true"
                />{/if}</button
            ></span
          ></label
        >
        <label class="login-field"
          >确认新密码<span class="password-control"
            ><input
              type={requiredConfirmPasswordVisible ? 'text' : 'password'}
              bind:value={requiredConfirmPassword}
              required
              minlength="8"
              autocomplete="new-password"
              placeholder="再次输入新密码"
            /><button
              class="password-toggle"
              type="button"
              aria-label={requiredConfirmPasswordVisible
                ? '隐藏确认密码'
                : '显示确认密码'}
              aria-pressed={requiredConfirmPasswordVisible}
              data-tooltip={requiredConfirmPasswordVisible
                ? '隐藏确认密码'
                : '显示确认密码'}
              on:click={() =>
                (requiredConfirmPasswordVisible =
                  !requiredConfirmPasswordVisible)}
              >{#if requiredConfirmPasswordVisible}<EyeOff
                  size={18}
                  strokeWidth={1.8}
                  aria-hidden="true"
                />{:else}<Eye
                  size={18}
                  strokeWidth={1.8}
                  aria-hidden="true"
                />{/if}</button
            ></span
          ></label
        >
        <button class="primary login-submit" disabled={busy}
          >{busy ? '正在更新' : '更新密码并继续'}</button
        >
      </form>
    </section>
  </main>
{:else}
  <div
    class="app-shell"
    class:sidebar-compact={sidebarCompact}
    class:sidebar-hover-mode={preferences.sidebar_mode === 'hover'}
  >
    <aside
      class="sidebar"
      class:sidebar-compact={sidebarCompact}
      on:mouseenter={() => (sidebarHovered = true)}
      on:mouseleave={() => (sidebarHovered = false)}
    >
      <div class="brand">
        <span class="brand-mark" aria-hidden="true">O</span><span
          class="brand-copy">OpsKeeper<small>智能值守平台</small></span
        >
      </div>
      <div class="workspace-label">WORKSPACE</div>
      <nav aria-label="主导航">
        <button
          aria-label="总览"
          class:active={view === 'overview'}
          class="nav-item"
          on:click={() => chooseView('overview')}
          data-tooltip={sidebarCompact ? '总览' : undefined}
          ><LayoutDashboard
            size={18}
            strokeWidth={1.8}
            aria-hidden="true"
          /><span class="nav-item-label">总览</span></button
        >
        <button
          aria-label="项目"
          class:active={view === 'organization'}
          class="nav-item"
          on:click={() => chooseView('organization')}
          data-tooltip={sidebarCompact ? '项目 / Project' : undefined}
          ><FolderKanban size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">项目</span
          ></button
        >
        <button
          aria-label="资源"
          class:active={view === 'resources'}
          class="nav-item"
          on:click={() => chooseView('resources')}
          data-tooltip={sidebarCompact ? '资源' : undefined}
          ><Boxes size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">资源</span
          ></button
        >
        <button
          aria-label="集群导入"
          class:active={view === 'discovery'}
          class="nav-item"
          on:click={() => chooseView('discovery')}
          data-tooltip={sidebarCompact ? '集群导入' : undefined}
          ><CloudDownload size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">集群导入</span
          ></button
        >
        <button
          aria-label="Skill"
          class:active={view === 'skill'}
          class="nav-item"
          on:click={() => chooseView('skill')}
          data-tooltip={sidebarCompact ? 'Skill' : undefined}
          ><ClipboardCheck
            size={18}
            strokeWidth={1.8}
            aria-hidden="true"
          /><span class="nav-item-label">Skill</span></button
        >
        <button
          aria-label="Agent 专家"
          class:active={view === 'agent'}
          class="nav-item"
          on:click={() => chooseView('agent')}
          data-tooltip={sidebarCompact ? 'Agent 专家' : undefined}
          ><Bot size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">Agent 专家</span
          ></button
        >
        <button
          aria-label="AI 诊断"
          class:active={view === 'diagnosis'}
          class="nav-item"
          on:click={() => chooseView('diagnosis')}
          data-tooltip={sidebarCompact ? 'AI 诊断' : undefined}
          ><ScanSearch size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">AI 诊断</span
          ></button
        >
        <button
          aria-label="自动巡检"
          class:active={view === 'inspection'}
          class="nav-item"
          on:click={() => chooseView('inspection')}
          data-tooltip={sidebarCompact ? '自动巡检' : undefined}
          ><Stethoscope size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">自动巡检</span
          ></button
        >
        <button
          aria-label="受控操作"
          class:active={view === 'operations'}
          class="nav-item"
          on:click={() => chooseView('operations')}
          data-tooltip={sidebarCompact ? '受控操作' : undefined}
          ><ClipboardCheck
            size={18}
            strokeWidth={1.8}
            aria-hidden="true"
          /><span class="nav-item-label">受控操作</span></button
        >
        <div class="nav-group" class:open={accessMenuOpen}>
          <button
            aria-label="展开成员菜单"
            aria-expanded={accessMenuOpen}
            class:active={view === 'access'}
            class="nav-item nav-group-trigger"
            on:click={() => (accessMenuOpen = !accessMenuOpen)}
            data-tooltip={sidebarCompact ? '成员 / Member' : undefined}
            ><UsersRound size={18} strokeWidth={1.8} aria-hidden="true" /><span
              class="nav-item-label">成员</span
            ><ChevronDown
              class="nav-group-chevron"
              size={14}
              strokeWidth={1.8}
              aria-hidden="true"
            /></button
          >
          {#if accessMenuOpen}
            <div class="nav-submenu" aria-label="成员子菜单">
              <button
                type="button"
                class:active={view === 'access' && accessTab === 'teams'}
                on:click={() => chooseAccessTab('teams')}
                ><Building2
                  size={15}
                  strokeWidth={1.8}
                  aria-hidden="true"
                /><span>团队管理</span></button
              ><button
                type="button"
                class:active={view === 'access' && accessTab === 'users'}
                on:click={() => chooseAccessTab('users')}
                ><UserRound
                  size={15}
                  strokeWidth={1.8}
                  aria-hidden="true"
                /><span>用户管理</span></button
              ><button
                type="button"
                class:active={view === 'access' && accessTab === 'roles'}
                on:click={() => chooseAccessTab('roles')}
                ><ShieldCheck
                  size={15}
                  strokeWidth={1.8}
                  aria-hidden="true"
                /><span>角色管理</span></button
              >
            </div>
          {/if}
        </div>
      </nav>
      <div class="sidebar-footer">
        <div class="user-menu-wrap sidebar-user-menu">
          <button
            class="user-menu-trigger"
            aria-label="打开用户菜单"
            aria-expanded={userMenuOpen}
            on:click={() => (userMenuOpen = !userMenuOpen)}
          >
            {#if avatarURL}<img
                src={avatarURL}
                alt=""
                class="avatar avatar-image"
              />{:else}<span class="avatar"
                >{(currentUser?.display_name || currentUser?.username || 'U')
                  .slice(0, 1)
                  .toUpperCase()}</span
              >{/if}<span class="user-menu-name"
              >{currentUser?.display_name || currentUser?.username}</span
            >
          </button>
          {#if userMenuOpen}<div class="user-menu" role="menu">
              <button role="menuitem" on:click={openProfile}
                ><UserRound
                  size={16}
                  strokeWidth={1.8}
                  aria-hidden="true"
                />个人中心</button
              ><button role="menuitem" on:click={logout} disabled={busy}
                ><LogOut
                  size={16}
                  strokeWidth={1.8}
                  aria-hidden="true"
                />退出登录</button
              >
            </div>{/if}
        </div>
        <button
          class="sidebar-toggle"
          aria-label={sidebarCompact ? '展开导航栏' : '折叠导航栏'}
          data-tooltip={sidebarCompact ? '展开导航栏' : '折叠导航栏'}
          on:click={toggleSidebar}
        >
          {#if sidebarCompact}<PanelLeftOpen
              size={18}
              strokeWidth={1.8}
              aria-hidden="true"
            />{:else}<PanelLeftClose
              size={18}
              strokeWidth={1.8}
              aria-hidden="true"
            />{/if}
        </button>
      </div>
    </aside>

    <main
      class="main-content"
      class:diagnosis-main-content={view === 'diagnosis'}
    >
      <header class="topbar">
        <div>
          <p class="breadcrumb">
            {view === 'access'
              ? viewBreadcrumb(view)
              : `${activeScope?.name ?? '平台'} / ${viewBreadcrumb(view)}`}
          </p>
          <h1>{viewTitle(view)}</h1>
        </div>
        <div class="topbar-actions">
          <div class="workspace-switcher topbar-workspace-switcher">
              <label class="workspace-team workspace-team-select"
                ><select
                  aria-label="切换团队"
                  value={selectedTeamId}
                  on:change={(event) =>
                    chooseTeam(
                      (event.currentTarget as HTMLSelectElement).value
                    )}
                >
                  {#if hasPlatformRole}<option value="">全部团队</option>{/if}
                  {#each teams as team}<option value={team.id}
                      >{team.name}</option
                    >{/each}
                </select></label
              >
              <label class="workspace-project"
                ><select
                  aria-label="切换项目"
                  value={selectedProjectId}
                  disabled={!workspaceProjects.length}
                  on:change={(event) =>
                    chooseProject(
                      (event.currentTarget as HTMLSelectElement).value
                    )}
                >
                  <option value="">全部项目</option>
                  {#each workspaceProjects as project}<option
                      value={project.id}>{project.name}</option
                    >{/each}
                </select></label
              >
            </div>
        </div>
        {#if activeMessage && !messageInChildSurface}
          <div class="topbar-message-slot">
            <MessageBanner message={activeMessage} tone={activeMessageTone} />
          </div>
        {/if}
      </header>

      {#if view === 'overview'}
        <section class="content-grid">
          <div class="metric-grid">
            <article class="metric">
              <span class="metric-label">团队</span><strong
                >{teams.length}</strong
              ><span class="metric-note">可访问的组织单元</span>
            </article>
            <article class="metric">
              <span class="metric-label">项目</span><strong
                >{visibleProjects.length}</strong
              ><span class="metric-note">当前作用域内</span>
            </article>
            <article class="metric">
              <span class="metric-label">资源</span><strong
                >{visibleResources.length}</strong
              ><span class="metric-note">已登记资源</span>
            </article>
          </div>
          <section class="panel wide-panel" aria-labelledby="health-heading">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">SYSTEM</p>
                <h2 id="health-heading">控制平面状态</h2>
              </div>
              <span
                class:healthy={health?.status === 'ready'}
                class="status-pill"
                ><span class="status-dot"></span>{health?.status === 'ready'
                  ? 'Ready'
                  : 'Checking'}</span
              >
            </div>
            <div class="status-table">
              <div class="table-header">
                <span>服务</span><span>状态</span><span>延迟</span>
              </div>
              {#each rows as row}<div class="table-row">
                  <span class="service-name">{row.name}</span><span
                    class:up={row.status === 'up'}
                    class:down={row.status === 'down'}
                    class="service-status"
                    ><span class="status-dot"></span>{row.status === 'up'
                      ? 'Operational'
                      : row.status === 'down'
                        ? 'Unavailable'
                        : 'Checking'}</span
                  ><span class="latency">{row.latency ?? '—'}</span>
                </div>{/each}
            </div>
          </section>
          <section class="panel recent-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">CATALOG</p>
                <h2>最近资源</h2>
              </div>
              <button
                class="text-button"
                on:click={() => chooseView('resources')}>查看全部 →</button
              >
            </div>
            {#if visibleResources.length === 0}<div class="empty-state">
                当前作用域还没有资源。
              </div>{:else}<div class="compact-list">
                {#each visibleResources.slice(0, 5) as resource}<button
                    class="compact-row"
                    on:click={() => {
                      selectedResourceId = resource.id;
                      chooseView('resources');
                      void loadResourceDetails(resource.id);
                    }}
                    ><span class="entity-summary"
                      ><span class="entity-icon resource-icon"
                        >{iconGlyph(
                          schemas.find(
                            (schema) => schema.kind === resource.kind
                          )?.icon
                        )}</span
                      ><span
                        ><strong>{resource.name}</strong><small
                          >{resourceSchemaName(resource.kind)} · {scopeName(
                            resource.scope_id
                          )}</small
                        ></span
                      ></span
                    ><span class="status-label {resource.status}"
                      >{resource.status}</span
                    ></button
                  >{/each}
              </div>{/if}
          </section>
        </section>
      {:else if view === 'profile'}
        <section class="profile-layout">
          <section class="panel profile-panel">
            <form class="profile-form" on:submit|preventDefault={saveProfile}>
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">PROFILE</p>
                  <h2>个人资料</h2>
                </div>
              </div>
              <div class="profile-avatar-row">
                {#if avatarURL}<img
                    src={avatarURL}
                    alt="当前头像"
                    class="profile-avatar avatar-image"
                  />{:else}<span class="profile-avatar"
                    >{(
                      currentUser?.display_name ||
                      currentUser?.username ||
                      'U'
                    )
                      .slice(0, 1)
                      .toUpperCase()}</span
                  >{/if}
                <div>
                  <strong>{currentUser?.username}</strong>
                  <p>PNG 或 JPEG，最大 1 MiB。</p>
                  <label
                    class="secondary-button upload-button"
                    for="profile-avatar-upload"
                    ><Upload
                      size={15}
                      strokeWidth={1.8}
                      aria-hidden="true"
                    />{avatarBusy ? '正在上传' : '更换头像'}</label
                  >
                  <input
                    id="profile-avatar-upload"
                    class="visually-hidden"
                    type="file"
                    accept="image/png,image/jpeg"
                    disabled={avatarBusy}
                    on:change={uploadAvatar}
                  />
                </div>
              </div>
              <div class="profile-fields">
                <label
                  >用户名<input
                    value={currentUser?.username ?? ''}
                    disabled
                    aria-label="用户名"
                  /></label
                >
                <label
                  >显示名<input
                    bind:value={profileDisplayName}
                    required
                    maxlength="120"
                    placeholder="请输入显示名"
                    aria-label="显示名"
                  /></label
                >
                <label
                  >邮箱<input
                    type="email"
                    bind:value={profileEmail}
                    placeholder="例如：name@example.com"
                    aria-label="邮箱"
                  /></label
                >
                <label
                  >电话<input
                    type="tel"
                    bind:value={profilePhone}
                    placeholder="例如：13800138000"
                    aria-label="电话"
                  /></label
                >
              </div>
              <div class="profile-team">
                <UsersRound
                  size={17}
                  strokeWidth={1.8}
                  aria-hidden="true"
                /><span
                  ><strong>所属团队</strong><small>当前未配置团队成员关系</small
                  ></span
                >
              </div>
            </form>
            <form
              class="profile-password-form"
              on:submit|preventDefault={() => changeOwnPassword()}
            >
              <div class="profile-password-row">
                <label>
                  <span class="visually-hidden">当前密码</span>
                  <input
                    type="password"
                    bind:value={profileCurrentPassword}
                    required
                    autocomplete="current-password"
                    placeholder="当前密码"
                    aria-label="当前密码"
                  />
                </label>
                <label>
                  <span class="visually-hidden">新密码</span>
                  <input
                    type="password"
                    bind:value={profileNewPassword}
                    required
                    minlength="8"
                    autocomplete="new-password"
                    placeholder="新密码"
                    aria-label="新密码"
                  />
                </label>
                <label>
                  <span class="visually-hidden">确认新密码</span>
                  <input
                    type="password"
                    bind:value={profileConfirmPassword}
                    required
                    minlength="8"
                    autocomplete="new-password"
                    placeholder="确认新密码"
                    aria-label="确认新密码"
                  />
                </label>
                <button class="primary" disabled={busy} aria-busy={busy}>
                  {busy ? '正在更新' : '更新密码'}
                </button>
              </div>
            </form>
          </section>

          <form class="profile-form" on:submit|preventDefault={saveProfile}>
            <section class="panel profile-panel">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">PREFERENCES</p>
                  <h2>界面偏好</h2>
                </div>
              </div>
              <fieldset class="preference-group">
                <legend>系统主题</legend>
                <div
                  class="segmented-control"
                  role="radiogroup"
                  aria-label="系统主题"
                >
                  <button
                    type="button"
                    class:active={preferences.theme === 'auto'}
                    role="radio"
                    aria-checked={preferences.theme === 'auto'}
                    on:click={() => {
                      preferences = { ...preferences, theme: 'auto' };
                      applyTheme();
                    }}
                    ><Monitor
                      size={16}
                      strokeWidth={1.8}
                      aria-hidden="true"
                    />自动</button
                  >
                  <button
                    type="button"
                    class:active={preferences.theme === 'light'}
                    role="radio"
                    aria-checked={preferences.theme === 'light'}
                    on:click={() => {
                      preferences = { ...preferences, theme: 'light' };
                      applyTheme();
                    }}
                    ><Sun
                      size={16}
                      strokeWidth={1.8}
                      aria-hidden="true"
                    />浅色</button
                  >
                  <button
                    type="button"
                    class:active={preferences.theme === 'dark'}
                    role="radio"
                    aria-checked={preferences.theme === 'dark'}
                    on:click={() => {
                      preferences = { ...preferences, theme: 'dark' };
                      applyTheme();
                    }}
                    ><Moon
                      size={16}
                      strokeWidth={1.8}
                      aria-hidden="true"
                    />深色</button
                  >
                </div>
              </fieldset>
              <fieldset class="preference-group">
                <legend>侧边导航栏</legend>
                <div
                  class="segmented-control"
                  role="radiogroup"
                  aria-label="侧边导航栏模式"
                >
                  <button
                    type="button"
                    class:active={preferences.sidebar_mode === 'fixed'}
                    role="radio"
                    aria-checked={preferences.sidebar_mode === 'fixed'}
                    on:click={() =>
                      (preferences = { ...preferences, sidebar_mode: 'fixed' })}
                    >固定模式</button
                  >
                  <button
                    type="button"
                    class:active={preferences.sidebar_mode === 'hover'}
                    role="radio"
                    aria-checked={preferences.sidebar_mode === 'hover'}
                    on:click={() =>
                      (preferences = {
                        ...preferences,
                        sidebar_mode: 'hover',
                        sidebar_collapsed: true
                      })}>窄栏悬浮展开</button
                  >
                </div>
                <p class="preference-help">
                  固定模式可通过侧栏右下角图标展开或收起；悬浮模式默认显示图标，鼠标移入后展开。
                </p>
              </fieldset>
              <div class="profile-actions">
                <button class="primary" disabled={busy} aria-busy={busy}
                  >{busy ? '正在保存' : '保存配置'}</button
                >
              </div>
            </section>
          </form>
        </section>
      {:else if view === 'organization'}
        <section class="content-grid two-column">
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">STRUCTURE</p>
                <h2>团队</h2>
              </div>
              <span class="count">{teams.length}</span>
            </div>
            <div class="table-list">
              {#each teams as team}
                {@const TeamIcon = teamIconComponent(team.icon)}
                <button
                  class:selected={selectedScopeId === team.scope.id}
                  class="list-row"
                  on:click={() => {
                    selectedScopeId = team.scope.id;
                    projectTeamId = team.id;
                  }}
                  ><span class="entity-summary"
                    ><span class="entity-icon team-icon"
                      ><TeamIcon size={17} strokeWidth={1.8} /></span
                    ><span
                      ><strong>{team.name}</strong><small
                        >{team.code} · {team.status}</small
                      ></span
                    ></span
                  ><span class="row-arrow">→</span></button
                >{:else}<div class="empty-state">暂无团队</div>{/each}
            </div>
            <div class="inline-form">
              <button
                class="primary"
                type="button"
                disabled={busy}
                on:click={openTeamDialog}
                ><Plus size={15} aria-hidden="true" />添加团队</button
              >
            </div>
          </section>
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">STRUCTURE</p>
                <h2>项目</h2>
              </div>
              <span class="count">{visibleProjects.length}</span>
            </div>
            <div class="table-list">
              {#each visibleProjects as project}<button
                  class:selected={selectedScopeId === project.scope.id}
                  class="list-row"
                  on:click={() => (selectedScopeId = project.scope.id)}
                  ><span class="entity-summary"
                    ><span class="entity-icon project-icon"
                      >{iconGlyph(project.icon)}</span
                    ><span
                      ><strong>{project.name}</strong><small
                        >{project.code} · {scopeName(project.team_id)}</small
                      ></span
                    ></span
                  ><span class="status-label {project.status}"
                    >{project.status}</span
                  ></button
                >{:else}<div class="empty-state">当前作用域暂无项目</div>{/each}
            </div>
            <form
              class="stack-form compact-form"
              on:submit|preventDefault={createProject}
            >
              <label
                >所属团队<select bind:value={projectTeamId} required
                  ><option value="" disabled>选择团队</option
                  >{#each teams as team}<option value={team.id}
                      >{team.name}</option
                    >{/each}</select
                ></label
              >
              <div class="form-row">
                <input
                  bind:value={projectIcon}
                  placeholder="图标，如 project 或 ▰"
                  aria-label="项目图标"
                />
                <input
                  bind:value={projectName}
                  required
                  placeholder="项目名称"
                  aria-label="项目名称"
                /><input
                  bind:value={projectCode}
                  required
                  placeholder="编码"
                  aria-label="项目编码"
                />
              </div>
              <button class="primary" disabled={busy || !projectTeamId}
                >新增项目</button
              >
            </form>
          </section>
        </section>
      {:else if view === 'discovery'}
        <section class="discovery-layout">
          <section class="panel discovery-control">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">KUBERNETES SOURCE</p>
                <h2>选择集群并扫描</h2>
              </div>
              <span class="entity-icon resource-icon" aria-hidden="true">☸</span
              >
            </div>
            {#if kubernetesClusters.length === 0}
              <div class="empty-state">
                请先在资源目录登记 Kubernetes 集群及其 kubeconfig 凭据。
              </div>
            {:else}
              <div class="discovery-toolbar">
                <label
                  >Kubernetes 集群<select
                    bind:value={selectedClusterId}
                    on:change={selectDiscoveryCluster}
                  >
                    {#each kubernetesClusters as cluster}
                      <option value={cluster.id}
                        >{cluster.name} · {scopeName(cluster.scope_id)}</option
                      >
                    {/each}
                  </select></label
                >
                <button
                  class="primary"
                  on:click={startDiscovery}
                  disabled={busy ||
                    !selectedClusterId ||
                    activeDiscovery?.status === 'running' ||
                    activeDiscovery?.status === 'queued'}>开始扫描</button
                >
              </div>
            {/if}
            {#if activeDiscovery}
              <div class="discovery-status">
                <span
                  class="status-pill"
                  class:healthy={activeDiscovery.status === 'succeeded'}
                >
                  <span class="status-dot"></span>{activeDiscovery.status}
                </span>
                <span>{activeDiscovery.item_count} 个候选</span>
                <span>{activeDiscovery.imported_count} 个已处理</span>
                <span>{formatDate(activeDiscovery.created_at)}</span>
              </div>
            {/if}
            {#if discoveryRuns.length > 0}
              <div class="run-history" aria-label="同步历史">
                {#each discoveryRuns.slice(0, 6) as run}
                  <button
                    class:active={activeDiscovery?.id === run.id}
                    on:click={() => void openDiscovery(run)}
                  >
                    <span>{formatDate(run.created_at)}</span>
                    <strong>{run.status}</strong>
                    <small>{run.item_count} 项</small>
                  </button>
                {/each}
              </div>
            {/if}
          </section>

          {#if activeDiscovery?.status === 'succeeded'}
            <section class="panel mapping-panel">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">NAMESPACE MAPPING</p>
                  <h2>命名空间映射项目</h2>
                </div>
                <span class="count">{namespaceCandidates.length}</span>
              </div>
              <div class="mapping-list">
                {#each namespaceCandidates as item}
                  {@const namespace = item.namespace || item.name}
                  {@const draft = projectMappingDrafts[namespace]}
                  {#if draft}
                    <div class="mapping-row">
                      <div class="mapping-source">
                        <span class="entity-icon project-icon">▰</span>
                        <span
                          ><strong>{namespace}</strong><small
                            >Namespace · {item.external_uid.slice(0, 12)}</small
                          ></span
                        >
                      </div>
                      <label
                        >处理方式<select bind:value={draft.mode}>
                          <option value="existing">映射已有项目</option>
                          <option value="create">创建新项目</option>
                          <option value="ignore">忽略</option>
                        </select></label
                      >
                      {#if draft.mode === 'existing'}
                        <label
                          >目标项目<select
                            bind:value={draft.project_id}
                            required
                          >
                            <option value="" disabled>选择项目</option>
                            {#each allowedProjectsForCluster() as project}
                              <option value={project.id}
                                >{project.name} · {teams.find(
                                  (team) => team.id === project.team_id
                                )?.name}</option
                              >
                            {/each}
                          </select></label
                        >
                      {:else if draft.mode === 'create'}
                        <div class="mapping-create-fields">
                          <label
                            >所属团队<select
                              bind:value={draft.team_id}
                              required
                            >
                              <option value="" disabled>选择团队</option>
                              {#each allowedTeamsForCluster() as team}
                                <option value={team.id}>{team.name}</option>
                              {/each}
                            </select></label
                          >
                          <label
                            >项目名称<input
                              bind:value={draft.name}
                              required
                            /></label
                          >
                          <label
                            >项目编码<input
                              bind:value={draft.code}
                              required
                            /></label
                          >
                        </div>
                      {:else}
                        <p class="mapping-note">
                          该命名空间及其工作负载不会进入项目和应用目录。
                        </p>
                      {/if}
                    </div>
                  {/if}
                {/each}
              </div>
            </section>

            <section class="panel application-preview-panel">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">APPLICATION PREVIEW</p>
                  <h2>工作负载映射应用</h2>
                </div>
                <span class="count">{applicationCandidates.length}</span>
              </div>
              <div class="application-preview-list">
                {#each applicationCandidates as item}
                  <label class="application-preview-row">
                    <input
                      type="checkbox"
                      bind:checked={selectedDiscoveryItems[item.id]}
                      disabled={projectMappingDrafts[item.namespace || '']
                        ?.mode === 'ignore'}
                    />
                    <span class="entity-icon resource-icon">⌘</span>
                    <span class="application-identity">
                      <strong>{item.name}</strong>
                      <small
                        >{item.namespace} · {String(
                          (
                            item.payload.kubernetes as
                              Record<string, unknown> | undefined
                          )?.workload_kind || 'Workload'
                        )}</small
                      >
                    </span>
                    <span class="application-facts">
                      <span>{payloadCount(item, 'services')} Service</span>
                      <span>{payloadCount(item, 'ingresses')} Ingress</span>
                      <span>{payloadCount(item, 'endpoints')} Endpoint</span>
                      <span>{payloadCount(item, 'instances')} Instance</span>
                    </span>
                  </label>
                {:else}
                  <div class="empty-state">集群中没有可导入的工作负载。</div>
                {/each}
              </div>
              <div class="import-actions">
                <p class="muted">
                  确认后创建或绑定项目，并将选中的工作负载写入项目应用；Kubernetes
                  子对象不会登记为独立资源。
                </p>
                <button
                  class="primary"
                  on:click={importDiscovery}
                  disabled={busy || namespaceCandidates.length === 0}
                >
                  确认导入项目与应用
                </button>
              </div>
            </section>
          {/if}
        </section>
      {:else if view === 'resources'}
        <section class="resources-layout">
          <section class="panel resource-list-panel">
            <div class="resource-catalog-rail">
              <button
                class:active={resourceCategory === '全部'}
                class="catalog-root"
                type="button"
                on:click={() => selectResourceCategory('全部')}
                ><span class="catalog-icon">{resourceCategoryIcon('全部')}</span
                ><span class="catalog-label">全部资源</span><span
                  >{visibleResources.length}</span
                ></button
              >
              {#each Object.entries(resourceCategoryOptions).filter(([name]) => name !== '全部') as [category, subtypes]}
                <div class="catalog-category">
                  <button
                    class:active={resourceCategory === category &&
                      resourceSubtype === '全部'}
                    class="catalog-category-button"
                    type="button"
                    on:click={() => selectResourceCategory(category)}
                  >
                    <span class="catalog-name"
                      ><span class="catalog-icon"
                        >{resourceCategoryIcon(category)}</span
                      >{category}</span
                    ><span
                      >{visibleResources.filter(
                        (item) => resourceCategoryFor(item) === category
                      ).length}</span
                    >
                  </button>
                  {#if expandedResourceCategory === category}
                    <div class="catalog-subtypes">
                      {#each subtypes as subtype}
                        <button
                          class:active={resourceCategory === category &&
                            resourceSubtype === subtype}
                          type="button"
                          on:click={() =>
                            selectResourceCategory(category, subtype)}
                        >
                          <span class="catalog-name"
                            ><span class="catalog-icon subtype-icon"
                              >{resourceCategoryIcon(subtype)}</span
                            >{subtype}</span
                          ><span
                            >{visibleResources.filter(
                              (item) =>
                                resourceCategoryFor(item) === category &&
                                resourceSubtypeFor(item) === subtype
                            ).length}</span
                          >
                        </button>
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
                  <select
                    bind:value={resourceStatusFilter}
                    aria-label="连接状态"
                    ><option value="all">全部状态</option><option value="active"
                      >正常</option
                    ><option value="disabled">已停用</option><option
                      value="unknown">未知</option
                    ></select
                  >
                  <select bind:value={resourceLevelFilter} aria-label="资源级别"
                    ><option value="all">全部级别</option><option
                      value="platform">平台级</option
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
                    class="resource-catalog-row"
                    on:toggle={() => void loadResourceDetails(resource.id)}
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
                        <span class="resource-cell provider-purpose-cell"
                          ><span class="provider-purpose-tags">
                            {#each providerBindingsFor(resource) as binding}
                              <span class="resource-tag provider-purpose-tag"
                                >{providerPurposeLabel(binding.tag)}</span
                              >
                            {:else}
                              <small class="resource-tags-empty">未设置</small>
                            {/each}
                          </span><small>用途</small></span
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
                            : resource.status}">{resourceCheck
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
                          <div class="provider-resource-meta-wide">
                            <span>服务地址</span><strong>{resourceEndpointFor(resource)}</strong>
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
                            <strong>Model 配置</strong><span>{providerModels.length} 个</span>
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
                              <div><span>最大输出</span><strong>{Number(model.max_output_tokens ?? 8192).toLocaleString()} Token</strong></div>
                              <div><span>温度</span><strong>{Number(model.temperature ?? 0.7)}</strong></div>
                              <div><span>优先级</span><strong>{Number(model.priority ?? 0)}</strong></div>
                            </div>
                          {:else}
                            <div class="empty-state">尚未配置 Model。</div>
                          {/each}
                        </div>
                      </div>
                    {:else}
                      <div class="resource-row-details">
                        <div>
                          <span>资源地址</span><strong
                            >{resourceEndpointFor(resource)}</strong
                          >
                        </div>
                        <div>
                          <span>连接测试</span><strong
                            >{selectedResourceId === resource.id &&
                            connectionCheck
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
                      </div>
                    {/if}
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
                    <p class="eyebrow">{editingProviderResourceId ? 'EDIT PROVIDER' : 'ADD RESOURCE'}</p>
                    <h2 id="resource-add-title">
                      <span>{editingProviderResourceId ? '编辑资源' : '添加资源'}</span>
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
                        if (resourceBasicConfigurationComplete())
                          resourceAddStep = 2;
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
                  {:else}
                    <button
                      class:active={resourceAddStep === 2}
                      disabled={!resourceBasicConfigurationComplete()}
                      type="button"
                      on:click={() => {
                        if (resourceBasicConfigurationComplete()) resourceAddStep = 2;
                      }}
                      ><b>2</b><span>配置资源</span></button
                    >
                  {/if}
                </aside>
                <div
                  class="resource-add-content"
                  class:type-selection-content={resourceAddStep === 1}
                >
                  <div class="resource-add-step-heading">
                    {#if activeMessage}
                      <MessageBanner message={activeMessage} tone={activeMessageTone} />
                    {/if}
                    <h3>{resourceAddStepTitle(resourceAddStep, resourceKind)}</h3>
                    {#if resourceAddStepValidationMessage()}
                      <p class="resource-add-step-validation" role="alert">
                        {resourceAddStepValidationMessage()}
                      </p>
                    {/if}
                    <div class="resource-add-step-actions">
                      {#if resourceAddStep === 1}
                        <button
                          class="primary"
                          type="button"
                          on:click={continueResourceAdd}>下一步</button
                        >
                      {:else if resourceKind === 'AIProvider' && resourceAddStep === 2}
                        <button
                          class="secondary"
                          type="button"
                          on:click={() => (resourceAddStep = 1)}>上一步</button
                        >
                        <button
                          class="primary"
                          type="button"
                          on:click={continueProviderAdd}>下一步</button
                        >
                      {:else if resourceKind === 'AIProvider' && resourceAddStep === 3}
                        <button
                          class="secondary"
                          type="button"
                          on:click={() => (resourceAddStep = 2)}>上一步</button
                        >
                        <button
                          class="primary"
                          type="button"
                          on:click={continueProviderAdd}>下一步</button
                        >
                      {:else if resourceKind === 'AIProvider' && resourceAddStep === 4}
                        <button
                          class="secondary"
                          type="button"
                          on:click={() => (resourceAddStep = 3)}>上一步</button
                        >
                        <button
                          class="primary"
                          type="submit"
                          form="provider-create-form"
                          disabled={busy || !selectedScopeId}
                          >{editingProviderResourceId ? '保存 Provider' : '创建 Provider'}</button
                        >
                      {:else if resourceAddStep === 2}
                        <button
                          class="secondary"
                          type="button"
                          on:click={() => (resourceAddStep = 1)}>上一步</button
                        >
                        <button
                          class="primary"
                          type="submit"
                          form="resource-create-form"
                          disabled={busy || !selectedScopeId}>创建资源</button
                        >
                      {/if}
                    </div>
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
                          disabled={Boolean(editingProviderResourceId)}
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
                          disabled={!resourceAddCategory || Boolean(editingProviderResourceId)}
                          on:change={(event) => {
                            resourceAddSubtype = (event.currentTarget as HTMLSelectElement).value;
                            const schema = resourceSchemaForSelection(resourceAddCategory, resourceAddSubtype);
                            resourceKind = resourceAddCategory === 'LLM' && resourceAddSubtype === 'Provider' ? 'AIProvider' : (schema?.kind ?? '');
                          }}
                          ><option value="">请选择资源子类型</option
                          >{#each resourceAddSubtypeOptions as subtype}<option
                              value={subtype}>{subtype}</option
                            >{/each}</select
                        ></label
                      >
                      </div>
                      <div class="resource-basic-identity-row">
                      <label class="resource-basic-name" class:invalid={resourceBasicConfigurationAttempted && !resourceName.trim()}>
                        <span><i>*</i>资源名称</span><input bind:value={resourceName} required placeholder="例如 production-resource" autocomplete="off" />
                      </label>
                      <label class="resource-basic-level">
                        <span>资源级别</span><input value={activeScopeSummary()} readonly aria-readonly="true" />
                      </label>
                      <label class="resource-basic-enabled">
                        <span>是否启用</span><span class="provider-toggle-control"><input type="checkbox" checked={resourceStatus === 'active'} on:change={(event) => (resourceStatus = (event.currentTarget as HTMLInputElement).checked ? 'active' : 'disabled')} aria-label="是否启用资源" /><i aria-hidden="true"></i></span>
                      </label>
                      </div>
                      <label class="resource-basic-labels">
                        <span>资源标签</span><input bind:value={resourceLabels} placeholder="填写 key=value，多个标签用逗号分隔，例如 env=prod, owner=platform" autocomplete="off" />
                      </label>
                    </div>
                  {:else if resourceKind === 'MCPServer' && resourceAddStep === 2}
                    <p class="resource-add-description">
                      通过安全的 Streamable HTTP 接入 MCP 服务。服务地址、工具白名单和响应上限会在每次调用前重新校验。
                    </p>
                    <form id="resource-create-form" class="stack-form resource-create-form" on:submit|preventDefault={createResource}>
                      <div class="mcp-resource-form">
                      <label><span><i>*</i>传输方式</span><select bind:value={mcpTransport}><option value="streamable_http">Streamable HTTP</option></select></label>
                      <label class="mcp-url-field"><span><i>*</i>HTTPS 服务地址</span><input bind:value={mcpURL} type="url" placeholder="https://mcp.example.com/mcp" autocomplete="off" /></label>
                      <label class="mcp-tools-field"><span><i>*</i>允许的工具</span><textarea bind:value={mcpToolAllowlist} rows="5" placeholder="每行一个工具，例如 inventory.read&#10;alerts.list" spellcheck="false"></textarea><small>仅精确匹配的工具会被允许，不能使用通配符。</small></label>
                      <div class="mcp-number-grid"><label><span>调用超时（秒）</span><input bind:value={mcpTimeoutSeconds} type="number" min="1" max="60" /></label><label><span>最大响应字节</span><input bind:value={mcpMaxResponseBytes} type="number" min="1" max="1048576" step="1024" /></label></div>
                      </div>
                    </form>
                  {:else if resourceKind === 'AIProvider' && resourceAddStep === 2}
                    <p class="resource-add-description">
                      配置 Provider 连接、运行边界和用途标记。凭据会作为独立加密对象保存，不会写入资源配置。
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
                          ><option value="chat_completions"
                            >Chat Completions</option
                          ></select
                      ></label
                      >
                      <div class="provider-purpose-options provider-config-purpose">
                        <span>Provider用途</span>
                        <div>
                          {#each providerPurposeOptions as purpose}
                            {@const unavailableReason = providerPurposeUnavailableReason(purpose.value)}
                            <button class:active={providerPurposeTags.includes(purpose.value)} class:unavailable={!providerPurposeAvailable(purpose.value)} type="button" disabled={!providerPurposeAvailable(purpose.value)} data-tooltip={unavailableReason || undefined} aria-pressed={providerPurposeTags.includes(purpose.value)} on:click={() => toggleProviderPurpose(purpose.value)}>{purpose.label}</button>
                          {/each}
                        </div>
                      </div>
                      <label class="provider-config-url"
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
                      <label class="provider-config-api-key"
                        class:invalid={providerConfigurationAttempted && !providerAPIKey.trim()}
                        ><span>API Key</span><span class="provider-api-key-control"><input
                            bind:value={providerAPIKey}
                            required
                            type={providerAPIKeyVisible ? 'text' : 'password'}
                            placeholder={providerAPIKeyLoading ? '正在读取 API Key…' : '请输入 API Key'}
                            autocomplete="new-password"
                          /><button
                            class="provider-api-key-toggle"
                            type="button"
                            aria-label={providerAPIKeyVisible ? '隐藏 API Key' : '显示 API Key'}
                            aria-pressed={providerAPIKeyVisible}
                            data-tooltip={providerAPIKeyVisible ? '隐藏 API Key' : '显示 API Key'}
                            on:click={() => (providerAPIKeyVisible = !providerAPIKeyVisible)}
                            >{#if providerAPIKeyVisible}<EyeOff size={16} strokeWidth={1.8} aria-hidden="true" />{:else}<Eye size={16} strokeWidth={1.8} aria-hidden="true" />{/if}</button
                          ></span></label
                      >
                      <label class="provider-config-timeout"
                        ><span>请求超时（秒）</span><input
                          bind:value={providerTimeoutSeconds}
                          min="1"
                          max="300"
                          type="number"
                        /></label
                      >
                      <label class="provider-config-concurrency"
                        ><span>最大并发</span><input
                          bind:value={providerMaxConcurrency}
                          min="1"
                          type="number"
                        /></label
                      >
                      <label class="provider-config-rate-limit"
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
                        >
                        <label
                          class:invalid={providerModelConfigurationAttempted &&
                            providerModelDraft.contextWindowTokens <= 0}
                          ><span><i>*</i>上下文窗口</span><input
                            bind:value={providerModelDraft.contextWindowTokens}
                            min="1"
                            required
                            type="number"
                          /></label
                        >
                        <label
                          ><span>最大输出 Token</span><input
                            bind:value={providerModelDraft.maxOutputTokens}
                            min="1"
                            type="number"
                          /></label
                        >
                        <label
                          ><span>优先级</span><input
                            bind:value={providerModelDraft.priority}
                            min="0"
                            type="number"
                          /></label
                        >
                        <label
                          ><span>温度</span><span class="provider-temperature-control"><input
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
                          ></span></label
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
                      {#each providerModels as model}<div
                          class="provider-model-row"
                        >
                          <strong>{model.name}</strong><span
                            >{model.contextWindowTokens.toLocaleString()} Token ·
                            温度 {model.temperature}</span
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
                            ><span class="visually-hidden">启用 {model.name}</span><span class="provider-toggle-control"><input
                                type="checkbox"
                                checked={model.enabled}
                                disabled={model.name === providerDefaultModel}
                                aria-label={'启用 ' + model.name}
                                on:change={(event) =>
                                  setProviderModelEnabled(
                                    model.name,
                                    (event.currentTarget as HTMLInputElement).checked
                                  )}
                              /><i aria-hidden="true"></i></span></label
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
                      使用默认 Model 完成连接核验后，才可创建 Provider 并发布以下 Model 列表。
                    </p>
                    <form
                      id="provider-create-form"
                      class="provider-summary"
                      on:submit|preventDefault={submitProviderCreate}
                    >
                      <div>
                        <span>Provider</span><strong>{resourceName}</strong
                        ><small
                          >{providerTypeOptions.find(
                            (item) => item.value === providerType
                          )?.label} · {resourceStatus === 'active'
                            ? '已启用'
                            : '未启用'}</small
                        >
                      </div>
                      <div>
                        <span>服务地址</span><strong>{providerBaseURL}</strong
                        ><small
                          >{providerProtocol} · 超时 {providerTimeoutSeconds} 秒 ·
                          并发 {providerMaxConcurrency}</small
                        >
                      </div>
                      <div>
                        <span>默认 Model</span><strong
                          >{providerDefaultModel}</strong
                        ><small
                          >共 {providerModels.length} 个 Model，凭据将加密保存</small
                        >
                      </div>
                      <div class="provider-test-summary">
                        <span>连接核验</span>
                        {#if providerDraftTestBusy}
                          <strong>正在测试默认 Model...</strong>
                          <small>请求正在发送至 {providerDefaultModel}。</small>
                        {:else if providerDraftTestPassedState}
                          <strong class="success">连接正常 · {providerDraftTest?.result?.latency_ms} ms</strong>
                          <small>{providerDraftTest?.result?.message}</small>
                        {:else if providerDraftTest?.error}
                          <strong class="failed">连接失败</strong>
                          <small>{providerDraftTest.error}</small>
                        {:else}
                          <strong>尚未核验</strong>
                          <small>需验证默认 Model 可成功响应后才能创建。</small>
                        {/if}
                        <button
                          class="secondary provider-test-button"
                          type="button"
                          disabled={providerDraftTestBusy}
                          on:click={() => void testProviderDraftConnection()}
                          >{providerDraftTestBusy ? '测试中' : '连接测试'}</button
                        >
                      </div>
                      <div>
                        <span>资源属性</span><strong>{activeScopeSummary()}</strong
                        ><small>{resourceLabelsText({ labels: parseLabels(resourceLabels) } as Resource) ? '已配置的资源标签' : '未配置资源标签'}</small>
                      </div>
                      <div>
                        <span>用途标记</span><strong>{providerPurposeTags.length > 0 ? providerPurposeTags.map(providerPurposeLabel).join('、') : '未设置'}</strong
                        ><small>同级别同一标记会自动切换至此 Provider。</small>
                      </div>
                      <div class="provider-summary-models">
                        <span>Model 列表</span
                        >{#each providerModels as model}<div>
                            <strong>{model.name}</strong><small
                              >{model.contextWindowTokens.toLocaleString()} Token
                              · {model.capabilities
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
                      <div class="form-row"><label><span>资源名称</span><input bind:value={editResourceName} required /></label><label><span>状态</span><select bind:value={editResourceStatus}><option value="active">正常</option><option value="disabled">停用</option><option value="unknown">未知</option></select></label></div>
                      <label><span>标签</span><input bind:value={editResourceLabels} placeholder="env=prod, owner=platform" /></label>
                      <label><span>传输方式</span><select bind:value={mcpTransport}><option value="streamable_http">Streamable HTTP</option></select></label>
                      <label><span>HTTPS 服务地址</span><input bind:value={mcpURL} type="url" placeholder="https://mcp.example.com/mcp" /></label>
                      <label><span>允许的工具</span><textarea bind:value={mcpToolAllowlist} rows="6" placeholder="每行一个工具"></textarea><small>只允许登记的工具，调用前会再次发现并校验。</small></label>
                      <div class="mcp-number-grid"><label><span>调用超时（秒）</span><input bind:value={mcpTimeoutSeconds} type="number" min="1" max="60" /></label><label><span>最大响应字节</span><input bind:value={mcpMaxResponseBytes} type="number" min="1" max="1048576" step="1024" /></label></div>
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
                      {#if createSchema?.schema.properties}
                        <div class="schema-inputs">
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
                                    : field.type === 'url' ||
                                        field.format === 'uri'
                                      ? 'url'
                                      : 'text'}
                                  placeholder={field.description || key}
                                  autocomplete="off"
                                />{/if}</label
                            >{/each}
                        </div>
                      {:else}
                        <label
                          >配置 JSON<textarea
                            bind:value={resourceConfig}
                            rows="4"
                            spellcheck="false"
                          ></textarea></label
                        >
                      {/if}
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
                  <div class="connection-status">
                    <div class="connection-summary">
                      <span
                        class:success={connectionCheck?.status === 'succeeded'}
                        class:failed={connectionCheck?.status === 'failed'}
                        class="connection-indicator"
                        aria-hidden="true"
                      ></span>
                      <span>
                        <strong
                          >{connectionCheck?.status === 'succeeded'
                            ? '连接正常'
                            : connectionCheck?.status === 'failed'
                              ? '连接失败'
                              : '尚未测试'}</strong
                        >
                        <small
                          >{connectionCheck
                            ? `${connectionCheck.message} · ${connectionCheck.latency_ms} ms · ${formatDate(connectionCheck.checked_at)}`
                            : '当前资源还没有连接测试记录'}</small
                        >
                      </span>
                    </div>
                    {#if connectionCheck?.capabilities.length}
                      <div class="capability-list" aria-label="连接器能力">
                        {#each connectionCheck.capabilities as capability}
                          <span>{capabilityName(capability)}</span>
                        {/each}
                      </div>
                    {/if}
                    <button
                      class="secondary connection-test-button"
                      on:click={testSelectedResourceConnection}
                      disabled={busy || connectionBusy}
                    >
                      <span aria-hidden="true">↻</span>
                      {connectionBusy ? '测试中' : '测试连接'}
                    </button>
                  </div>
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
                        <div class="provider-purpose-options provider-config-purpose"><span>Provider用途</span><div>{#each providerPurposeOptions as purpose}<button class:active={providerPurposeTags.includes(purpose.value)} class:unavailable={!providerPurposeAvailable(purpose.value)} type="button" disabled={!providerPurposeAvailable(purpose.value)} data-tooltip={providerPurposeUnavailableReason(purpose.value) || undefined} aria-pressed={providerPurposeTags.includes(purpose.value)} on:click={() => toggleProviderPurpose(purpose.value)}>{purpose.label}</button>{/each}</div></div>
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
                      <label><span><i>*</i>传输方式</span><select bind:value={mcpTransport}><option value="streamable_http">Streamable HTTP</option></select></label>
                      <label class="mcp-url-field"><span><i>*</i>HTTPS 服务地址</span><input bind:value={mcpURL} type="url" placeholder="https://mcp.example.com/mcp" /></label>
                      <label class="mcp-tools-field"><span><i>*</i>允许的工具</span><textarea bind:value={mcpToolAllowlist} rows="6" placeholder="每行一个工具"></textarea><small>只允许登记的工具，调用前会再次发现并校验。</small></label>
                      <div class="mcp-number-grid"><label><span>调用超时（秒）</span><input bind:value={mcpTimeoutSeconds} type="number" min="1" max="60" /></label><label><span>最大响应字节</span><input bind:value={mcpMaxResponseBytes} type="number" min="1" max="1048576" step="1024" /></label></div>
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
                      : '继承资源仅可查看'}>保存修改</button
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
      {:else if view === 'inspection'}
        <section class="content-grid">
          <section class="panel wide-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">NEW POLICY</p>
                <h2>创建巡检策略</h2>
              </div>
            </div>
            <form
              class="stack-form"
              on:submit|preventDefault={createInspectionPolicy}
            >
              <div class="form-grid">
                <label
                  >名称<input
                    bind:value={inspectionPolicyName}
                    required
                    maxlength="200"
                  /></label
                >
                <label>Cron<input bind:value={inspectionCron} required /></label
                >
                <label
                  >时区<input bind:value={inspectionTimezone} required /></label
                >
                <label
                  >超时（秒）<input
                    type="number"
                    min="1"
                    max="3600"
                    bind:value={inspectionTimeoutSeconds}
                  /></label
                >
                <label
                  >重试次数<input
                    type="number"
                    min="0"
                    max="10"
                    bind:value={inspectionRetries}
                  /></label
                >
                <label
                  >目标并发<input
                    type="number"
                    min="1"
                    max="64"
                    bind:value={inspectionMaxConcurrent}
                  /></label
                >
                <label
                  >Tool 预算<input
                    type="number"
                    min="1"
                    max="100"
                    bind:value={inspectionMaxToolCalls}
                  /></label
                >
                <label
                  >Token 预算<input
                    type="number"
                    min="1"
                    max="200000"
                    bind:value={inspectionMaxTokens}
                  /></label
                >
              </div>
              <label
                >标签选择器（JSON 对象）<textarea
                  rows="3"
                  bind:value={inspectionTargetLabels}
                ></textarea></label
              >
              <fieldset>
                <legend>目标资源</legend>
                <div class="check-grid">
                  {#each executableTargets as target}
                    <label class="check-row"
                      ><input
                        type="checkbox"
                        checked={inspectionTargetIds.includes(target.id)}
                        on:change={() =>
                          (inspectionTargetIds = toggleInspectionSelection(
                            inspectionTargetIds,
                            target.id
                          ))}
                      />{target.name} · {target.kind}</label
                    >
                  {/each}
                </div>
              </fieldset>
              <label
                >解释 AgentProfile（可选）<select
                  bind:value={inspectionAgentProfileId}
                >
                  <option value="">使用内置巡检解释 Agent</option>
                  {#each agentProfileResources.filter((item) => resourceInActiveWorkspace(item) && item.status === 'active') as profile}
                    <option value={profile.id}
                      >{profile.name} · {scopeName(profile.scope_id)}</option
                    >
                  {/each}
                </select></label
              >
              <button
                class="primary"
                disabled={busy ||
                  !inspectionPolicyName ||
                  (inspectionTargetIds.length === 0 &&
                    inspectionTargetLabels.trim() === '{}')}>创建策略</button
              >
            </form>
          </section>
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">POLICIES</p>
                <h2>巡检策略</h2>
              </div>
              <span class="count">{inspectionPolicies.length}</span>
            </div>
            <div class="table-list">
              {#each inspectionPolicies as policy}<article class="list-row">
                  <div>
                    <strong>{policy.name}</strong>
                    <p>
                      {policy.cron} · {policy.timezone} · {policy
                        .target_resource_ids.length} 个目标 · {policy.status}
                    </p>
                  </div>
                  <div class="inline-actions">
                    <button
                      class="quiet-button"
                      disabled={busy || policy.status !== 'active'}
                      on:click={() => rerunInspection(policy.id)}
                      >立即运行</button
                    ><button
                      class="quiet-button"
                      disabled={busy}
                      on:click={() =>
                        setInspectionPolicyStatus(
                          policy.id,
                          policy.status === 'active' ? 'disabled' : 'active'
                        )}
                      >{policy.status === 'active' ? '停止' : '恢复'}</button
                    >
                  </div>
                </article>{:else}<p class="empty-state">
                  当前作用域还没有巡检策略。
                </p>{/each}
            </div>
          </section>
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">HEALTH</p>
                <h2>最近运行</h2>
              </div>
              <span class="count">{inspectionRuns.length}</span>
            </div>
            <div class="table-list">
              {#each inspectionRuns as run}<article class="list-row">
                  <div>
                    <strong>{run.score ?? '—'} 分 · {run.status}</strong>
                    <p>
                      {new Date(run.window_start).toLocaleString()} · LLM {run.llm_status}
                    </p>
                  </div>
                </article>{:else}<p class="empty-state">
                  尚无运行记录。
                </p>{/each}
            </div>
          </section>
          <section class="panel wide-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">FINDINGS</p>
                <h2>异常与恢复</h2>
              </div>
              <span class="count">{inspectionFindings.length}</span>
            </div>
            <div class="table-list">
              {#each inspectionFindings as finding}<article class="list-row">
                  <div>
                    <strong>{finding.severity} · {finding.rule}</strong>
                    <p>{finding.message || '无补充说明'} · {finding.status}</p>
                  </div>
                </article>{:else}<p class="empty-state">
                  没有已记录的异常。
                </p>{/each}
            </div>
          </section>
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">WEBHOOKS</p>
                <h2>通知渠道</h2>
              </div>
              <span class="count">{notificationChannels.length}</span>
            </div>
            <div class="table-list">
              {#each notificationChannels as channel}<article class="list-row">
                  <div>
                    <strong>{channel.name}</strong>
                    <p>
                      {channel.kind} · {channel.status} · 每分钟 {channel.rate_limit_per_minute}
                      次
                    </p>
                  </div>
                </article>{:else}<p class="empty-state">
                  当前作用域没有启用的通知渠道。
                </p>{/each}
            </div>
            <form
              class="stack-form compact-form"
              on:submit|preventDefault={createNotificationChannel}
            >
              <label
                >名称<input
                  bind:value={channelName}
                  required
                  maxlength="120"
                /></label
              >
              <label
                >HTTPS Webhook<input
                  type="url"
                  pattern="https://.*"
                  bind:value={channelWebhookURL}
                  required
                /></label
              >
              <label
                >每分钟上限<input
                  type="number"
                  min="1"
                  max="10000"
                  bind:value={channelRateLimit}
                /></label
              >
              <button class="primary" disabled={busy}>添加渠道</button>
            </form>
          </section>
        </section>
      {:else if view === 'operations'}
        <section class="content-grid two-column">
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">APPROVAL WORKFLOW</p>
                <h2>创建受控操作</h2>
              </div>
              <span class="scope-type">Medium+ 需人工审批</span>
            </div>
            <form
              class="stack-form compact-form"
              on:submit|preventDefault={createOperationRequest}
            >
              <label
                >目标资源<select bind:value={operationTargetId} required
                  ><option value="" disabled>选择可访问资源</option
                  >{#each resources.filter((item) => item.scope_id === selectedScopeId && item.status === 'active') as item}<option
                      value={item.id}
                      >{item.name} · {resourceSchemaName(item.kind)}</option
                    >{/each}</select
                ></label
              >
              <div class="form-row">
                <label
                  >操作<select bind:value={operationName}
                    ><option value="kubernetes.restart_workload"
                      >重启 Kubernetes 工作负载</option
                    ><option value="kubernetes.scale_workload"
                      >扩缩容 Kubernetes 工作负载</option
                    ></select
                  ></label
                ><label
                  >风险<select bind:value={operationRisk}
                    ><option value="low">Low</option><option value="medium"
                      >Medium（默认审批）</option
                    ><option value="high">High（默认审批）</option></select
                  ></label
                >
              </div>
              <label
                >精确参数 JSON<textarea
                  bind:value={operationParameters}
                  rows="5"
                  spellcheck="false"
                  required
                ></textarea></label
              >
              <label
                >影响范围<input
                  bind:value={operationImpact}
                  placeholder="说明可能影响的应用、副本或访问窗口"
                /></label
              >
              <label
                >回滚建议<input
                  bind:value={operationRollback}
                  placeholder="例如恢复到原副本数"
                /></label
              >
              <p class="form-hint">
                提交会生成参数哈希；参数变更后原审批自动无效。删除和写 SQL
                被系统永久拒绝。
              </p>
              <button class="primary" disabled={busy || !operationTargetId}
                >创建 dry-run 请求</button
              >
            </form>
          </section>
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">MCP SERVERS</p>
                <h2>MCP 工具快照</h2>
              </div>
              <span class="scope-type">外部内容不可信</span>
            </div>
            <div class="table-list">
              {#each resources.filter((item) => item.kind === 'MCPServer' && item.scope_id === selectedScopeId) as server}<article
                  class="list-row"
                >
                  <div>
                    <strong>{server.name}</strong>
                    <p>
                      {(operationSnapshots[server.id] ?? [])[0]?.tools
                        ?.length ?? 0} 个已发现且允许的工具；描述和响应一律按不可信文本处理。
                    </p>
                    {#if (operationSnapshots[server.id] ?? [])[0]}<small
                        >快照 {(operationSnapshots[server.id] ??
                          [])[0].content_hash.slice(0, 12)} · {(operationSnapshots[
                          server.id
                        ] ?? [])[0].status}</small
                      >{/if}
                  </div>
                  <button
                    class="quiet-button"
                    disabled={busy}
                    on:click={() => discoverMCP(server.id)}
                    >发现 / 健康检查</button
                  >
                </article>{:else}<p class="empty-state">
                  当前作用域没有 MCPServer 资源。先在资源目录以 HTTPS URL
                  和工具白名单登记。
                </p>{/each}
            </div>
          </section>
          <section class="panel wide-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">AUDITABLE REQUESTS</p>
                <h2>操作请求与审批</h2>
              </div>
              <span class="count">{operationRequests.length}</span>
            </div>
            <div class="table-list">
              {#each operationRequests as item}<article class="list-row">
                  <div>
                    <strong
                      >{item.operation_name} · {item.risk_level} · {item.status}</strong
                    >
                    <p>
                      目标 {resources.find(
                        (resource) => resource.id === item.target_resource_id
                      )?.name ?? item.target_resource_id.slice(0, 8)} · 参数哈希 {item.parameters_hash.slice(
                        0,
                        12
                      )} · {item.expires_at
                        ? `有效至 ${formatDate(item.expires_at)}`
                        : '无需人工审批'}
                    </p>
                    <small
                      >影响：{item.impact_summary ||
                        '未填写'}；回滚：{item.rollback_summary ||
                        '未填写'}</small
                    >
                    <pre class="config-preview">{JSON.stringify(
                        item.parameters,
                        null,
                        2
                      )}</pre>
                  </div>
                  <div class="inline-actions">
                    {#if item.status === 'pending'}<button
                        class="quiet-button"
                        disabled={busy}
                        on:click={() => approveOperation(item, 'approved')}
                        >批准</button
                      ><button
                        class="quiet-button"
                        disabled={busy}
                        on:click={() => approveOperation(item, 'rejected')}
                        >拒绝</button
                      >{:else if item.status === 'approved'}<button
                        class="primary"
                        disabled={busy}
                        on:click={() => startOperation(item)}>开始执行</button
                      >{/if}
                  </div>
                </article>{:else}<p class="empty-state">
                  尚无操作请求。所有请求、审批和执行结果都会进入审计记录。
                </p>{/each}
            </div>
          </section>
        </section>
      {:else if view === 'diagnosis'}
        <section
          class="diagnosis-workbench-f"
          class:history-collapsed={diagnosisHistoryCollapsed}
          class:context-collapsed={diagnosisContextCollapsed}
          style={`--diagnosis-history-width:${diagnosisHistoryWidth}px;--diagnosis-context-width:${diagnosisContextWidth}px`}
        >
          {#if !diagnosisHistoryCollapsed}
            <aside class="diagnosis-history-panel">
              <div class="diagnosis-panel-top">
                <div>
                  <h2>会话历史</h2>
                  <small>{diagnosisSessions.length} 个会话</small>
                </div>
                <div class="diagnosis-heading-actions">
                  <button
                    class="icon-button"
                    aria-label="清空会话历史"
                    title="清空会话历史"
                    on:click={clearDiagnosisHistory}
                    ><Trash2 size={15} /></button
                  >
                  <button
                    class="icon-button"
                    aria-label="新建诊断会话"
                    title="新建诊断会话"
                    on:click={newDiagnosisSession}><Plus size={16} /></button
                  >
                </div>
              </div>
              <input
                class="diagnosis-session-search"
                bind:value={diagnosisSessionSearch}
                placeholder="搜索会话"
                aria-label="搜索会话"
              />
              <div class="diagnosis-session-list-f">
                {#each diagnosisSessions.filter((session) => !diagnosisSessionSearch.trim() || (session.title || '')
                      .toLowerCase()
                      .includes(diagnosisSessionSearch.toLowerCase())) as session}
                  <div class="diagnosis-session-item">
                    <button
                      class:active={selectedDiagnosisId === session.id}
                      class="diagnosis-session-row-f"
                      on:click={() => void openDiagnosis(session.id)}
                    >
                      <strong class="diagnosis-session-title-f"
                        >{session.title || '未命名诊断'}</strong
                      >
                      <span class="diagnosis-session-meta-f"
                        ><small>{formatDate(session.created_at)}</small><em
                          class={`diagnosis-session-status-f ${session.status}`}
                          >{diagnosisStatusLabel(session.status)}</em
                        ></span
                      >
                    </button>
                    <div class="diagnosis-session-actions">
                      <button
                        aria-label="重命名会话"
                        title="重命名会话"
                        on:click|stopPropagation={() =>
                          renameDiagnosisSession(session)}
                        ><Pencil size={13} /></button
                      >
                      <button
                        aria-label="删除会话"
                        title="删除会话"
                        on:click|stopPropagation={() =>
                          deleteDiagnosisSession(session)}
                        ><Trash2 size={13} /></button
                      >
                    </div>
                  </div>
                {:else}<p class="diagnosis-empty">还没有诊断会话。</p>{/each}
              </div>
            </aside>
          {/if}
          <div
            class="diagnosis-splitter left"
            role="separator"
            aria-orientation="vertical"
            on:pointerdown={(event) => startDiagnosisResize('history', event)}
          >
            <button
              aria-label={diagnosisHistoryCollapsed
                ? '展开会话历史'
                : '折叠会话历史'}
              on:click={() =>
                (diagnosisHistoryCollapsed = !diagnosisHistoryCollapsed)}
              >{#if diagnosisHistoryCollapsed}<ChevronRight
                  size={16}
                />{:else}<ChevronLeft size={16} />{/if}</button
            >
          </div>
          <section class="diagnosis-conversation-f">
            <header class="diagnosis-conversation-head">
              <div class="diagnosis-conversation-title">
                <h1>{diagnosisSnapshot?.session.title || '新建诊断会话'}</h1>
                <small>{activeScope?.name ?? '当前级别'} · 只读证据链</small>
              </div>
              <div class="diagnosis-loaded-context">
                <span>已加载上下文</span>
                {#each diagnosisTargets
                  .filter( (resource) => diagnosisTargetIds.includes(resource.id) )
                  .slice(0, 3) as resource}<span class="diagnosis-context-chip"
                    >{resource.name}</span
                  >{/each}
                {#if diagnosisTargetIds.length > 3}<span
                    class="diagnosis-context-chip accent"
                    >+{diagnosisTargetIds.length - 3}</span
                  >{/if}
                {#if diagnosisTargetIds.length === 0}<span
                    class="diagnosis-context-chip muted">未选择</span
                  >{/if}
              </div>
              <div class="diagnosis-head-actions">
                <button
                  class="icon-button super-session-button"
                  aria-label="超级会话"
                  title="超级会话"
                  on:click={() =>
                    (notice = '超级会话已启用，将持续保留当前上下文。')}
                  ><Sparkles size={16} /></button
                >
                <span class="diagnosis-head-status"
                  ><i class:running={diagnosisGenerating}
                  ></i>{diagnosisGenerating
                    ? '正在生成回答'
                    : diagnosisSnapshot
                      ? diagnosisStatusLabel(diagnosisSnapshot.session.status)
                      : '等待提问'}</span
                >
              </div>
            </header>
            <div class="diagnosis-message-list-f">
              {#if !diagnosisSnapshot}<div class="diagnosis-welcome">
                  <span class="diagnosis-welcome-icon"
                    ><Stethoscope size={23} /></span
                  >
                  <h2>从一个问题开始</h2>
                  <p>
                    可直接提问；如需查询资源状态、内容或性能，请在右侧选择已授权的上下文资源。
                    AIEngine 会按需调用受控只读工具。
                  </p>
                </div>{/if}
              {#if diagnosisSnapshot}{#each diagnosisSnapshot.messages as message, index}
                  <article class="diagnosis-message-f {message.role}">
                    <span class="diagnosis-message-avatar"
                      >{message.role === 'assistant' ? 'AI' : '你'}</span
                    >
                    <div class="diagnosis-message-content">
                      <div class="diagnosis-message-meta">
                        {#if message.role === 'assistant'}<strong
                            >AI 助手</strong
                          >{/if}<small>{formatDate(message.created_at)}</small>
                      </div>
                      {#if diagnosisEditingMessageId === message.id}
                        <div class="diagnosis-edit-box">
                          <textarea
                            bind:value={diagnosisEditDraft}
                            aria-label="编辑诊断问题"
                            rows="3"
                          ></textarea>
                          <div>
                            <button
                              class="secondary"
                              on:click={() => (diagnosisEditingMessageId = '')}
                              >取消</button
                            ><button
                              class="primary"
                              on:click={() => void saveDiagnosisEdit()}
                              disabled={busy}>重新发送</button
                            >
                          </div>
                        </div>
                      {:else}
                        {#if message.role === 'assistant' && !diagnosisGenerating && index === diagnosisSnapshot.messages
                              .map((item) => item.role)
                              .lastIndexOf('assistant')}{#each diagnosisActionData(diagnosisSnapshot) as actionGroup}
                            <div class="diagnosis-action-group">
                              <button class="diagnosis-action-row" aria-expanded={Boolean(diagnosisActionExpanded[actionGroup.id])} on:click={() => (diagnosisActionExpanded = { ...diagnosisActionExpanded, [actionGroup.id]: !diagnosisActionExpanded[actionGroup.id] })}><span class="diagnosis-action-chevron">{diagnosisActionExpanded[actionGroup.id] ? '⌄' : '›'}</span><span class="diagnosis-action-icon" aria-hidden="true"><ClipboardCheck size={13} /></span><strong>{actionGroup.title}</strong><em class:running={actionGroup.status === '进行中'}>{actionGroup.status}</em><small>{actionGroup.duration} · {actionGroup.children.length} 项</small></button>
                              {#if diagnosisActionExpanded[actionGroup.id]}<div class="diagnosis-action-children">{#each actionGroup.children as child}<div><button class="diagnosis-action-row child" aria-expanded={Boolean(diagnosisActionChildren[child.id])} on:click={() => (diagnosisActionChildren = { ...diagnosisActionChildren, [child.id]: !diagnosisActionChildren[child.id] })}><span class="diagnosis-action-chevron">{diagnosisActionChildren[child.id] ? '⌄' : '›'}</span><span class="diagnosis-action-icon" aria-hidden="true"><PlugZap size={13} /></span><strong>{child.title}</strong><em>{child.status}</em><small>{child.duration}</small></button>{#if diagnosisActionChildren[child.id]}<div class="diagnosis-action-detail"><pre>{child.input}</pre><pre>{child.output}</pre></div>{/if}</div>{/each}</div>{/if}
                            </div>
                          {/each}{/if}
                        <div class="diagnosis-bubble-f">
                          <div class="diagnosis-markdown">{@html renderDiagnosisMarkdown(message.content)}</div>
                          {#if diagnosisInterruptedReason && message.role === 'assistant' && index === diagnosisSnapshot.messages
                                .map((item) => item.role)
                                .lastIndexOf('assistant')}<span
                              class="diagnosis-interruption"
                              ><i
                              ></i>回答已中断：{diagnosisInterruptedReason}</span
                            >{/if}{#if message.role === 'assistant'}<button
                              class="bubble-icon copy"
                              aria-label="复制回答"
                              title="复制回答"
                              on:click={() =>
                                copyDiagnosisAnswer(message.content)}
                              ><Copy size={14} /></button
                            >{/if}{#if message.role === 'user' && isLastDiagnosisUser(index)}<button
                              class="bubble-icon edit"
                              aria-label="编辑并重新发送"
                              title="编辑并重新发送"
                              on:click={() => beginDiagnosisEdit(message)}
                              ><Pencil size={14} /></button
                            >{/if}
                        </div>
                      {/if}
                    </div>
                  </article>
                {/each}{/if}
              {#if diagnosisGenerating || diagnosisStreamingText || diagnosisInterruptedReason}
                {#each diagnosisSnapshot ? diagnosisActionData(diagnosisSnapshot) : [] as actionGroup}
                  <div class="diagnosis-action-group diagnosis-live-actions">
                    <button
                      class="diagnosis-action-row"
                      aria-expanded={Boolean(diagnosisActionExpanded[actionGroup.id])}
                      on:click={() =>
                        (diagnosisActionExpanded = {
                          ...diagnosisActionExpanded,
                          [actionGroup.id]: !diagnosisActionExpanded[actionGroup.id]
                        })}
                    ><span class="diagnosis-action-chevron">{diagnosisActionExpanded[actionGroup.id] ? '⌄' : '›'}</span><span class="diagnosis-action-icon" aria-hidden="true"><ClipboardCheck size={13} /></span><strong>{actionGroup.title}</strong><em class:running={actionGroup.status === '进行中'}>{actionGroup.status}</em><small>{actionGroup.duration} · {actionGroup.children.length} 项</small></button>
                    {#if diagnosisActionExpanded[actionGroup.id]}<div class="diagnosis-action-children">
                      {#each actionGroup.children as child}<div>
                        <button
                          class="diagnosis-action-row child"
                          aria-expanded={Boolean(diagnosisActionChildren[child.id])}
                          on:click={() =>
                            (diagnosisActionChildren = {
                              ...diagnosisActionChildren,
                              [child.id]: !diagnosisActionChildren[child.id]
                            })}
                        ><span class="diagnosis-action-chevron">{diagnosisActionChildren[child.id] ? '⌄' : '›'}</span><span class="diagnosis-action-icon" aria-hidden="true"><PlugZap size={13} /></span><strong>{child.title}</strong><em>{child.status}</em><small>{child.duration}</small></button>
                        {#if diagnosisActionChildren[child.id]}<div class="diagnosis-action-detail"><pre>{child.input}</pre><pre>{child.output}</pre></div>{/if}
                      </div>{/each}
                    </div>{/if}
                  </div>
                {/each}
                <article class="diagnosis-message-f assistant diagnosis-streaming-message">
                  <span class="diagnosis-message-avatar">AI</span>
                  <div class="diagnosis-message-content">
                    <div class="diagnosis-message-meta"><strong>AI 助手</strong><small>实时响应</small></div>
                    <div class="diagnosis-bubble-f diagnosis-streaming-bubble">
                      {#if diagnosisStreamingText}<div class="diagnosis-markdown">{@html renderDiagnosisMarkdown(diagnosisStreamingText)}</div>{:else}<span class="diagnosis-thinking"><i></i><i></i><i></i><span>正在思考</span></span>{/if}
                      {#if diagnosisInterruptedReason}<span class="diagnosis-interruption"><i></i>回答已中断：{diagnosisInterruptedReason}</span>{/if}
                    </div>
                  </div>
                </article>
              {/if}
            </div>
            <form
              class="diagnosis-composer-f"
              on:submit|preventDefault={() => void submitDiagnosisMessage()}
            >
              <div class="diagnosis-composer-shell">
                <textarea
                  bind:value={diagnosisComposerText}
                  on:keydown={handleDiagnosisComposerKeydown}
                  placeholder="描述问题，或输入 / 调用 Skill…"
                  aria-label="输入诊断问题"
                  rows="3"
                  maxlength="16000"
                ></textarea>
                <div class="diagnosis-composer-tools">
                  <div>
                    <button
                      type="button"
                      class="diagnosis-tool"
                      title="添加附件"
                      on:click={() => (notice = '附件入口已打开。')}
                      ><Paperclip size={15} />附件</button
                    ><button
                      type="button"
                      class="diagnosis-tool"
                      title="添加链接"
                      on:click={() => (notice = '链接入口已打开。')}
                      ><Link2 size={15} />链接</button
                    ><button
                      type="button"
                      class="diagnosis-tool"
                      title="选择 Skills"
                      on:click={() =>
                        (notice =
                          'Skills：指标查询、日志查询、Kubernetes 只读查询。')}
                      ><Sparkles size={15} />Skills</button
                    ><button
                      type="button"
                      class="diagnosis-tool"
                      title="选择 Agent"
                      on:click={() => (notice = 'Agent：故障定位 Agent。')}
                      ><Bot size={15} />Agent</button
                    >
                  </div>
                  <div>
                    <select
                      bind:value={selectedProviderId}
                      aria-label="选择模型服务商"
                      >{#each diagnosisAvailableProviders as provider}<option
                          value={provider.provider_resource_id}
                          >{provider.name}</option
                        >{:else}<option value=""
                          >当前作用域暂无可用模型服务商</option
                        >{/each}</select
                    ><select bind:value={llmModelName} aria-label="选择模型"
                      ><option value="">选择模型</option
                      >{#each diagnosisProviderModels as model}<option
                          value={String(model.name ?? '')}
                          >{String(model.name ?? '')}</option
                        >{/each}</select
                    ><button
                      class="primary diagnosis-send-button"
                      type="button"
                      disabled={busy ||
                        (!diagnosisComposerText.trim() && !diagnosisGenerating)}
                      on:click={() =>
                        diagnosisGenerating
                          ? stopDiagnosisGeneration()
                          : void submitDiagnosisMessage()}
                      >{#if diagnosisGenerating}<Square
                          size={14}
                          fill="currentColor"
                        />停止{:else}<Send size={14} />发送{/if}</button
                    >
                  </div>
                </div>
              </div>
              <small class="diagnosis-composer-note"
                ><span>Enter 发送 · Shift + Enter 换行</span><span
                  >当前模型支持：文本、工具调用、流式输出</span
                ></small
              >
            </form>
          </section>
          <div
            class="diagnosis-splitter right"
            role="separator"
            aria-orientation="vertical"
            on:pointerdown={(event) => startDiagnosisResize('context', event)}
          >
            <button
              aria-label={diagnosisContextCollapsed
                ? '展开诊断上下文'
                : '折叠诊断上下文'}
              on:click={() =>
                (diagnosisContextCollapsed = !diagnosisContextCollapsed)}
              >{#if diagnosisContextCollapsed}<ChevronLeft
                  size={16}
                />{:else}<ChevronRight size={16} />{/if}</button
            >
          </div>
          {#if !diagnosisContextCollapsed}
            <aside class="diagnosis-context-panel-f">
              <div class="diagnosis-panel-top">
                <div>
                  <h2>诊断上下文</h2>
                  <small
                    >{diagnosisTargetIds.length} / {diagnosisTargets.length} 已加载</small
                  >
                </div>
              </div>
              <div class="diagnosis-context-tabs">
                <button
                  class:active={diagnosisContextTab === 'context'}
                  on:click={() => (diagnosisContextTab = 'context')}
                  >上下文</button
                ><button
                  class:active={diagnosisContextTab === 'evidence'}
                  on:click={() => (diagnosisContextTab = 'evidence')}
                  >证据链</button
                >
              </div>
              {#if diagnosisContextTab === 'context'}
                <p class="diagnosis-context-note">
                  <strong>上下文开关</strong><br />只把打开的资源提供给当前
                  Agent；关闭不会删除资源，也不会影响权限。
                </p>
                <div class="diagnosis-resource-list-f">
                  {#each diagnosisTargets as resource}<label
                      class:selected={diagnosisTargetIds.includes(resource.id)}
                      ><span class="diagnosis-resource-icon"
                        >{resourceIcon(resource.kind)}</span
                      ><span
                        ><strong>{resource.name}</strong><small
                          >{resourceSchemaName(resource.kind)} · {scopeName(
                            resource.scope_id
                          )}</small
                        ></span
                      ><input
                        type="checkbox"
                        checked={diagnosisTargetIds.includes(resource.id)}
                        on:change={() => toggleDiagnosisContext(resource.id)}
                      /></label
                    >{:else}<p class="diagnosis-empty">
                      当前作用域没有可用于诊断的活动资源。
                    </p>{/each}
                </div>
              {:else}
                <div class="diagnosis-evidence-list-f">
                  {#each diagnosisSnapshot?.evidence ?? [] as evidence}<button
                      class:active={selectedEvidence?.id === evidence.id}
                      on:click={() => (selectedEvidence = evidence)}
                      ><strong
                        >{evidence.capability || 'Connector 只读结果'}</strong
                      ><small
                        >{formatDate(evidence.collected_at)} · {evidence.partial
                          ? '部分结果'
                          : '完整结果'} · 不可信输入</small
                      ></button
                    >{:else}<p class="diagnosis-empty">
                      执行完成后，Connector 返回的只读结果会出现在这里。
                    </p>{/each}
                </div>
                {#if selectedEvidence}<pre
                    class="diagnosis-evidence-detail">{JSON.stringify(
                      selectedEvidence.content,
                      null,
                      2
                    )}</pre>{/if}
              {/if}
            </aside>
          {/if}
        </section>
      {:else if view === 'agent'}
        <section class="content-grid two-column ai-runtime">
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">AGENT PROFILES</p>
                <h2>Agent 专家配置</h2>
              </div>
              <span class="count">{agentProfileResources.length}</span>
            </div>
            <div class="stack-form compact-form">
              <label
                >AgentProfile<select
                  bind:value={selectedAgentProfileId}
                  on:change={loadAgentProfileVersions}
                  ><option value="" disabled>选择 AgentProfile</option
                  >{#each agentProfileResources as item}<option value={item.id}
                      >{item.name} · {scopeName(item.scope_id)}</option
                    >{/each}</select
                ></label
              >
              <label
                >已发布版本<select bind:value={selectedAgentProfileVersionId}
                  ><option value="">选择版本</option
                  >{#each agentProfileVersions as version}<option
                      value={version.id}
                      >v{version.version} · {version.status}</option
                    >{/each}</select
                ></label
              >
              <button
                class="primary"
                type="button"
                disabled={busy ||
                  !selectedAgentProfileId ||
                  !selectedAgentProfileVersionId}
                on:click={publishAgentProfileVersion}>发布版本</button
              >
              {#if agentProfileVersions.length === 0}<p class="muted-copy">
                  版本发布后，AIEngine
                  执行时会固定使用已发布的专家指令和工具契约。
                </p>{/if}
            </div>
          </section>
          <section class="panel wide-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">NEW PROFILE</p>
                <h2>创建 AgentProfile</h2>
              </div>
              <span class="scope-type">版本化契约</span>
            </div>
            <form
              class="stack-form"
              on:submit|preventDefault={createAgentProfile}
            >
              <div class="form-row">
                <label
                  >名称<input
                    bind:value={agentProfileName}
                    required
                    placeholder="例如：PostgreSQL 故障专家"
                  /></label
                ><label
                  >适用资源类型<input
                    bind:value={agentProfileTargetKinds}
                    required
                    placeholder="Application, PostgreSQL"
                  /></label
                >
              </div>
              <label
                >专家指令<textarea
                  bind:value={agentProfileInstruction}
                  rows="5"
                  required
                  placeholder="描述诊断范围、判断原则和输出要求"
                ></textarea></label
              >
              <div class="form-row">
                <label
                  >模型能力<input
                    bind:value={agentProfileCapabilities}
                    placeholder="text, tool_calling, stream"
                  /></label
                ><label
                  >允许工具<input
                    bind:value={agentProfileAllowedTools}
                    placeholder="connector_postgresql_inspect"
                  /></label
                >
              </div>
              <div class="form-row">
                <label
                  >输入 Schema<textarea
                    bind:value={agentProfileInputSchema}
                    rows="4"
                    spellcheck="false"
                  ></textarea></label
                ><label
                  >输出 Schema<textarea
                    bind:value={agentProfileOutputSchema}
                    rows="4"
                    spellcheck="false"
                  ></textarea></label
                >
              </div>
              <button class="primary" disabled={busy || !selectedScopeId}
                >创建并发布 v1</button
              >
            </form>
          </section>
          <section class="panel wide-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">RELEASE HISTORY</p>
                <h2>版本历史</h2>
              </div>
              <span class="count">{agentProfileVersions.length}</span>
            </div>
            <div class="table-list">
              {#each agentProfileVersions as version}<div
                  class="list-row static"
                >
                  <span
                    ><strong>v{version.version}</strong><small
                      >{formatDate(version.created_at)} · {version.status}</small
                    ></span
                  ><span class="status-label {version.status}"
                    >{version.status === 'published'
                      ? '已发布'
                      : version.status === 'disabled'
                        ? '已停用'
                        : '草稿'}</span
                  >
                </div>{:else}<div class="empty-state">
                  选择 AgentProfile 后显示版本历史。
                </div>{/each}
            </div>
          </section>
        </section>
      {:else if view === 'skill'}
        <section class="content-grid two-column ai-runtime">
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">SKILL REGISTRY</p>
                <h2>Skill 版本</h2>
              </div>
              <span class="count">{skillVersions.length}</span>
            </div>
            <div class="stack-form compact-form">
              <label
                >Skill<select
                  bind:value={selectedSkillId}
                  required
                  on:change={loadSkillVersions}
                  ><option value="" disabled>选择 Skill 资源</option
                  >{#each skillResources as item}<option value={item.id}
                      >{item.name} · {scopeName(item.scope_id)}</option
                    >{/each}</select
                ></label
              >
              <label
                >版本<select bind:value={selectedSkillVersionId}
                  ><option value="" disabled>选择版本</option
                  >{#each skillVersions as version}<option value={version.id}
                      >v{version.version} · {version.status} · {version.risk_level}</option
                    >{/each}</select
                ></label
              >
              <div class="form-actions">
                <button
                  class="secondary"
                  type="button"
                  on:click={setSkillDefault}
                  disabled={busy || !selectedSkillVersionId}
                  >设为当前 Scope 默认</button
                >
                <button
                  class="primary"
                  type="button"
                  on:click={publishSkillVersion}
                  disabled={busy || !selectedSkillVersionId}>发布版本</button
                >
              </div>
            </div>
          </section>

          <section class="panel wide-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">NEW VERSION</p>
                <h2>创建 Skill 草稿</h2>
              </div>
              <span class="scope-type">不可变版本</span>
            </div>
            <form
              class="stack-form"
              on:submit|preventDefault={createSkillVersion}
            >
              <label
                >Agent Instruction<textarea
                  bind:value={skillInstruction}
                  rows="5"
                  required
                  placeholder="明确目标、边界与输出 JSON 结构"
                ></textarea></label
              >
              <label
                >适用资源类型<input
                  bind:value={skillTargetKinds}
                  required
                  placeholder="Application, Kubernetes"
                /></label
              >
              <fieldset class="skill-tool-picker">
                <legend>允许调用的 Connector 工具</legend>
                <p>仅已勾选的只读工具会暴露给模型；每个版本创建后不可修改。</p>
                <div class="skill-tool-grid">
                  {#each skillToolOptions as tool}
                    <label
                      class:selected={selectedSkillToolNames.includes(
                        tool.name
                      )}
                    >
                      <input
                        type="checkbox"
                        checked={selectedSkillToolNames.includes(tool.name)}
                        on:change={() => toggleSkillTool(tool.name)}
                      />
                      <span
                        ><strong>{tool.title}</strong><small
                          >{tool.description}</small
                        ></span
                      >
                    </label>
                  {/each}
                </div>
              </fieldset>
              <div class="form-row">
                <label
                  >输入 Schema<textarea
                    bind:value={skillInputSchema}
                    rows="5"
                    spellcheck="false"
                  ></textarea></label
                >
                <label
                  >输出 Schema<textarea
                    bind:value={skillOutputSchema}
                    rows="5"
                    spellcheck="false"
                  ></textarea></label
                >
              </div>
              <button class="primary" disabled={busy || !selectedSkillId}
                >创建版本</button
              >
            </form>
          </section>
        </section>
      {:else if view === 'access'}
        <section class="access-page">
          <section class="panel access-workbench">
            {#if accessTab !== 'roles'}
              <div class="access-filterbar">
                <div class="access-filter-copy">
                  <div>
                    <h2>{accessTab === 'teams' ? '团队列表' : '用户列表'}</h2>
                    <p>
                      {accessTab === 'teams'
                        ? '展开团队可查看其成员与项目。仅平台管理员可添加、编辑或删除团队。'
                        : '角色包含直接授权和成员组继承授权；管理员仅可授权、删除或重置其他用户的密码。'}
                    </p>
                  </div>
                  <span class="access-count"
                    >{accessTab === 'teams'
                      ? visibleAccessTeams.length + ' 个团队'
                      : visibleAccessUsers.length + ' 个用户'}</span
                  >
                </div>
                <label class="access-search">
                  <Search size={15} aria-hidden="true" />
                  <span class="sr-only"
                    >搜索{accessTab === 'teams' ? '团队' : '用户'}</span
                  ><input
                    bind:value={accessSearch}
                    placeholder={accessTab === 'teams'
                      ? '搜索团队名称、编码或状态'
                      : '搜索姓名、用户名、邮箱或手机号'}
                  />
                </label>
                <div class="access-heading-actions">
                  {#if accessTab === 'teams'}
                    <button
                      class="secondary danger-action"
                      type="button"
                      disabled={selectedAccessTeamIds.length === 0 || busy}
                      data-tooltip={selectedAccessTeamIds.length === 0
                        ? '请先选择可管理的团队'
                        : '批量禁用所选团队'}
                      on:click={() =>
                        requestDisable('team', selectedAccessTeamIds)}
                      ><Trash2 size={15} aria-hidden="true" />批量删除</button
                    >
                    {#if accessCanCreateTeam}<button
                        class="primary"
                        type="button"
                        on:click={openTeamDialog}
                        ><Plus size={15} aria-hidden="true" />添加团队</button
                      >{/if}
                  {:else if accessTab === 'users'}
                    <button
                      class="secondary danger-action"
                      type="button"
                      disabled={selectedAccessUserIds.length === 0 || busy}
                      data-tooltip={selectedAccessUserIds.length === 0
                        ? '请先选择可管理的用户'
                        : '批量禁用所选用户'}
                      on:click={() =>
                        requestDisable('user', selectedAccessUserIds)}
                      ><Trash2 size={15} aria-hidden="true" />批量删除</button
                    >
                    {#if accessCanCreateUser}<button
                        class="primary"
                        type="button"
                        on:click={() => {
                          resetUserDialog();
                          userDialogOpen = true;
                        }}><Plus size={15} aria-hidden="true" />添加用户</button
                      >{/if}
                  {/if}
                </div>
              </div>
            {/if}
            {#if accessLoading}
              <div class="access-state" aria-live="polite">
                正在加载管理数据...
              </div>
            {:else if accessLoadError}
              <div class="access-state access-error">
                <span>{accessLoadError}</span><button
                  class="secondary"
                  type="button"
                  on:click={loadAccess}>重试</button
                >
              </div>
            {:else if accessTab === 'teams'}
              <div class="access-table access-team-table">
                <div class="access-table-header">
                  <input
                    type="checkbox"
                    aria-label="选择全部可管理团队"
                    checked={visibleAccessTeams.some(canManageTeam) &&
                      visibleAccessTeams
                        .filter(canManageTeam)
                        .every((team) =>
                          selectedAccessTeamIds.includes(team.id)
                        )}
                    on:change={(event) => {
                      selectedAccessTeamIds = event.currentTarget.checked
                        ? visibleAccessTeams
                            .filter(canManageTeam)
                            .map((team) => team.id)
                        : [];
                    }}
                  /><span>团队</span><span>成员</span><span>项目</span><span
                    >状态</span
                  ><span>操作</span>
                </div>
                {#each visibleAccessTeams as team}
                  {@const teamMembers = accessTeamUsers[team.id] ?? []}
                  {@const teamProjects = projects.filter(
                    (project) => project.team_id === team.id
                  )}
                  {@const TeamIcon = teamIconComponent(team.icon)}
                  <article class="access-record">
                    <div class="access-table-row">
                      <input
                        type="checkbox"
                        aria-label={`选择团队 ${team.name}`}
                        disabled={!canManageTeam(team) ||
                          team.status !== 'active'}
                        bind:group={selectedAccessTeamIds}
                        value={team.id}
                      />
                      <button
                        class="access-team-trigger"
                        type="button"
                        aria-expanded={teamAccessExpanded[team.id]}
                        on:click={() => toggleTeamAccess(team.id)}
                      >
                        <span class="entity-icon team-icon"
                          ><TeamIcon size={17} strokeWidth={1.8} /></span
                        ><span
                          ><strong>{team.name}</strong><small>{team.code}</small
                          ></span
                        ><ChevronDown
                          size={16}
                          class={teamAccessExpanded[team.id]
                            ? 'expanded'
                            : undefined}
                          aria-hidden="true"
                        />
                      </button>
                      <span class="access-metric"
                        ><strong>{teamMembers.length}</strong><small
                          >位可见成员</small
                        ></span
                      ><span class="access-metric"
                        ><strong>{teamProjects.length}</strong><small
                          >个关联项目</small
                        ></span
                      ><span class="status-label {team.status}"
                        >{team.status === 'active' ? '启用' : '已禁用'}</span
                      >
                      <div class="access-row-actions">
                        {#if canManageTeam(team)}
                          <button
                            class="icon-button"
                            type="button"
                            aria-label={`编辑团队 ${team.name}`}
                            data-tooltip="编辑团队"
                            on:click={() => openEditTeam(team)}
                            ><Pencil size={15} aria-hidden="true" /></button
                          ><button
                            class="icon-button danger-action"
                            type="button"
                            aria-label={`删除团队 ${team.name}`}
                            data-tooltip="禁用团队"
                            disabled={team.status !== 'active'}
                            on:click={() => requestDisable('team', [team.id])}
                            ><Trash2 size={15} aria-hidden="true" /></button
                          >
                        {:else}<span class="read-only-label">只读</span>{/if}
                      </div>
                    </div>
                    {#if teamAccessExpanded[team.id]}
                      <div class="team-directory-detail">
                        <div class="directory-subsection">
                          <span class="directory-label">成员</span>
                          {#each teamMembers as member}<button
                              class="directory-user"
                              type="button"
                              on:click={() => {
                                accessTab = 'users';
                                accessSearch = member.username;
                              }}
                              ><span class="avatar tiny-avatar"
                                >{(member.display_name || member.username)
                                  .slice(0, 1)
                                  .toUpperCase()}</span
                              ><span
                                ><strong
                                  >{member.display_name ||
                                    member.username}</strong
                                ><small
                                  >{userRoles(member.id)
                                    .map(roleLabel)
                                    .join(' · ') || '未分配角色'}</small
                                ></span
                              ></button
                            >{:else}<span class="directory-empty"
                              >当前账号看不到该团队的成员</span
                            >{/each}
                        </div>
                        <div class="directory-subsection">
                          <span class="directory-label">项目</span>
                          {#each teamProjects as project}<div
                              class="directory-project"
                            >
                              <span class="project-dot"></span><span
                                ><strong>{project.name}</strong><small
                                  >{project.code} · {project.status}</small
                                ></span
                              >
                            </div>{:else}<span class="directory-empty"
                              >暂无项目</span
                            >{/each}
                        </div>
                      </div>
                    {/if}
                  </article>
                {:else}<div class="access-state">
                    没有匹配的团队。请清除搜索条件后重试。
                  </div>{/each}
              </div>
            {:else if accessTab === 'users'}
              <div class="access-table access-user-table">
                <div class="access-table-header">
                  <input
                    type="checkbox"
                    aria-label="选择全部可管理用户"
                    checked={visibleAccessUsers.some(canManageUser) &&
                      visibleAccessUsers
                        .filter(canManageUser)
                        .every((user) =>
                          selectedAccessUserIds.includes(user.id)
                        )}
                    on:change={(event) => {
                      selectedAccessUserIds = event.currentTarget.checked
                        ? visibleAccessUsers
                            .filter(canManageUser)
                            .map((user) => user.id)
                        : [];
                    }}
                  /><span>用户</span><span>授权范围</span><span>角色与权限</span
                  ><span>状态</span><span>操作</span>
                </div>
                {#each visibleAccessUsers as user}
                  <article class="access-table-row access-user-row">
                    <input
                      type="checkbox"
                      aria-label={`选择用户 ${user.display_name || user.username}`}
                      disabled={!canManageUser(user) ||
                        user.status !== 'active'}
                      bind:group={selectedAccessUserIds}
                      value={user.id}
                    />
                    <div class="access-user-main">
                      <span class="avatar access-avatar"
                        >{(user.display_name || user.username)
                          .slice(0, 1)
                          .toUpperCase()}</span
                      ><span
                        ><strong>{user.display_name || user.username}</strong
                        ><small
                          >@{user.username}{user.email
                            ? ` · ${user.email}`
                            : ''}</small
                        ></span
                      >
                    </div>
                    <div class="access-user-scopes">
                      {#each userScopeNames(user.id).slice(0, 2) as scope}<span
                          >{scope}</span
                        >{:else}<span class="permission-empty"
                          >无可见 Scope</span
                        >{/each}
                    </div>
                    <div class="access-user-auth">
                      <div class="access-user-roles">
                        {#each userRoles(user.id) as role}<span
                            class="role-chip">{roleLabel(role)}</span
                          >{:else}<span class="role-chip muted-chip"
                            >未分配角色</span
                          >{/each}
                      </div>
                      <div class="access-user-permissions">
                        {#each userPermissions(user.id).slice(0, 3) as permission}<span
                            data-tooltip={permissionDescription(permission)}
                            title={permissionDescription(permission)}
                            >{permission}</span
                          >{:else}<span class="permission-empty">暂无权限</span
                          >{/each}{#if userPermissions(user.id).length > 3}<span
                            >+{userPermissions(user.id).length - 3}</span
                          >{/if}
                      </div>
                    </div>
                    <span class="status-label {user.status}"
                      >{user.status === 'active'
                        ? '启用'
                        : user.status === 'locked'
                          ? '已锁定'
                          : '已禁用'}</span
                    >
                    <div class="access-row-actions">
                      {#if canManageUser(user)}
                        <button
                          class="icon-button"
                          type="button"
                          aria-label={`编辑用户 ${user.display_name || user.username}`}
                          data-tooltip="编辑用户与授权"
                          on:click={() => openEditUser(user)}
                          ><Pencil size={15} aria-hidden="true" /></button
                        ><button
                          class="icon-button danger-action"
                          type="button"
                          aria-label={`删除用户 ${user.display_name || user.username}`}
                          data-tooltip="禁用用户"
                          disabled={user.status !== 'active'}
                          on:click={() => requestDisable('user', [user.id])}
                          ><Trash2 size={15} aria-hidden="true" /></button
                        >
                      {:else}<span class="read-only-label"
                          >{user.id === currentUser?.id
                            ? '当前账号'
                            : '只读'}</span
                        >{/if}
                    </div>
                  </article>
                {:else}<div class="access-state">
                    没有匹配的用户，或当前账号没有成员查看权限。
                  </div>{/each}
              </div>
            {:else}
              <div class="role-catalog-grid">
                <div class="role-catalog-toolbar">
                  <div>
                    <h2>角色权限</h2>
                    <p>
                      角色权限决定用户在对应 Scope
                      内可以执行的操作；仅管理员可为其他用户授权。
                    </p>
                  </div>
                  <span class="access-role-boundary"
                    ><ShieldCheck
                      size={16}
                      aria-hidden="true"
                    />授权时只显示当前账号可完整授予的角色</span
                  >
                </div>
                {#each roles as role}<article class="role-catalog-item">
                    <div>
                      <strong>{roleLabel(role.name)}</strong><small
                        >{roleScopeLabel(role.scope_type)}{role.builtin
                          ? ' · 内置'
                          : ''}</small
                      >
                    </div>
                    <div class="permission-list">
                      {#each role.permissions as permission}<span
                          data-tooltip={permissionDescription(
                            String(permission)
                          )}
                          title={permissionDescription(String(permission))}
                          >{permission}</span
                        >{/each}
                    </div>
                  </article>{:else}<div class="access-state">
                    当前账号没有角色目录查看权限。
                  </div>{/each}
              </div>
            {/if}
          </section>
        </section>
        {#if teamDialogOpen}
          <div
            class="dialog-backdrop"
            role="presentation"
            on:click={(event) => {
              if (event.currentTarget === event.target) teamDialogOpen = false;
            }}
          >
            <dialog open class="dialog" aria-labelledby="team-dialog-title">
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">TEAM</p>
                  <h2 id="team-dialog-title">新增团队</h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
                <button
                  class="icon-button"
                  type="button"
                  aria-label="关闭"
                  on:click={() => (teamDialogOpen = false)}>×</button
                >
              </div>
              <form class="stack-form" on:submit|preventDefault={createTeam}>
                <div class="team-identity-field">
                  <button
                    class="team-icon-picker-trigger"
                    type="button"
                    aria-label="选择团队图标"
                    data-tooltip="选择团队图标"
                    on:click={() => openTeamIconPicker('create')}
                    ><span class="entity-icon team-icon"
                      ><svelte:component
                        this={teamIconComponent(teamIcon)}
                        size={16}
                        strokeWidth={1.8}
                      /></span
                    ></button
                  ><label
                    >名称<input
                      bind:value={teamName}
                      required
                      maxlength="120"
                      placeholder="例如：支付平台"
                    /></label
                  >
                </div>
                <label
                  >团队编码<input
                    bind:value={teamCode}
                    required
                    placeholder="例如：payments"
                  /></label
                ><label
                  >图标<input bind:value={teamIcon} placeholder="team" /></label
                >
                <div class="form-actions">
                  <button
                    class="secondary"
                    type="button"
                    on:click={() => (teamDialogOpen = false)}>取消</button
                  ><button class="primary" disabled={busy}>创建团队</button>
                </div>
              </form>
            </dialog>
          </div>
        {/if}
        {#if userDialogOpen}
          <div
            class="dialog-backdrop"
            role="presentation"
            on:click={(event) => {
              if (event.currentTarget === event.target) userDialogOpen = false;
            }}
          >
            <dialog open class="dialog" aria-labelledby="user-dialog-title">
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">USER ACCESS</p>
                  <h2 id="user-dialog-title">新增用户</h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
                <button
                  class="icon-button"
                  type="button"
                  aria-label="关闭"
                  on:click={() => (userDialogOpen = false)}>×</button
                >
              </div>
              <form class="stack-form" on:submit|preventDefault={createUser}>
                <div class="form-row">
                  <label
                    ><span
                      >用户名<span class="required-mark" aria-hidden="true"
                        >*</span
                      ></span
                    ><input
                      value={newUserUsername}
                      on:input={(event) =>
                        updateNewUserUsername(event.currentTarget.value)}
                      required
                      placeholder="登录用户名"
                    /></label
                  ><label
                    >显示名<input
                      bind:value={newUserDisplayName}
                      placeholder="默认使用用户名"
                    /></label
                  >
                </div>
                <div class="form-row">
                  <label
                    >邮箱<input
                      type="email"
                      bind:value={newUserEmail}
                      placeholder="name@example.com"
                    /></label
                  ><label
                    >手机号<input
                      bind:value={newUserPhone}
                      placeholder="+86"
                    /></label
                  >
                </div>
                <fieldset class="preference-group">
                  <legend>一次性密码</legend>
                  <div
                    class="segmented-control"
                    role="radiogroup"
                    aria-label="一次性密码方式"
                  >
                    <button
                      type="button"
                      class:active={newUserPasswordMode === 'generated'}
                      on:click={() => (newUserPasswordMode = 'generated')}
                      >自动生成</button
                    >
                    <button
                      type="button"
                      class:active={newUserPasswordMode === 'manual'}
                      on:click={() => (newUserPasswordMode = 'manual')}
                      >手动设置</button
                    >
                  </div>
                  {#if newUserPasswordMode === 'manual'}
                    <label
                      >一次性密码<input
                        type="password"
                        bind:value={newUserPassword}
                        required
                        minlength="8"
                        autocomplete="new-password"
                        placeholder="至少 8 位"
                      /></label
                    >
                  {:else}
                    <p class="form-help">
                      创建后显示一次性密码，仅可查看和复制一次。
                    </p>
                  {/if}
                </fieldset>
                <section class="new-user-grants" aria-label="用户授权">
                  <div class="new-user-grants-heading">
                    <div>
                      <strong>授权配置</strong>
                    </div>
                    <button
                      class="secondary"
                      type="button"
                      on:click={addNewUserGrant}
                      disabled={busy || manageableScopeChoices.length === 0}
                      ><Plus size={15} aria-hidden="true" />添加授权</button
                    >
                  </div>
                  <div class="new-user-grant-header" aria-hidden="true">
                    <span>级别</span><span>对象</span><span>角色</span><span
                      >操作</span
                    >
                  </div>
                  {#each newUserGrants as grant, grantIndex}
                    <section class="new-user-grant-row">
                      <div class="new-user-grant-fields">
                        <label
                          ><span class="sr-only">授权级别</span><select
                            value={grant.scopeType}
                            on:change={(event) =>
                              chooseNewUserGrantType(
                                grantIndex,
                                event.currentTarget
                                  .value as NewUserGrant['scopeType']
                              )}
                            >{#each ['platform', 'team', 'project'] as type}
                              {#if newUserGrantScopes(type as NewUserGrant['scopeType']).length > 0}
                                <option value={type}
                                  >{grantScopeLabel(
                                    type as NewUserGrant['scopeType']
                                  )}</option
                                >
                              {/if}
                            {/each}</select
                          ></label
                        >
                        <label
                          ><span class="sr-only">授权对象</span>
                          {#if grant.scopeType === 'platform'}
                            <span class="new-user-no-object">无需选择</span>
                          {:else}
                            <select
                              value={grant.scopeID}
                              on:change={(event) =>
                                updateNewUserGrant(grantIndex, {
                                  scopeID: event.currentTarget.value,
                                  roleID: '',
                                  resourceGrants: []
                                })}
                              ><option value=""
                                >选择{grant.scopeType === 'team'
                                  ? '团队'
                                  : '项目'}</option
                              >{#each newUserGrantScopes(grant.scopeType) as scope}
                                <option value={scope.id}>{scope.name}</option>
                              {/each}</select
                            >
                          {/if}
                        </label>
                        <label
                          ><span class="sr-only">角色</span><select
                            value={grant.roleID}
                            disabled={!grant.scopeID}
                            on:change={(event) =>
                              updateNewUserGrant(grantIndex, {
                                roleID: event.currentTarget.value,
                                resourceGrants: []
                              })}
                            ><option value="">选择角色</option
                            >{#each newUserGrantRoles(grant) as role}
                              <option value={role.id}
                                >{grantRoleLabel(role.name)}</option
                              >
                            {/each}</select
                          ></label
                        >
                        <button
                          class="icon-button danger-action"
                          type="button"
                          data-tooltip="移除此授权"
                          aria-label="移除此授权"
                          disabled={busy || newUserGrants.length === 1}
                          on:click={() => removeNewUserGrant(grantIndex)}
                          ><Trash2 size={15} aria-hidden="true" /></button
                        >
                      </div>
                      {#if newUserGrantIsScopeViewer(grant)}
                        <div class="new-user-resource-grants">
                          <div>
                            <strong>范围资源权限</strong>
                            <small
                              >{grantScopeLabel(
                                grant.scopeType
                              )}观察员默认可读取该范围资源；可为指定资源追加操作或管理权限。</small
                            >
                          </div>
                          {#each grant.resourceGrants as resourceGrant, resourceIndex}
                            <div class="new-user-resource-grant-row">
                              <label
                                >资源<select
                                  value={resourceGrant.resourceID}
                                  on:change={(event) =>
                                    updateNewUserResourceGrant(
                                      grantIndex,
                                      resourceIndex,
                                      { resourceID: event.currentTarget.value }
                                    )}
                                  ><option value="">选择范围内资源</option
                                  >{#each newUserGrantResources(grant) as resource}
                                    <option value={resource.id}
                                      >{resource.name} · {resource.kind}</option
                                    >
                                  {/each}</select
                                ></label
                              >
                              <label
                                >资源权限<select
                                  value={resourceGrant.roleID}
                                  on:change={(event) =>
                                    updateNewUserResourceGrant(
                                      grantIndex,
                                      resourceIndex,
                                      { roleID: event.currentTarget.value }
                                    )}
                                  ><option value="">选择资源权限</option
                                  >{#each newUserGrantResourceRoles(grant) as resourceRole}
                                    <option value={resourceRole.id}
                                      >{roleLabel(resourceRole.name)}</option
                                    >
                                  {/each}</select
                                ></label
                              >
                              <button
                                class="icon-button danger-action"
                                type="button"
                                data-tooltip="移除资源权限"
                                aria-label="移除资源权限"
                                on:click={() =>
                                  removeNewUserResourceGrant(
                                    grantIndex,
                                    resourceIndex
                                  )}
                                ><Trash2 size={14} aria-hidden="true" /></button
                              >
                            </div>
                          {/each}
                          <button
                            class="secondary"
                            type="button"
                            on:click={() => addNewUserResourceGrant(grantIndex)}
                            disabled={busy}
                            ><Plus
                              size={14}
                              aria-hidden="true"
                            />添加资源权限</button
                          >
                        </div>
                      {/if}
                    </section>
                  {:else}
                    <p class="form-help">当前账号没有可授权的范围。</p>
                  {/each}
                </section>
                <div class="form-actions">
                  {#if createdUserCredentials}
                    <div class="created-credentials-inline" aria-live="polite">
                      <span
                        >一次性密码：<strong
                          >{createdUserCredentials.password}</strong
                        ></span
                      >
                      <button
                        class="icon-button"
                        type="button"
                        aria-label="复制一次性密码"
                        data-tooltip="复制一次性密码"
                        on:click={copyOneTimePassword}
                        >{#if copiedControl === 'created-password'}<ClipboardCheck
                            size={15}
                            aria-hidden="true"
                          />{:else}<Copy
                            size={15}
                            aria-hidden="true"
                          />{/if}</button
                      >
                    </div>
                  {/if}
                  <button
                    class="secondary"
                    type="button"
                    on:click={() => (userDialogOpen = false)}>取消</button
                  ><button
                    class="primary"
                    disabled={busy ||
                      newUserGrants.length === 0 ||
                      newUserGrants.some(
                        (grant) =>
                          !grant.scopeID ||
                          !grant.roleID ||
                          grant.resourceGrants.some(
                            (resourceGrant) =>
                              !resourceGrant.resourceID || !resourceGrant.roleID
                          )
                      )}>创建用户并授权</button
                  >
                </div>
              </form>
            </dialog>
          </div>
        {/if}
        {#if editingTeam}
          <div
            class="dialog-backdrop"
            role="presentation"
            on:click={(event) => {
              if (event.currentTarget === event.target) editingTeam = null;
            }}
          >
            <dialog
              open
              class="dialog"
              aria-labelledby="edit-team-dialog-title"
            >
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">TEAM</p>
                  <h2 id="edit-team-dialog-title">编辑团队</h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
                <button
                  class="icon-button"
                  type="button"
                  aria-label="关闭"
                  on:click={() => (editingTeam = null)}>×</button
                >
              </div>
              <form class="stack-form" on:submit|preventDefault={saveTeam}>
                <div class="team-identity-field">
                  <button
                    class="team-icon-picker-trigger"
                    type="button"
                    aria-label="选择团队图标"
                    data-tooltip="选择团队图标"
                    on:click={() => openTeamIconPicker('edit')}
                    ><span class="entity-icon team-icon"
                      ><svelte:component
                        this={teamIconComponent(editTeamIcon)}
                        size={16}
                        strokeWidth={1.8}
                      /></span
                    ></button
                  ><label
                    >名称<input
                      bind:value={editTeamName}
                      required
                      maxlength="120"
                      placeholder="例如：支付平台"
                    /></label
                  >
                </div>
                <label
                  >状态<select bind:value={editTeamStatus}
                    ><option value="active">启用</option><option
                      value="disabled">禁用</option
                    ></select
                  ></label
                >
                <div class="form-actions">
                  <button
                    class="secondary"
                    type="button"
                    on:click={() => (editingTeam = null)}>取消</button
                  ><button class="primary" disabled={busy}>保存团队</button>
                </div>
              </form>
            </dialog>
          </div>
        {/if}
        {#if iconPickerTarget}
          <div
            class="dialog-backdrop"
            role="presentation"
            on:click={(event) => {
              if (event.currentTarget === event.target) iconPickerTarget = null;
            }}
          >
            <dialog
              open
              class="dialog icon-picker-dialog"
              aria-labelledby="icon-picker-title"
            >
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">ICON PICKER</p>
                  <h2 id="icon-picker-title">选择图标</h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
                <button
                  class="icon-button"
                  type="button"
                  aria-label="关闭"
                  on:click={() => (iconPickerTarget = null)}>×</button
                >
              </div>
              <div class="icon-picker-body">
                <label class="icon-search"
                  ><Search size={16} aria-hidden="true" /><span class="sr-only"
                    >搜索图标</span
                  ><input
                    bind:value={teamIconSearch}
                    placeholder="搜索图标，如 Kubernetes、数据库"
                    aria-label="搜索图标"
                  /></label
                >
                <div class="team-icon-grid" aria-label="团队图标列表">
                  {#each filteredTeamIconOptions as option}
                    {@const TeamIcon = teamIconComponent(option.value)}
                    <button
                      class:active={(iconPickerTarget === 'create'
                        ? teamIcon
                        : editTeamIcon) === option.value}
                      type="button"
                      on:click={() => selectTeamIcon(option.value)}
                      aria-label={`选择图标 ${option.label}`}
                      ><span class="entity-icon team-icon"
                        ><TeamIcon size={18} strokeWidth={1.8} /></span
                      ><span>{option.label}</span></button
                    >
                  {:else}
                    <p class="icon-picker-empty">没有匹配的图标。</p>
                  {/each}
                </div>
              </div>
            </dialog>
          </div>
        {/if}
        {#if editingUser}
          <div
            class="dialog-backdrop"
            role="presentation"
            on:click={(event) => {
              if (event.currentTarget === event.target) editingUser = null;
            }}
          >
            <dialog
              open
              class="dialog wide-dialog"
              aria-labelledby="edit-user-dialog-title"
            >
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">USER ACCESS</p>
                  <h2 id="edit-user-dialog-title">编辑用户与授权</h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
                <button
                  class="icon-button"
                  type="button"
                  aria-label="关闭"
                  on:click={() => (editingUser = null)}>×</button
                >
              </div>
              <form class="stack-form" on:submit|preventDefault={saveUser}>
                <div class="form-row">
                  <label
                    >用户名<input
                      value={editingUser.username}
                      disabled
                      aria-label="用户名不可修改"
                    /></label
                  >
                  <label
                    >显示名<input
                      bind:value={editUserDisplayName}
                      required
                      maxlength="120"
                      placeholder="默认使用用户名"
                    /></label
                  >
                </div>
                <label
                  >授权 Scope<select
                    value={editUserScopeId}
                    on:change={(event) =>
                      chooseEditUserScope(event.currentTarget.value)}
                    >{#each manageableScopeChoices as scope}<option
                        value={scope.id}>{scope.name} · {scope.type}</option
                      >{/each}</select
                  ></label
                >
                <fieldset class="role-picker" disabled={!editUserScopeId}>
                  <legend>直接授权角色</legend>
                  {#each availableEditUserRoles as role}<label class="check-row"
                      ><input
                        type="checkbox"
                        bind:group={editUserRoleIds}
                        value={role.id}
                      /><span
                        ><strong>{roleLabel(role.name)}</strong><small
                          >{role.permissions.length} 项权限</small
                        ></span
                      ></label
                    >{:else}<p class="muted">
                      当前账号在该 Scope 没有可授予角色。
                    </p>{/each}
                </fieldset>
                <p class="form-help">
                  成员组继承的角色保持不变；这里只调整所选 Scope 下的直接角色。
                </p>
                {#if editingScopeViewer}
                  <section class="scope-viewer-resource-access">
                    <div class="scope-viewer-resource-heading">
                      <div>
                        <strong>范围资源权限</strong>
                        <p>
                          {grantScopeLabel(
                            scopeType(
                              editUserScopeId
                            ) as NewUserGrant['scopeType']
                          )}观察员默认可读取该范围资源；可为指定资源追加操作或管理权限。
                        </p>
                      </div>
                      <ShieldCheck size={17} aria-hidden="true" />
                    </div>
                    <div class="form-row">
                      <label
                        >资源角色<select bind:value={editUserResourceRoleId}>
                          <option value="">选择资源角色</option>
                          {#each availableScopeViewerResourceRoles as resourceRole}
                            <option value={resourceRole.id}
                              >{roleLabel(resourceRole.name)}</option
                            >
                          {/each}
                        </select></label
                      >
                      <label
                        >具体资源<select bind:value={editUserResourceId}>
                          <option value="">选择范围内资源</option>
                          {#each scopeViewerResources as resource}
                            <option value={resource.id}
                              >{resource.name} · {resource.kind}</option
                            >
                          {/each}
                        </select></label
                      >
                    </div>
                    <button
                      class="secondary"
                      type="button"
                      disabled={busy ||
                        !editUserResourceRoleId ||
                        !editUserResourceId}
                      on:click={grantScopeViewerResource}
                    >
                      <Plus size={15} aria-hidden="true" />添加资源权限
                    </button>
                    <div class="scope-viewer-resource-list">
                      {#each scopeViewerResourceBindings as resourceBinding}
                        <div class="scope-viewer-resource-item">
                          <span
                            ><strong>{resourceBinding.resource_name}</strong
                            ><small
                              >{roleLabel(resourceBinding.role_name)}</small
                            ></span
                          >
                          <button
                            class="icon-button danger-action"
                            type="button"
                            aria-label="移除资源权限"
                            data-tooltip="移除资源权限"
                            on:click={() =>
                              revokeScopeViewerResource(resourceBinding)}
                          >
                            <Trash2 size={14} aria-hidden="true" />
                          </button>
                        </div>
                      {:else}
                        <span class="permission-empty"
                          >尚未添加具体资源权限</span
                        >
                      {/each}
                    </div>
                  </section>
                {/if}
                {#if passwordResetCredentials}
                  <section class="role-preview" aria-live="polite">
                    <strong>一次性密码已生成</strong>
                    <label
                      >用户名<input
                        value={passwordResetCredentials.username}
                        readonly
                      /></label
                    >
                    <label
                      >一次性密码<input
                        value={passwordResetCredentials.password}
                        readonly
                      /></label
                    >
                    <div class="form-actions">
                      <button
                        class="secondary"
                        type="button"
                        on:click={() => copyPasswordResetCredentials(false)}
                        >{#if copiedControl === 'reset-username'}<ClipboardCheck
                            size={15}
                            aria-hidden="true"
                          />已复制{:else}<Copy
                            size={15}
                            aria-hidden="true"
                          />复制用户名{/if}</button
                      >
                      <button
                        class="primary"
                        type="button"
                        on:click={() => copyPasswordResetCredentials(true)}
                        >{#if copiedControl === 'reset-credentials'}<ClipboardCheck
                            size={15}
                            aria-hidden="true"
                          />已复制{:else}<Copy
                            size={15}
                            aria-hidden="true"
                          />复制用户名和密码{/if}</button
                      >
                    </div>
                  </section>
                {/if}
                <div class="form-actions">
                  <button
                    class="secondary"
                    type="button"
                    disabled={busy}
                    on:click={resetManagedUserPassword}>重置密码</button
                  >
                  <button
                    class="secondary"
                    type="button"
                    on:click={() => (editingUser = null)}>取消</button
                  ><button class="primary" disabled={busy || !editUserScopeId}
                    >保存授权</button
                  >
                </div>
              </form>
            </dialog>
          </div>
        {/if}
        {#if disableTarget}
          <div class="dialog-backdrop" role="presentation">
            <dialog
              open
              class="dialog confirm-dialog"
              aria-labelledby="disable-dialog-title"
            >
              <div class="dialog-heading">
                <div>
                  <p class="eyebrow">CONFIRM ACTION</p>
                  <h2 id="disable-dialog-title">
                    删除{disableTarget.ids.length} 个{disableTarget.kind ===
                    'team'
                      ? '团队'
                      : '用户'}？
                  </h2>
                </div>
                {#if activeMessage}<MessageBanner message={activeMessage} tone={activeMessageTone} />{/if}
              </div>
              <p class="confirm-copy">
                {disableTarget.kind === 'team'
                  ? '团队将被禁用，其项目与历史数据会保留。禁用后团队不可继续用于新操作。'
                  : '用户将被禁用并无法继续登录，现有角色绑定与审计记录会保留。'}
              </p>
              <div class="form-actions">
                <button
                  class="secondary"
                  type="button"
                  disabled={busy}
                  on:click={() => (disableTarget = null)}>取消</button
                ><button
                  class="danger-button"
                  type="button"
                  disabled={busy}
                  on:click={confirmDisable}
                  >{busy ? '正在处理' : '确认删除'}</button
                >
              </div>
            </dialog>
          </div>
        {/if}
      {/if}
    </main>
  </div>
{/if}
