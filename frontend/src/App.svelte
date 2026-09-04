<script lang="ts">
import { onMount, tick } from 'svelte';
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
    LogOut,
    PanelLeftClose,
    PanelLeftOpen,
    Pencil,
    PlugZap,
    Plus,
    RefreshCw,
    ScanSearch,
    Search,
    ShieldCheck,
    Sparkles,
    Stethoscope,
    Trash2,
    UserRound,
    UsersRound,
    icons as lucideIcons
  } from 'lucide-svelte';
  import { fetchHealth, toStatusRows, type HealthReport } from './lib/health';
  import { renderDiagnosisMarkdown as renderDiagnosisMarkdownShared } from './lib/diagnosisMarkdown';
  import BrandIcon from './lib/BrandIcon.svelte';
  import MessageBanner from './components/MessageBanner.svelte';
  import DiagnosisModelPicker from './features/diagnosis/DiagnosisModelPicker.svelte';
  import DiagnosisComposer from './features/diagnosis/DiagnosisComposer.svelte';
  import DiagnosisContextPanel from './features/diagnosis/DiagnosisContextPanel.svelte';
  import DiagnosisSessionList from './features/diagnosis/DiagnosisSessionList.svelte';
  import DiagnosisConversationHeader from './features/diagnosis/DiagnosisConversationHeader.svelte';
  import DiagnosisConversationMessages from './features/diagnosis/DiagnosisConversationMessages.svelte';
  import AgentProfilesPage from './features/agent/AgentProfilesPage.svelte';
  import SkillRegistryPage from './features/skill/SkillRegistryPage.svelte';
  import ProfilePage from './features/profile/ProfilePage.svelte';
  import OperationsPage from './features/operations/OperationsPage.svelte';
  import AuthGate from './features/auth/AuthGate.svelte';
  import DiscoveryPage from './features/discovery/DiscoveryPage.svelte';
  import InspectionPage from './features/inspection/InspectionPage.svelte';
  import OverviewPage from './features/overview/OverviewPage.svelte';
  import OrganizationPage from './features/organization/OrganizationPage.svelte';
  import ResourceCatalogRail from './features/resources/ResourceCatalogRail.svelte';
  import ResourceCatalogToolbar from './features/resources/ResourceCatalogToolbar.svelte';
  import ResourceCatalogList from './features/resources/ResourceCatalogList.svelte';
  import ResourceAddStepNavigation from './features/resources/ResourceAddStepNavigation.svelte';
  import ResourceAddWorkflowHeader from './features/resources/ResourceAddWorkflowHeader.svelte';
  import ResourceBasicConfiguration from './features/resources/ResourceBasicConfiguration.svelte';
  import MCPResourceConfiguration from './features/resources/MCPResourceConfiguration.svelte';
  import MCPResourceSummary from './features/resources/MCPResourceSummary.svelte';
  import ProviderResourceConfiguration from './features/resources/ProviderResourceConfiguration.svelte';
  import ProviderModelConfiguration from './features/resources/ProviderModelConfiguration.svelte';
  import ProviderResourceSummary from './features/resources/ProviderResourceSummary.svelte';
  import Topbar from './layouts/Topbar.svelte';
  import AppShell from './layouts/AppShell.svelte';
  import {
    api,
    ApiError,
    type ConnectionCheck,
    type ConnectorCapability,
    type DiagnosisCausalChain,
    type DiagnosisEvidence,
    type DiagnosisEvent,
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
    { value: 'general', label: '通用', requiredCapabilities: ['text'] },
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
  let diagnosisModelMenuOpen = false;
  let diagnosisModelMenuProviderId = '';
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
  let diagnosisAnswerCompleted = false;
  let diagnosisLiveProcessExpanded = false;
  let diagnosisStreamingText = '';
  // Text emitted before a model turn reveals that it will call a tool is only
  // a candidate. It must not leak into the final assistant answer.
  let diagnosisStreamingTurnBase = '';
  let diagnosisStreamingStartedAt = 0;
  let diagnosisStreamingAssistantBaseline = 0;
  let diagnosisMessageListElement: HTMLDivElement | null = null;
  let diagnosisActionExpanded: Record<string, boolean> = {};
  let diagnosisProcessExpanded: Record<string, boolean> = {};
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
  let editingResourceId = '';
  let mcpTransport = 'streamable_http';
  let mcpURL = '';
  let mcpToken = '';
  let mcpRequestHeaders = '';
  let mcpToolAllowlist = '';
  let mcpTimeoutSeconds = 120;
  let mcpMaxResponseBytes = 4 * 1024 * 1024;
  let mcpDraftTest: { signature: string; result?: MCPSnapshot; error?: string } | null = null;
  let mcpDraftTestBusy = false;
  let mcpConfigurationAttempted = false;
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
  $: diagnosisSelectedProvider = diagnosisAvailableProviders.find(
    (item) => item.provider_resource_id === selectedProviderId
  );
  $: diagnosisModelMenuProvider = diagnosisAvailableProviders.find(
    (item) => item.provider_resource_id === (diagnosisModelMenuProviderId || selectedProviderId)
  ) ?? diagnosisSelectedProvider ?? diagnosisAvailableProviders[0];
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
    document.addEventListener('click', handleDiagnosisMarkdownClick);
    return () => {
      if (noticeTimer !== null) window.clearTimeout(noticeTimer);
      if (errorTimer !== null) window.clearTimeout(errorTimer);
      if (loginErrorTimer !== null) window.clearTimeout(loginErrorTimer);
      stopHealthPolling();
      closeDiagnosisEvents();
      media.removeEventListener('change', refreshTheme);
      document.removeEventListener('pointerdown', handleDocumentPointerDown);
      document.removeEventListener('click', handleDiagnosisMarkdownClick);
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
      diagnosisModelMenuOpen = false;
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
    if (diagnosisModelMenuOpen && !target.closest('.diagnosis-model-picker')) {
      diagnosisModelMenuOpen = false;
    }
  }

  function toggleDiagnosisModelMenu() {
    diagnosisModelMenuOpen = !diagnosisModelMenuOpen;
    if (diagnosisModelMenuOpen) {
      diagnosisModelMenuProviderId = selectedProviderId || diagnosisAvailableProviders[0]?.provider_resource_id || '';
    }
  }

  function chooseDiagnosisModelProvider(providerID: string) {
    const provider = diagnosisAvailableProviders.find(
      (item) => item.provider_resource_id === providerID
    );
    if (!provider) return;
    selectedProviderId = providerID;
    diagnosisModelMenuProviderId = providerID;
    if (!provider.models.some((model) => String(model.name ?? '') === llmModelName)) {
      llmModelName = String(provider.models[0]?.name ?? '');
    }
  }

  function chooseDiagnosisModel(modelName: string) {
    const provider = diagnosisModelMenuProvider;
    if (!provider) return;
    selectedProviderId = provider.provider_resource_id;
    llmModelName = modelName;
    diagnosisModelMenuOpen = false;
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
  async function loadMCPSnapshots(resourceId: string) {
    try {
      operationSnapshots = {
        ...operationSnapshots,
        [resourceId]: await api.mcpSnapshots(resourceId)
      };
    } catch (error) {
      errorMessage = describeError(error, 'MCP 工具快照加载失败');
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

  function resetDiagnosisStreamState() {
    diagnosisAnswerCompleted = false;
    diagnosisLiveProcessExpanded = false;
    diagnosisStreamingText = '';
    diagnosisStreamingTurnBase = '';
    diagnosisStreamingStartedAt = 0;
  }

  async function openDiagnosis(id: string) {
    closeDiagnosisEvents();
    selectedDiagnosisId = id;
    selectedEvidence = null;
    diagnosisEditingMessageId = '';
    diagnosisEditDraft = '';
    diagnosisInterruptedReason = '';
    resetDiagnosisStreamState();
    try {
      diagnosisSnapshot = await api.diagnosisSession(id);
      diagnosisSnapshot = {
        ...diagnosisSnapshot,
        messages: diagnosisSnapshot.messages.filter(
          (message) => !diagnosisHiddenMessageIds.includes(message.id)
        )
      };
      diagnosisStreamingAssistantBaseline = diagnosisSnapshot.messages.filter(
        (message) => message.role === 'assistant'
      ).length;
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
      diagnosisEventCursor = Math.max(
        0,
        ...(diagnosisSnapshot.events ?? []).map((event) => Number(event.id) || 0)
      );
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
      'model.started',
      'model.resumed',
      'assistant.progress',
      'assistant.delta',
      'assistant.completed',
      'tool.requested',
      'tool.started',
      'tool.completed',
      'tool.failed',
      'evidence.collected',
      'report.ready',
      'diagnosis.failed',
      'diagnosis.cancelled',
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
    if (!appendDiagnosisEvent(eventType, payload, diagnosisEventCursor)) return;
    if (eventType === 'model.started') {
      diagnosisStreamingTurnBase = diagnosisStreamingText;
      // Keep the current live node mounted across model-turn boundaries. A
      // tool-decision turn is removed from the answer only when the tool call
      // is actually observed; clearing here would hide already received text
      // while the next model request is still pending.
      diagnosisStreamingStartedAt ||= Date.now();
      diagnosisGenerating = true;
      return;
    }
    if (eventType === 'assistant.delta') {
      const text = String(payload.text ?? '');
      if (text) {
        // assistant.delta is an append-only UTF-8 text fragment. Keep it byte
        // identical to the durable assistant message; the renderer alone
        // decides whether the currently complete syntax is Markdown.
        diagnosisStreamingText += text;
        diagnosisStreamingStartedAt ||= Date.now();
      }
      diagnosisGenerating = true;
      return;
    }
    if (eventType === 'assistant.progress') {
      // Tool decisions are timeline metadata, not assistant answer text.
      if (String(payload.kind ?? '') === 'tool_decision') {
        // Partial model text may have been emitted before the function call
        // became visible. Roll it back now that the turn is known to be a
        // tool-decision turn; the text remains available in the timeline.
        diagnosisStreamingText = diagnosisStreamingTurnBase;
      }
      diagnosisGenerating = true;
      diagnosisStreamingStartedAt ||= Date.now();
      return;
    }
    if (eventType === 'model.resumed') {
      // The preceding model turn produced tool observations. Its candidate
      // text belongs to the execution timeline, not to the final answer.
      diagnosisStreamingText = '';
      diagnosisStreamingTurnBase = '';
      diagnosisGenerating = true;
      return;
    }
    if (eventType === 'assistant.completed') {
      const finalText = String(payload.text ?? '');
      if (finalText) {
        // A completion event closes the stream; it is not a second rendering
        // pass. Only adapters that produced no deltas need their final text to
        // seed the live node; streamed text remains byte-for-byte unchanged.
        if (!diagnosisStreamingText) diagnosisStreamingText = finalText;
      }
      // This is the only AIEngine event that confirms the final answer is
      // complete. Earlier deltas, model turns and tool events never infer it.
      diagnosisAnswerCompleted = true;
      diagnosisLiveProcessExpanded = false;
      diagnosisGenerating = true;
      void refreshDiagnosis(id);
      return;
    }
    if (eventType === 'execution.started' || eventType === 'tool.requested' || eventType === 'tool.started' || eventType === 'tool.completed' || eventType === 'tool.failed' || eventType === 'phase.changed') {
      if (eventType === 'tool.requested') {
        // Move any candidate sentence already received for this model turn to
        // the process timeline as soon as the action becomes real. This keeps
        // the answer text free of tool-planning prose without waiting for the
        // tool result or a model.resumed event.
        diagnosisStreamingText = diagnosisStreamingTurnBase;
      }
      diagnosisGenerating = true;
      diagnosisStreamingStartedAt ||= Date.now();
      return;
    }
    if (eventType === 'execution.failed' || eventType === 'execution.cancelled' || eventType === 'diagnosis.failed' || eventType === 'diagnosis.cancelled') {
      const reason = String(payload.error ?? payload.message ?? payload.error_message ?? '').trim();
      const timedOut = /(?:context\s+)?deadline exceeded/i.test(reason) || String(payload.error_code ?? payload.code ?? '').toLowerCase() === 'timeout';
      const tokenBudget = String(payload.error_code ?? payload.code ?? '').toLowerCase() === 'token_budget' || /token budget exceeded/i.test(reason);
      diagnosisInterruptedReason = timedOut
        ? '诊断执行超时，已保留已生成内容。'
        : tokenBudget
          ? reason || '模型累计上下文预算已用尽，已保留已生成内容。'
        : reason || (eventType === 'execution.cancelled' || eventType === 'diagnosis.cancelled' ? '回答被取消。' : '回答生成失败。');
      diagnosisGenerating = false;
      diagnosisAnswerCompleted = false;
      void refreshDiagnosis(id);
      return;
    }
    if (eventType === 'execution.completed') {
      // AIEngine completes before the diagnosis orchestrator persists the
      // assistant message, report and terminal session status. Keep the live
      // stream open so those post-processing events cannot race the UI.
      void refreshDiagnosis(id);
      return;
    }
    if (eventType === 'report.ready') {
      diagnosisLiveProcessExpanded = false;
      diagnosisAnswerCompleted = true;
      void refreshDiagnosis(id).finally(() => {
        diagnosisGenerating = false;
        diagnosisStreamingStartedAt = 0;
      });
      return;
    }
    // Evidence and phase events are already merged into the live snapshot by
    // appendDiagnosisEvent. Avoid replacing the streaming DOM with a slower
    // snapshot read for every event; refresh only at explicit terminal
    // boundaries above.
  }

  function appendDiagnosisEvent(type: string, payload: Record<string, unknown>, id: number) {
    if (!diagnosisSnapshot || !id) return false;
    const current = diagnosisSnapshot.events ?? [];
    if (current.some((item) => item.id === id)) return false;
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
    return true;
  }

  function closeDiagnosisEvents() {
    diagnosisEvents?.close();
    diagnosisEvents = null;
  }

  async function refreshDiagnosis(id = selectedDiagnosisId) {
    if (!id || id !== selectedDiagnosisId) return;
    const refreshToken = ++diagnosisRefreshToken;
    try {
      const snapshot = await api.diagnosisSession(id);
      if (refreshToken !== diagnosisRefreshToken || id !== selectedDiagnosisId) return;
      const localEvents = diagnosisSnapshot?.session.id === id
        ? diagnosisSnapshot.events ?? []
        : [];
      const mergedByID = new Map<number, NonNullable<DiagnosisSnapshot['events']>[number]>();
      for (const event of snapshot.events ?? []) mergedByID.set(event.id, event);
      // SSE events can arrive before the database-backed snapshot catches up.
      // Keep the locally received payload when both sides have the same ID.
      for (const event of localEvents) mergedByID.set(event.id, event);
      const mergedEvents = [...mergedByID.values()].sort((left, right) => left.id - right.id);
      diagnosisEventCursor = Math.max(
        diagnosisEventCursor,
        ...mergedEvents.map((event) => Number(event.id) || 0)
      );
      diagnosisSnapshot = {
        ...snapshot,
        events: mergedEvents,
        messages: snapshot.messages.filter(
          (message) => !diagnosisHiddenMessageIds.includes(message.id)
        )
      };
      const assistantMessageCount = diagnosisSnapshot.messages.filter(
        (message) => message.role === 'assistant'
      ).length;
      if (assistantMessageCount > diagnosisStreamingAssistantBaseline) {
        // Keep the live answer mounted for this turn. It is already the same
        // content that was persisted; clearing it here causes a second render
        // and a visible flash. A new turn explicitly resets the stream.
        diagnosisGenerating = diagnosisAnswerCompleted;
      } else {
        // A snapshot can race the SSE stream and still contain the previous
        // session status. Preserve local activity until a terminal snapshot or
        // a persisted assistant answer confirms that this run is over.
        diagnosisGenerating = !diagnosisInterruptedReason &&
          (diagnosisGenerating || isDiagnosisRunning(snapshot.session.status));
      }
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
    const question = diagnosisQuestion.trim();
    await action(async () => {
      diagnosisStreamingAssistantBaseline = diagnosisSnapshot
        ? diagnosisSnapshot.messages.filter((message) => message.role === 'assistant').length
        : 0;
      resetDiagnosisStreamState();
      const session = await api.startDiagnosis({
        scope_id: selectedScopeId,
        question,
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
      await scrollDiagnosisQuestionIntoView('', question);
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

  function writeDiagnosisClipboard(content: string, successMessage: string) {
    const clipboard = navigator.clipboard;
    if (!clipboard?.writeText) {
      notice = '当前浏览器不允许直接复制，请手动选择文本。';
      return;
    }
    void clipboard.writeText(content).then(
      () => (notice = successMessage),
      () => (notice = '当前浏览器不允许直接复制，请手动选择文本。')
    );
  }

  function diagnosisProcessText(snapshot: DiagnosisSnapshot | null) {
    if (!snapshot) return '';
    const lines: string[] = [];
    for (const item of diagnosisLiveTimeline(snapshot)) {
      if (item.kind === 'analysis') {
        if (item.text) lines.push(item.text);
        continue;
      }
      const actions = item.actions ?? [item];
      if (actions.length > 1) {
        lines.push(`调用工具 · ${actions.length} 个动作 · ${item.status ?? '执行中'}`);
      }
      for (const action of actions) {
        lines.push(`${diagnosisActionLabel(action)} · ${action.status ?? '执行中'} · ${action.duration ?? '—'}`);
        if (action.input) lines.push(`入参:\n${action.input}`);
        if (action.output) lines.push(`出参:\n${action.output}`);
      }
    }
    return lines.join('\n\n');
  }

  function copyDiagnosisMessage(message: DiagnosisMessage, processExpanded: boolean) {
    const answer = message.content ?? '';
    const process = processExpanded ? diagnosisProcessText(diagnosisSnapshot) : '';
    const content = process ? `${answer}\n\n执行过程\n\n${process}` : answer;
    writeDiagnosisClipboard(content, '回答已复制。');
  }

  function copyDiagnosisCode(encoded: string) {
    let content = '';
    try {
      content = decodeURIComponent(encoded);
    } catch {
      content = encoded;
    }
    writeDiagnosisClipboard(content, '代码已复制。');
  }

  function handleDiagnosisMarkdownClick(event: MouseEvent) {
    const target = event.target as HTMLElement | null;
    const button = target?.closest<HTMLButtonElement>('[data-code-copy]');
    if (button) {
      event.preventDefault();
      copyDiagnosisCode(button.dataset.codeCopy ?? '');
    }
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
      const question = diagnosisFollowup.trim();
      diagnosisStreamingAssistantBaseline = diagnosisSnapshot
        ? diagnosisSnapshot.messages.filter((message) => message.role === 'assistant').length
        : 0;
      resetDiagnosisStreamState();
      const created = await api.askDiagnosis(selectedDiagnosisId, question);
      diagnosisFollowup = '';
      diagnosisInterruptedReason = '';
      diagnosisGenerating = true;
      await refreshDiagnosis();
      openDiagnosisEvents(selectedDiagnosisId);
      await scrollDiagnosisQuestionIntoView(created.id, question);
    });
  }

  async function scrollDiagnosisQuestionIntoView(messageID = '', content = '') {
    await tick();
    const list = diagnosisMessageListElement;
    if (!list) return;
    const candidates = Array.from(
      list.querySelectorAll<HTMLElement>('[data-diagnosis-message-id]')
    );
    const target = [...candidates].reverse().find((item) => {
      if (messageID && item.dataset.diagnosisMessageId === messageID) return true;
      return Boolean(content && item.dataset.diagnosisMessageContent === content);
    });
    if (!target) return;
    const listRect = list.getBoundingClientRect();
    const targetRect = target.getBoundingClientRect();
    const topPadding = 16;
    const nextTop = list.scrollTop + targetRect.top - listRect.top - topPadding;
    list.scrollTo({ top: Math.max(0, nextTop), behavior: 'smooth' });
  }

  function isDiagnosisRunning(status: DiagnosisStatus | string) {
    return ['queued', 'planning', 'collecting', 'analyzing'].includes(status);
  }

  function newDiagnosisSession() {
    closeDiagnosisEvents();
    diagnosisModelMenuOpen = false;
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
    resetDiagnosisStreamState();
    diagnosisStreamingAssistantBaseline = 0;
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
      diagnosisStreamingAssistantBaseline = diagnosisSnapshot!.messages.filter(
        (message) => message.role === 'assistant'
      ).length;
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
      diagnosisInterruptedReason = '';
      diagnosisGenerating = true;
      openDiagnosisEvents(selectedDiagnosisId);
      await scrollDiagnosisQuestionIntoView(created.id, content);
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
      callID: string;
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
    const jsonText = (value: unknown, fallback: string) => {
      if (value === undefined) return fallback;
      if (typeof value === 'string') {
        try {
          return JSON.stringify(JSON.parse(value), null, 2);
        } catch {
          return JSON.stringify(value, null, 2);
        }
      }
      try {
        return JSON.stringify(value, null, 2) ?? fallback;
      } catch {
        return fallback;
      }
    };
    const groups: ToolGroup[] = [];
    let currentGroup: ToolGroup | null = null;

    for (const event of events) {
      // A new model turn or progress summary is a user-visible boundary:
      // tool calls on either side belong to different collapsed groups.
      if (event.type === 'model.started') {
        currentGroup = null;
        continue;
      }
      if (event.type === 'assistant.delta' || event.type === 'assistant.progress') {
        if (String(event.payload?.text ?? '').trim()) currentGroup = null;
        continue;
      }
      if (!event.type.startsWith('tool.')) continue;
      const payload = event.payload ?? {};
      const tool = String(payload.tool ?? '');
      const resourceID = String(payload.resource_id ?? '');
      const callID = String(payload.call_id ?? payload.call_sequence ?? '');
      // Evidence bookkeeping also emits tool.completed, but it is not an
      // AIEngine invocation and therefore must not appear in this trace. An
      // unknown-tool failure intentionally has an empty resource_id, so the
      // presence of the AIEngine lifecycle field is the discriminator.
      const isAIEngineToolEvent = Object.prototype.hasOwnProperty.call(payload, 'resource_id') || Boolean(callID);
      if (!tool || !isAIEngineToolEvent) continue;
      if (!currentGroup) {
        currentGroup = {
          id: `tool-group-${event.id}`,
          title: '调用工具',
          status: '进行中',
          duration: '—',
          children: []
        };
        groups.push(currentGroup);
      }
      let action = [...currentGroup.children]
        .reverse()
        .find((item) => item.tool === tool && item.resourceID === resourceID && item.status === '进行中' &&
          (callID ? item.callID === callID : !item.callID));
      if (!action) {
        action = {
          id: `tool-${event.id}`,
          icon: 'tool',
          title: titleForTool(tool),
          status: '进行中',
          duration: '—',
          input: jsonText(payload.arguments, JSON.stringify({ tool, resource_id: resourceID }, null, 2)),
          output: '等待工具执行结果…',
          created_at: event.created_at,
          updated_at: event.created_at,
          tool,
          resourceID,
          callID
        };
        currentGroup.children.push(action);
      }
      action.updated_at = event.created_at;
      if (Object.prototype.hasOwnProperty.call(payload, 'arguments')) {
        action.input = jsonText(payload.arguments, action.input);
      }
      if (event.type === 'tool.completed') {
        action.status = '已完成';
        const duration = Number(payload.duration_ms ?? 0);
        action.duration = duration > 0 ? `${Math.max(1, Math.round(duration / 1000))}s` : diagnosisDuration(action.created_at, event.created_at);
        action.output = jsonText(payload.output, JSON.stringify({ status: 'succeeded', duration_ms: payload.duration_ms ?? 0 }, null, 2));
      } else if (event.type === 'tool.failed') {
        action.status = '失败';
        const duration = Number(payload.duration_ms ?? 0);
        action.duration = duration > 0 ? `${Math.max(1, Math.round(duration / 1000))}s` : diagnosisDuration(action.created_at, event.created_at);
        action.output = jsonText(payload.output, JSON.stringify({ status: 'failed', error: payload.error ?? '工具调用失败' }, null, 2));
      }
      const completed = currentGroup.children.every((item) => item.status !== '进行中');
      currentGroup.status = completed ? (currentGroup.children.some((item) => item.status === '失败') ? '失败' : '已完成') : '进行中';
      currentGroup.duration = diagnosisDuration(currentGroup.children[0].created_at, action.updated_at);
    }
    return groups;
  }

  function diagnosisProcessActionCount(snapshot: DiagnosisSnapshot | null) {
    if (!snapshot) return 0;
    return (snapshot.events ?? []).filter(
      (event) => event.type === 'tool.completed' || event.type === 'tool.failed'
    ).length;
  }

  function diagnosisProcessDuration(snapshot: DiagnosisSnapshot | null) {
    if (!snapshot) return '—';
    const events = [...(snapshot.events ?? [])].sort((a, b) => a.id - b.id);
    const started = events.find((event) => event.type === 'execution.started');
    if (!started) return '—';
    const finished = [...events].reverse().find((event) =>
      event.type === 'execution.completed' ||
      event.type === 'execution.failed' ||
      event.type === 'execution.cancelled' ||
      event.type === 'report.ready'
    );
    return diagnosisDuration(started.created_at, finished?.created_at);
  }

  type DiagnosisEvidenceTimelineItem = {
    id: string;
    kind: 'turn' | 'analysis' | 'phase' | 'tool-group' | 'tool' | 'observation' | 'status';
    title: string;
    detail?: string;
    status?: string;
    tool?: string;
    input?: string;
    output?: string;
    duration?: string;
    children?: DiagnosisEvidenceTimelineItem[];
    resourceID?: string;
    evidenceIds?: string[];
  };

  function diagnosisEvidenceTimeline(snapshot: DiagnosisSnapshot | null): DiagnosisEvidenceTimelineItem[] {
    if (!snapshot) return [];
    const events = [...(snapshot.events ?? [])].sort((a, b) => a.id - b.id);
    const items: DiagnosisEvidenceTimelineItem[] = [];
    const tools = new Map<string, DiagnosisEvidenceTimelineItem>();
    let currentToolGroup: DiagnosisEvidenceTimelineItem | null = null;
    let currentTurn: DiagnosisEvidenceTimelineItem | null = null;
    let activeExecutionStarted = false;
    let userMessageCursor = 0;
    const userMessages = [...(snapshot.messages ?? [])].filter((message) => message.role === 'user');
    const jsonText = (value: unknown, fallback = '暂无数据') => {
      if (value === undefined || value === null) return fallback;
      if (typeof value === 'string') {
        try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; }
      }
      try { return JSON.stringify(value, null, 2) ?? fallback; } catch { return fallback; }
    };
    const toolName = (value: unknown) => String(value ?? '').trim() || '受控工具';
    const collectedEvidence = (snapshot.evidence ?? []).map((evidence) => ({
      id: evidence.id,
      sourceResourceID: evidence.source_resource_id ?? evidence.target_resource_id ?? '',
      capability: evidence.capability
    }));
    const compactQuestion = (value: string) => value.replace(/\s+/g, ' ').trim() || '本次诊断';
    const beginTurn = (event: DiagnosisEvent) => {
      if (currentTurn) return currentTurn;
      const message = userMessages[userMessageCursor++];
      currentTurn = {
        id: `chain-${event.id}`,
        kind: 'turn',
        title: compactQuestion(message?.content ?? '本次诊断'),
        detail: '从规划到结论的完整回答过程',
        children: []
      };
      items.push(currentTurn);
      currentToolGroup = null;
      tools.clear();
      return currentTurn;
    };
    const addChild = (item: DiagnosisEvidenceTimelineItem) => {
      beginTurn(events[0] ?? ({ id: 0 } as DiagnosisEvent));
      currentTurn?.children?.push(item);
    };
    for (const event of events) {
      const payload = event.payload ?? {};
      // Planning is the beginning of a response chain. This keeps the plan,
      // actions, observations and terminal state together in their original
      // event order, including events emitted before execution.started.
      if (event.type === 'phase.changed' && String(payload.phase ?? '').trim() === 'planning') {
        if (currentTurn && activeExecutionStarted) {
          currentTurn = null;
          activeExecutionStarted = false;
        }
        if (!currentTurn) beginTurn(event);
      } else if (event.type === 'plan.created') {
        if (!currentTurn) beginTurn(event);
      }
      if (event.type === 'execution.started') {
        // Every execution is a separate answer chain, including follow-up
        // questions in the same diagnosis session.
        const turnBeforeExecution = currentTurn as DiagnosisEvidenceTimelineItem | null;
        if (turnBeforeExecution && (turnBeforeExecution.children ?? []).some((child) => child.kind === 'status')) currentTurn = null;
        if (!currentTurn) beginTurn(event);
        activeExecutionStarted = true;
        currentToolGroup = null;
        tools.clear();
        continue;
      }
      if (event.type === 'model.started') {
        currentToolGroup = null;
        tools.clear();
        addChild({ id: `event-${event.id}`, kind: 'analysis', title: '开始分析', detail: String(payload.detail ?? '正在评估问题并决定下一步行动') });
        continue;
      }
      if (event.type === 'assistant.progress') {
        currentToolGroup = null;
        tools.clear();
        const text = String(payload.text ?? '').trim();
        if (text) addChild({ id: `event-${event.id}`, kind: 'analysis', title: '阶段说明', detail: text });
        continue;
      }
      if (event.type === 'phase.changed') {
        currentToolGroup = null;
        tools.clear();
        const phase = String(payload.phase ?? '').trim();
        if (phase) addChild({ id: `event-${event.id}`, kind: 'phase', title: diagnosisStatusLabel(phase), detail: String(payload.detail ?? '') || undefined });
        continue;
      }
      if (event.type === 'model.resumed') {
        currentToolGroup = null;
        tools.clear();
        const observation = payload.observation ?? payload.observations;
        if (observation !== undefined) addChild({ id: `event-${event.id}`, kind: 'observation', title: '收到工具结果，重新评估', detail: jsonText(observation) });
        continue;
      }
      if (event.type === 'execution.completed' || event.type === 'execution.failed' || event.type === 'execution.cancelled') {
        activeExecutionStarted = false;
        currentToolGroup = null;
        tools.clear();
        addChild({ id: `event-${event.id}`, kind: 'status', title: event.type === 'execution.completed' ? '本轮执行完成' : event.type === 'execution.cancelled' ? '本轮执行已取消' : '本轮执行失败', detail: String(payload.error ?? payload.message ?? '') || undefined });
        continue;
      }
      if (event.type === 'evidence.collected') continue;
      if (!event.type.startsWith('tool.') || !Object.prototype.hasOwnProperty.call(payload, 'resource_id')) continue;
      const key = String(payload.call_id ?? payload.call_sequence ?? event.id);
      if (!currentToolGroup) {
        currentToolGroup = {
          id: `tool-group-${event.id}`,
          kind: 'tool-group',
          title: '连续调用工具',
          status: '执行中',
          children: []
        };
        addChild(currentToolGroup);
      }
      const children = currentToolGroup.children ?? (currentToolGroup.children = []);
      let item = tools.get(key);
      if (!item) {
        item = { id: `tool-${key}`, kind: 'tool', title: toolName(payload.tool), tool: toolName(payload.tool), status: '等待执行', input: jsonText(payload.arguments), output: '等待工具执行结果…', resourceID: String(payload.resource_id ?? '') };
        tools.set(key, item);
        children.push(item);
      }
      if (Object.prototype.hasOwnProperty.call(payload, 'arguments')) item.input = jsonText(payload.arguments);
      if (event.type === 'tool.requested') item.status = '等待执行';
      if (event.type === 'tool.started') item.status = '执行中';
      if (event.type === 'tool.completed' || event.type === 'tool.failed') {
        item.status = event.type === 'tool.completed' ? '已完成' : '失败';
        item.output = jsonText(payload.output, payload.error ? jsonText({ error: payload.error }) : '暂无出参');
        const duration = Number(payload.duration_ms ?? 0);
        if (duration > 0) item.duration = diagnosisDurationMilliseconds(duration);
      }
      currentToolGroup.status = children.some((child) => child.status === '失败')
        ? '存在失败'
        : children.every((child) => child.status === '已完成') ? '已完成' : '执行中';
      const names = [...new Set(children.map((child) => child.tool ?? child.title).filter(Boolean))];
      currentToolGroup.detail = `${children.length} 个动作${names.length ? ` · ${names.join('、')}` : ''}`;
    }
    // Evidence is persisted after the tool result. Link it back to the most
    // likely tool by source resource and capability, while keeping the
    // evidence drawer useful even when a connector does not expose a call id.
    const toolItems = items.flatMap((item) => item.children?.flatMap((child) => child.kind === 'tool-group' ? child.children ?? [] : []) ?? []);
    for (const evidence of collectedEvidence) {
      const candidate = [...toolItems].reverse().find((tool) =>
        (!evidence.sourceResourceID || tool.resourceID === evidence.sourceResourceID) &&
        (!evidence.capability || tool.tool === evidence.capability || tool.tool?.includes(evidence.capability) || evidence.capability.includes(tool.tool ?? ''))
      ) ?? [...toolItems].reverse().find((tool) => !evidence.sourceResourceID || tool.resourceID === evidence.sourceResourceID);
      if (candidate) candidate.evidenceIds = [...(candidate.evidenceIds ?? []), evidence.id];
    }
    const latestExecution = [...events].map((event, index) => ({ event, index }))
      .reverse().find(({ event }) => event.type === 'execution.started');
    const latestRunEvents = latestExecution ? events.slice(latestExecution.index + 1) : events;
    const terminal = Boolean(snapshot.session.status === 'succeeded' ||
      snapshot.session.status === 'failed' ||
      snapshot.session.status === 'cancelled' ||
      latestRunEvents.some((event) => event.type === 'assistant.completed' || event.type === 'report.ready' || event.type === 'diagnosis.failed' || event.type === 'diagnosis.cancelled'));
    if (!terminal) return items;
    return items.map((turn) => ({
      ...turn,
      children: (turn.children ?? []).map((child) => {
        if (child.kind !== 'tool-group') return child;
        const children = (child.children ?? []).map((tool) => {
          if (tool.status === '等待执行') return { ...tool, status: '未执行' };
          if (tool.status === '执行中') return { ...tool, status: '已中断' };
          return tool;
        });
        return {
          ...child,
          children,
          status: children.some((tool) => tool.status === '失败') ? '存在失败' : '已完成'
        };
      })
    }));
  }

  function diagnosisResourceName(resourceID?: string) {
    if (!resourceID) return '未标记资源';
    return resources.find((resource) => resource.id === resourceID)?.name
      ?? diagnosisTargets.find((resource) => resource.id === resourceID)?.name
      ?? resourceID;
  }

  function diagnosisEvidenceSummary(evidence: DiagnosisEvidence) {
    const summary = evidence.summary ?? {};
    const entries = Object.entries(summary).slice(0, 3).map(([key, value]) => {
      const text = typeof value === 'string' ? value : JSON.stringify(value);
      return `${key}: ${text ?? ''}`;
    });
    return entries.join(' · ') || '已保存工具返回结果，可展开查看完整内容。';
  }

  function diagnosisEvidenceSourceTools(snapshot: DiagnosisSnapshot, evidence: DiagnosisEvidence) {
    const timeline = diagnosisEvidenceTimeline(snapshot);
    const labels = timeline.flatMap((turn) => turn.children ?? [])
      .flatMap((item) => item.kind === 'tool-group' ? item.children ?? [] : [item])
      .filter((item) => item.evidenceIds?.includes(evidence.id))
      .map((item) => item.tool ?? item.title);
    return [...new Set(labels)];
  }

  function diagnosisActiveCausalChain(snapshot: DiagnosisSnapshot | null): DiagnosisCausalChain | null {
    if (!snapshot?.causal_chains?.length) return null;
    return snapshot.causal_chains.find((chain) => chain.status === 'active')
      ?? snapshot.causal_chains[0]
      ?? null;
  }

  function diagnosisCausalNodes(chain: DiagnosisCausalChain) {
    const nodes = new Map(chain.nodes.map((node) => [node.id, node]));
    const incoming = new Set(chain.links.map((link) => link.to));
    const ordered: typeof chain.nodes = [];
    const seen = new Set<string>();
    const visit = (id: string) => {
      if (seen.has(id)) return;
      const node = nodes.get(id);
      if (!node) return;
      seen.add(id);
      ordered.push(node);
      for (const link of chain.links.filter((item) => item.from === id)) visit(link.to);
    };
    for (const node of chain.nodes) if (!incoming.has(node.id)) visit(node.id);
    for (const node of chain.nodes) visit(node.id);
    return ordered;
  }

  function diagnosisCausalEvidenceIDs(chain: DiagnosisCausalChain, nodeID: string) {
    const node = chain.nodes.find((item) => item.id === nodeID);
    const related = chain.links.filter((item) => item.from === nodeID || item.to === nodeID);
    return [...new Set([
      ...(node?.evidence_ids ?? []),
      ...related.flatMap((item) => item.evidence_ids ?? [])
    ])];
  }

  function scrollToDiagnosisEvidence(id: string) {
    document.getElementById(`evidence-${id}`)?.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  type DiagnosisTraceItem = {
    id: number;
    kind: 'analysis' | 'action' | 'observation' | 'phase';
    text: string;
    iteration?: number;
  };

  function diagnosisTraceData(snapshot: DiagnosisSnapshot | null): DiagnosisTraceItem[] {
    if (!snapshot) return [];
    const events = [...(snapshot.events ?? [])].sort((a, b) => a.id - b.id);
    const items: DiagnosisTraceItem[] = [];
    const toolTitle = (name: string) => {
      const titles: Record<string, string> = {
        'connector.query_metrics': '查询监控指标',
        'connector.get_alerts': '查询告警',
        'connector.query_logs': '查询日志',
        'connector.inspect_postgresql': '检查 PostgreSQL',
        'connector.read_kubernetes': '查询 Kubernetes'
      };
      return titles[name] ?? (name || '受控工具');
    };
    const phaseTitle = (value: string) => {
      const labels: Record<string, string> = {
        planning: '规划执行路径',
        collecting: '采集环境信息',
        analyzing: '分析环境观察',
        budget_exceeded: '达到执行预算'
      };
      return labels[value] ?? value;
    };
    const observationPreview = (value: unknown) => {
      if (value === undefined || value === null) return '';
      let text = '';
      if (typeof value === 'string') text = value;
      else {
        try {
          text = JSON.stringify(value);
        } catch {
          text = '';
        }
      }
      text = String(text ?? '').replace(/\s+/g, ' ').trim();
      return text.length > 120 ? `${text.slice(0, 120)}…` : text;
    };
    for (const event of events) {
      const payload = event.payload ?? {};
      const iteration = Number(payload.iteration ?? 0) || undefined;
      if (event.type === 'model.started') {
        items.push({ id: event.id, kind: 'analysis', iteration, text: String(payload.detail ?? '正在分析当前目标并评估下一步') });
      } else if (event.type === 'assistant.progress') {
        const text = String(payload.text ?? '').trim();
        if (text) items.push({ id: event.id, kind: 'analysis', iteration, text });
      } else if (event.type === 'tool.requested') {
        if (!Object.prototype.hasOwnProperty.call(payload, 'resource_id')) continue;
        const tool = toolTitle(String(payload.tool ?? ''));
        items.push({ id: event.id, kind: 'action', iteration, text: `准备调用 ${tool}` });
      } else if (event.type === 'tool.started') {
        if (!Object.prototype.hasOwnProperty.call(payload, 'resource_id')) continue;
        const tool = toolTitle(String(payload.tool ?? ''));
        items.push({ id: event.id, kind: 'action', iteration, text: `${tool} 执行中` });
      } else if (event.type === 'tool.completed') {
        if (!Object.prototype.hasOwnProperty.call(payload, 'resource_id')) continue;
        const tool = toolTitle(String(payload.tool ?? ''));
        items.push({ id: event.id, kind: 'observation', iteration, text: `${tool} 已返回结果` });
      } else if (event.type === 'tool.failed') {
        if (!Object.prototype.hasOwnProperty.call(payload, 'resource_id')) continue;
        const tool = toolTitle(String(payload.tool ?? ''));
        items.push({ id: event.id, kind: 'observation', iteration, text: `${tool} 返回错误，正在重新评估` });
      } else if (event.type === 'model.resumed') {
        const batch = Array.isArray(payload.observations) ? payload.observations : [];
        if (batch.length > 1) {
          const names = batch
            .map((item) => toolTitle(String((item as Record<string, unknown>)?.tool ?? '')))
            .filter(Boolean);
          const failed = batch.filter((item) => String((item as Record<string, unknown>)?.outcome ?? '') === 'error').length;
          const summary = failed > 0 ? `，其中 ${failed} 项返回错误` : '';
          items.push({
            id: event.id,
            kind: 'observation',
            iteration,
            text: `已收到 ${names.join('、') || `${batch.length} 个工具`} 的并行结果${summary}，正在重新评估下一步`
          });
        } else {
          const tool = toolTitle(String(payload.tool ?? ''));
          const outcome = String(payload.outcome ?? '') === 'error' ? '错误' : '结果';
          const preview = observationPreview(payload.observation);
          items.push({ id: event.id, kind: 'observation', iteration, text: `已收到 ${tool} 的${outcome}${preview ? `：${preview}` : ''}，正在重新评估下一步` });
        }
      } else if (event.type === 'phase.changed') {
        const phase = String(payload.phase ?? '').trim();
        if (phase) items.push({ id: event.id, kind: 'phase', iteration, text: phaseTitle(phase) });
      }
    }
    return items;
  }

  type DiagnosisLiveTimelineItem = {
    id: number;
    kind: 'analysis' | 'action';
    text?: string;
    tool?: string;
    label?: string;
    status?: string;
    duration?: string;
    elapsed?: string;
    iteration?: number;
    input?: string;
    output?: string;
    actions?: DiagnosisLiveTimelineItem[];
    createdAt?: string;
    updatedAt?: string;
  };

  function diagnosisMilliseconds(value: unknown, fallback = 0) {
    const milliseconds = Number(value);
    return Number.isFinite(milliseconds) && milliseconds >= 0 ? milliseconds : fallback;
  }

  function diagnosisDurationMilliseconds(milliseconds: number) {
    if (!Number.isFinite(milliseconds) || milliseconds < 0) return '—';
    return milliseconds < 1000
      ? `${Math.max(1, Math.round(milliseconds))}ms`
      : `${(milliseconds / 1000).toFixed(milliseconds >= 10000 ? 1 : 2).replace(/\.0+$/, '').replace(/(\.\d)0$/, '$1')}s`;
  }

  function diagnosisLiveTimeline(snapshot: DiagnosisSnapshot | null): DiagnosisLiveTimelineItem[] {
    if (!snapshot) return [];
    const events = [...(snapshot.events ?? [])].sort((a, b) => a.id - b.id);
    const runStart = [...events].reverse().findIndex((event) => event.type === 'execution.started');
    const activeEvents = runStart < 0 ? events : events.slice(events.length - runStart - 1);
    const items: DiagnosisLiveTimelineItem[] = [];
    // A model delta is initially ambiguous: it can become the final answer or
    // the short user-visible explanation that precedes a tool call. Resolve
    // that ambiguity from the complete event batch without changing the text.
    // Deltas belonging to an iteration that contains tool lifecycle events are
    // rendered in the execution timeline in their original event position.
    const toolIterations = new Set(
      activeEvents
        .filter((event) => event.type.startsWith('tool.'))
        .map((event) => Number(event.payload?.iteration ?? 0) || 0)
        .filter((iteration) => iteration > 0)
    );
    const analysisByIteration = new Map<number, DiagnosisLiveTimelineItem>();
    const startedAt = activeEvents.find((event) => event.type === 'execution.started')?.created_at;
    const toolStarted = new Map<string, string>();
    const toolRequested = new Map<string, string>();
    let currentGroup: DiagnosisLiveTimelineItem | null = null;
    let lastAnalysis = '';
    const toolName = (event: DiagnosisEvent) => String(event.payload?.tool ?? '').trim();
    const toolKey = (event: DiagnosisEvent) => String(event.payload?.call_id ?? event.payload?.call_sequence ?? event.id);
    const actionLabel = (tool: string) => {
      const normalized = tool.toLowerCase();
      return /(?:^|[._-])(exec|command|shell|terminal|run)(?:$|[._-])/.test(normalized)
        ? `执行命令 ${tool}`
        : `调用工具 ${tool}`;
    };
    const jsonText = (value: unknown, fallback = '{}') => {
      if (value === undefined || value === null) return fallback;
      if (typeof value === 'string') {
        try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; }
      }
      try { return JSON.stringify(value, null, 2) ?? fallback; } catch { return fallback; }
    };
    const ensureGroup = (event: DiagnosisEvent) => {
      if (currentGroup) return currentGroup;
      currentGroup = {
        id: event.id,
        kind: 'action',
        tool: '',
        status: '执行中',
        duration: '—',
        elapsed: '—',
        iteration: Number(event.payload?.iteration ?? 0) || undefined,
        actions: [],
        createdAt: event.created_at,
        updatedAt: event.created_at
      };
      items.push(currentGroup);
      return currentGroup;
    };
    for (const event of activeEvents) {
      const payload = event.payload ?? {};
      const iteration = Number(payload.iteration ?? 0) || undefined;
      if (event.type === 'assistant.delta') {
        if (iteration && toolIterations.has(iteration)) {
          let analysis = analysisByIteration.get(iteration);
          if (!analysis) {
            analysis = { id: event.id, kind: 'analysis', text: '', iteration };
            analysisByIteration.set(iteration, analysis);
            items.push(analysis);
          }
          analysis.text = `${analysis.text ?? ''}${String(payload.text ?? '')}`;
        }
        continue;
      }
      if (event.type === 'assistant.progress') {
        currentGroup = null;
        const text = String(payload.text ?? '').trim();
        if (String(payload.kind ?? '') === 'tool_decision' && iteration && toolIterations.has(iteration)) {
          // The same sentence may have been emitted as deltas before the
          // function call was finalized. Keep the delta version only once.
          const analysis = analysisByIteration.get(iteration);
          if (analysis && analysis.text) continue;
        }
        if (text && text !== lastAnalysis) {
          items.push({ id: event.id, kind: 'analysis', text, iteration });
          lastAnalysis = text;
        }
        continue;
      }
      if (event.type === 'model.started' || event.type === 'model.resumed') {
        currentGroup = null;
        continue;
      }
      if (!event.type.startsWith('tool.')) continue;
      const tool = toolName(event);
      if (!tool || !Object.prototype.hasOwnProperty.call(payload, 'resource_id')) continue;
      const key = toolKey(event);
      const group = ensureGroup(event);
      const actions = group.actions ?? (group.actions = []);
      let action = actions.find((item) => item.id === Number(key) || item.tool === tool && item.createdAt === toolRequested.get(key));
      if (!action) {
        action = {
          id: event.id,
          kind: 'action',
          tool,
          label: actionLabel(tool),
          status: '等待执行',
          duration: '—',
          elapsed: '—',
          iteration,
          input: payload.arguments === undefined ? undefined : jsonText(payload.arguments),
          output: '等待工具执行结果…',
          createdAt: event.created_at,
          updatedAt: event.created_at
        };
        actions.push(action);
      }
      action.updatedAt = event.created_at;
      if (payload.arguments !== undefined) action.input = jsonText(payload.arguments);
      if (event.type === 'tool.requested') {
        toolRequested.set(key, event.created_at);
        action.status = '等待执行';
      } else if (event.type === 'tool.started') {
        toolStarted.set(key, event.created_at);
        action.status = '执行中';
      } else if (event.type === 'tool.completed' || event.type === 'tool.failed') {
        const started = toolStarted.get(key) ?? toolRequested.get(key) ?? event.created_at;
        const inferredDuration = Math.max(0, new Date(event.created_at).getTime() - new Date(started).getTime());
        const duration = diagnosisMilliseconds(payload.duration_ms, inferredDuration);
        const elapsedFallback = startedAt
          ? Math.max(0, new Date(event.created_at).getTime() - new Date(startedAt).getTime())
          : 0;
        action.status = event.type === 'tool.completed' ? '已完成' : '失败，正在重新评估';
        action.duration = diagnosisDurationMilliseconds(duration);
        action.elapsed = diagnosisDurationMilliseconds(diagnosisMilliseconds(payload.elapsed_ms, elapsedFallback));
        action.output = payload.output === undefined
          ? jsonText(payload.error ? { error: payload.error } : {})
          : jsonText(payload.output);
      }
      group.tool = actions.length === 1 ? tool : undefined;
      group.status = actions.some((item) => item.status === '失败，正在重新评估')
        ? '失败'
        : actions.every((item) => item.status === '已完成' || item.status === '失败，正在重新评估')
          ? '已完成'
          : '执行中';
      group.duration = diagnosisDurationMilliseconds(
        Math.max(0, new Date(event.created_at).getTime() - new Date(group.createdAt ?? event.created_at).getTime())
      );
      group.elapsed = diagnosisDurationMilliseconds(startedAt
        ? Math.max(0, new Date(event.created_at).getTime() - new Date(startedAt).getTime())
        : 0);
      group.updatedAt = event.created_at;
    }
    // A terminal session can be observed before the final tool lifecycle event
    // reaches the client. Do not leave stale requested actions labelled as
    // "等待执行" next to an already persisted answer.
    const latestExecution = [...events].map((event, index) => ({ event, index }))
      .reverse().find(({ event }) => event.type === 'execution.started');
    const latestRunEvents = latestExecution ? events.slice(latestExecution.index + 1) : events;
    const terminal = Boolean(snapshot.session.status === 'succeeded' ||
      snapshot.session.status === 'failed' ||
      snapshot.session.status === 'cancelled' ||
      latestRunEvents.some((event) => event.type === 'assistant.completed' || event.type === 'report.ready' || event.type === 'diagnosis.failed' || event.type === 'diagnosis.cancelled'));
    if (!terminal) return items;
    return items.flatMap((item) => {
      if (item.kind === 'analysis') return [item];
      const actions = (item.actions ?? []).filter((action) => action.status !== '等待执行' && action.status !== '执行中');
      if (!actions.length) return [];
      return [{
        ...item,
        actions,
        tool: actions.length === 1 ? actions[0].tool : undefined,
        status: actions.some((action) => action.status === '失败，正在重新评估') ? '失败' : '已完成'
      }];
    });
  }

  function diagnosisActionGroupComplete(item: DiagnosisLiveTimelineItem) {
    const actions = item.actions ?? [item];
    return actions.length > 1 && actions.every(
      (action) => action.status === '已完成' || action.status === '失败，正在重新评估'
    );
  }

  function diagnosisActionLabel(item: DiagnosisLiveTimelineItem) {
    return item.label || (item.tool ? `调用工具 ${item.tool}` : '执行动作');
  }

  function diagnosisHasRunningActions(snapshot: DiagnosisSnapshot | null) {
    if (snapshot && (snapshot.session.status === 'succeeded' || snapshot.session.status === 'failed' || snapshot.session.status === 'cancelled')) {
      return false;
    }
    return Boolean(
      snapshot && diagnosisActionData(snapshot).some((group) => group.status === '进行中')
    );
  }

  function diagnosisHasPersistedNewAnswer(snapshot: DiagnosisSnapshot | null) {
    return Boolean(
      snapshot &&
      snapshot.messages.filter((message) => message.role === 'assistant').length >
        diagnosisStreamingAssistantBaseline
    );
  }

  function deferFinalDiagnosisMessage(message: DiagnosisMessage, index: number) {
    if ((!diagnosisAnswerCompleted && !diagnosisStreamingText) || message.role !== 'assistant') return false;
    const messages = diagnosisSnapshot?.messages ?? [];
    return index === messages.map((item) => item.role).lastIndexOf('assistant') &&
      diagnosisHasPersistedNewAnswer(diagnosisSnapshot);
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
        throw new Error('请填写有效的 MCP Server 地址和配置。');
      }
      if (resourceKind === 'MCPServer' && !mcpDraftTestPassed()) {
        throw new Error('请先在总结核验步骤完成 MCP Server 连接测试。');
      }
      const config = isProvider
        ? providerConfigForCreate()
        : resourceKind === 'MCPServer'
          ? mcpConfigForSave()
        : buildSchemaConfig(createSchema, resourceConfigValues, resourceConfig);
      const credentialId = isProvider
        ? await createProviderCredential()
        : resourceKind === 'MCPServer'
          ? await createMCPCredential()
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
      const credentialId = await saveProviderCredential(provider);
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

  async function updateMCPFromWorkflow() {
    const server = resources.find((resource) => resource.id === editingResourceId);
    if (!server) return;
    await action(async () => {
      if (!mcpConfigurationValid()) {
        throw new Error('请填写有效的 MCP Server 地址和配置。');
      }
      if (!mcpDraftTestPassed()) {
        throw new Error('请先在总结核验步骤完成 MCP Server 连接测试。');
      }
      const config = mcpConfigForSave();
      const credentialId = await saveMCPCredential(server);
      const updated = await api.updateResource(server.id, {
        name: resourceName.trim(),
        subtype: resourceSubtypeFor({ kind: 'MCPServer', subtype: resourceAddSubtype, config }),
        status: resourceStatus,
        labels: parseLabels(resourceLabels),
        config,
        ...(credentialId ? { credential_id: credentialId } : {})
      });
      resources = resources.map((resource) =>
        resource.id === updated.id ? updated : resource
      );
      selectedResourceId = updated.id;
      editingResourceId = '';
      resourceAddMenuOpen = false;
      resourceAddStep = 1;
      notice = `MCPServer“${updated.name}”已更新`;
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
    // Provider and MCP Server checks use dedicated test/discovery endpoints;
    // the generic connector check history must not overwrite their result.
    if (current.kind === 'AIProvider' || current.kind === 'MCPServer') return;
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
      (resource) =>
        resource.kind !== 'AIProvider' &&
        resource.kind !== 'MCPServer' &&
        resourceHasConnector(resource)
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
      } else if (resource.kind === 'MCPServer') {
        const snapshot = await api.discoverMCP(resource.id);
        operationSnapshots = {
          ...operationSnapshots,
          [resource.id]: [snapshot, ...(operationSnapshots[resource.id] ?? [])]
        };
        check = {
          id: `mcp-server-${resource.id}`,
          resource_id: resource.id,
          status: snapshot.status === 'succeeded' ? 'succeeded' : 'failed',
          message: snapshot.error_message || (snapshot.status === 'succeeded' ? 'MCP Server 连接正常' : 'MCP Server 连接失败'),
          latency_ms: snapshot.latency_ms ?? 0,
          capabilities: [],
          checked_at: new Date().toISOString()
        };
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
    return ['AIProvider', 'MCPServer', 'Kubernetes', 'Prometheus', 'Loki'].includes(
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
      general: '通用',
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
    if (resource.kind === 'MCPServer') {
      openMCPWorkflowForEdit(resource);
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
        throw new Error('请填写有效的 MCP Server 地址和配置。');
      }
      const config = isProvider
        ? providerConfigForCreate()
        : selectedResource.kind === 'MCPServer'
          ? mcpConfigForSave()
        : buildSchemaConfig(selectedSchema, resourceConfigValues, editResourceConfig);
      const credentialId = isProvider
        ? await createProviderCredential()
        : selectedResource.kind === 'MCPServer'
          ? await saveMCPCredential(selectedResource)
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
        void api.credentialSecret(resource.credential_id).then((credential) => {
          if (selectedResourceId !== resource.id) return;
          try {
            const secret = JSON.parse(credential.secret) as { token?: string; headers?: Record<string, unknown> };
            mcpToken = String(secret.token ?? '');
            if (secret.headers) mcpRequestHeaders = Object.entries(secret.headers).map(([key, value]) => `${key}: ${String(value)}`).join('\n');
          } catch {
            // Legacy plain-token credentials have no request headers.
            mcpToken = credential.secret.trim();
          }
        }).catch(() => undefined);
      }
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

  function resetResourceConfig() {
    resourceConfigValues = {};
    resourceSensitiveValues = {};
    resourceConfig = '{}';
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

  function emptyProviderModelDraft(): ProviderModelDraft {
    return {
      name: '',
      contextWindowTokens: 128000,
      maxOutputTokens: 128000,
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

  function mcpTransportForSubtype(subtype: string) {
    return subtype.trim().toLowerCase().replace(/[^a-z]/g, '') === 'sse'
      ? 'sse'
      : 'streamable_http';
  }

  function mcpConfigurationValid() {
    if (!['streamable_http', 'sse'].includes(mcpTransport) || !mcpURL.trim()) return false;
    try {
      const url = new URL(mcpURL.trim());
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return false;
    } catch {
      return false;
    }
    try { parseMCPHeaders(mcpRequestHeaders); } catch { return false; }
    return true;
  }

  function parseMCPHeaders(raw: string): Record<string, string> {
    const headers: Record<string, string> = {};
    for (const line of raw.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)) {
      const separator = line.indexOf(':');
      if (separator <= 0) throw new Error('请求 Header 格式应为“名称: 值”，每行一个。');
      const key = line.slice(0, separator).trim();
      const value = line.slice(separator + 1).trim();
      if (!key || /[\r\n:]/.test(key) || /[\r\n]/.test(value)) throw new Error('请求 Header 名称或值无效。');
      headers[key] = value;
    }
    return headers;
  }
  function mcpHeaderCount() {
    try { return Object.keys(parseMCPHeaders(mcpRequestHeaders)).length; } catch { return 0; }
  }

  function mcpDraftSignature() {
    return JSON.stringify({ transport: mcpTransport, url: mcpURL.trim(), token: mcpToken, headers: mcpRequestHeaders, tools: mcpToolAllowlist, timeout: mcpTimeoutSeconds, max: mcpMaxResponseBytes });
  }

  function mcpDraftTestPassed() {
    return Boolean(mcpDraftTest?.signature === mcpDraftSignature() && mcpDraftTest.result?.status === 'succeeded');
  }

  async function testMCPDraftConnection() {
    const signature = mcpDraftSignature();
    mcpDraftTestBusy = true;
    mcpDraftTest = { signature };
    try {
      if (!mcpConfigurationValid()) throw new Error('请填写有效的 Server 地址和请求 Header。');
      const result = await api.testDraftMCP({
        transport: mcpTransport, url: mcpURL.trim(), token: mcpToken.trim(),
        request_headers: parseMCPHeaders(mcpRequestHeaders),
        tool_allowlist: mcpToolAllowlist.split(/[\n,]/).map((item) => item.trim()).filter(Boolean),
        timeout_seconds: Number(mcpTimeoutSeconds), max_response_bytes: Number(mcpMaxResponseBytes)
      });
      mcpDraftTest = result.status === 'succeeded' ? { signature, result } : { signature, result, error: result.error_message || 'MCP Server 不可用。' };
    } catch (error) {
      mcpDraftTest = { signature, error: describeError(error, 'MCP Server 验证失败') };
    } finally { mcpDraftTestBusy = false; }
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

  async function saveProviderCredential(provider: Resource) {
    if (!provider.credential_id) return createProviderCredential(resourceName);
    await api.updateCredential(provider.credential_id, {
      name: `${resourceName.trim() || 'AI Provider'} API Key`,
      purpose: 'AI Provider 访问凭据',
      secret: providerAPIKey.trim()
    });
    return provider.credential_id;
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
    editingProviderResourceId = '';
    editingResourceId = '';
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
    if (resourceKind === 'MCPServer') mcpTransport = mcpTransportForSubtype(subtype);
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
      !editingProviderResourceId && !editingResourceId
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
    editingResourceId = '';
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

  function openMCPWorkflowForEdit(resource: Resource) {
    selectedScopeId = resource.scope_id;
    selectedResourceId = resource.id;
    resourceKind = 'MCPServer';
    resourceAddCategory = 'MCPServer';
    resourceAddSubtype = resourceSubtypeFor(resource);
    resourceCategory = 'MCPServer';
    resourceSubtype = resourceAddSubtype;
    editingProviderResourceId = '';
    editingResourceId = resource.id;
    resourceName = resource.name;
    resourceStatus = resource.status;
    resourceLabels = Object.entries(resource.labels ?? {})
      .map(([key, value]) => `${key}=${value}`)
      .join(', ');
    syncResourceEditor(resource);
    mcpDraftTest = null;
    mcpConfigurationAttempted = false;
    resourceAddStep = 1;
    resourceTypeSelectionAttempted = false;
    resourceBasicConfigurationAttempted = false;
    resourceEditorOpen = false;
    resourceAddMenuOpen = true;
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
    const validationMessage = providerSummaryValidationMessage();
    if (validationMessage) {
      errorMessage = validationMessage;
      return;
    }
    if (editingProviderResourceId) {
      await updateProviderFromWorkflow();
      return;
    }
    await createResource();
  }

  function providerPurposeConfigurationValid() {
    return providerPurposeTags.every((purpose) => providerPurposeAvailable(purpose));
  }

  function providerSummaryValidationMessage() {
    if (providerAPIKeyLoading) return '正在读取 Provider API Key，请稍候后再保存。';
    if (!providerPurposeConfigurationValid())
      return '当前选择的角色与默认 Model 的能力不匹配，请调整角色或 Model 能力。';
    if (providerDraftTest?.error)
      return `连接测试失败：${providerDraftTest.error}`;
    if (!providerDraftTestPassed())
      return '请先完成默认 Model 的连接测试并确认测试通过。';
    return '';
  }

  function resourceAddStepTitle(step: number, kind: string) {
    if (step === 1) return '基础配置';
    if (kind === 'MCPServer') return ['MCP 配置', '总结核验'][step - 2] ?? 'MCP 配置';
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

  async function createMCPCredential() {
    const token = mcpToken.trim();
    const headers = parseMCPHeaders(mcpRequestHeaders);
    if (!selectedScopeId || (!token && Object.keys(headers).length === 0)) return '';
    const credential = await api.createCredential({
      scope_id: selectedScopeId,
      name: `${resourceName || 'MCP Server'} 访问凭据`,
      purpose: 'MCP Server Token 与请求 Header',
      secret: JSON.stringify({ token, headers })
    });
    return credential.id;
  }

  async function saveMCPCredential(existing: Resource) {
    const headers = parseMCPHeaders(mcpRequestHeaders);
    if (!existing.credential_id) return createMCPCredential();
    let token = mcpToken.trim();
    if (!token) {
      try {
        const current = await api.credentialSecret(existing.credential_id);
        try {
          const parsed = JSON.parse(current.secret) as { token?: string };
          token = String(parsed.token ?? '').trim();
        } catch {
          token = current.secret.trim();
        }
      } catch {
        // Legacy/plain credentials cannot be read as JSON; leave them intact
        // unless the operator supplied a replacement token.
      }
    }
    if (!token && Object.keys(headers).length === 0) return existing.credential_id;
    await api.updateCredential(existing.credential_id, {
      name: `${(editResourceName.trim() || resourceName.trim() || 'MCP Server')} 访问凭据`,
      purpose: 'MCP Server Token 与请求 Header',
      secret: JSON.stringify({ token, headers })
    });
    return existing.credential_id;
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
    Application: ['虚拟机', '容器化', '云原生'],
    Artifact: ['Generic', 'Docker', 'Helm'],
    Repository: ['Git', 'Bundle'],
    Host: ['Direct', 'Agent'],
    Docker: ['Direct', 'Agent'],
    Kubernetes: ['Direct', 'Agent'],
    Redis: ['Direct', 'Agent'],
    TongRDS: ['Direct', 'Agent'],
    Kafka: ['Direct', 'Agent'],
    RabbitMQ: ['Direct', 'Agent'],
    Elasticsearch: ['Direct', 'Agent'],
    OceanBase: ['Direct', 'Agent'],
    Oracle: ['Direct', 'Agent'],
    MySQL: ['Direct', 'Agent'],
    PostgreSQL: ['Direct', 'Agent'],
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
    if (resource.kind === 'Application') return 'Application';
    if (resource.kind === 'Artifact') return 'Artifact';
    if (resource.kind === 'Repository') return 'Repository';
    if (resource.kind === 'Host') return 'Host';
    if (['Redis', 'Kafka', 'Elasticsearch', 'RabbitMQ', 'TongRDS', 'OceanBase', 'Oracle', 'MySQL', 'PostgreSQL'].includes(resource.kind))
      return resource.kind;
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
    const labelMap: Record<string, string> = {
      Application: '虚拟机',
      Artifact: 'Generic',
      Kubernetes: 'Direct',
      Host: 'Direct',
      Docker: 'Direct',
      Redis: 'Direct',
      Kafka: 'Direct',
      Elasticsearch: 'Direct',
      RabbitMQ: 'Direct',
      TongRDS: 'Direct',
      OceanBase: 'Direct',
      Oracle: 'Direct',
      MySQL: 'Direct',
      PostgreSQL: 'Direct',
      Repository: 'Git',
      MCPServer: 'StreamHTTP',
      AIProvider: 'OpenAI',
      Prometheus: '指标',
      Loki: '日志',
      Tempo: '链路',
      Alertmanager: '告警'
    };
    const explicit = String(resource.subtype || resource.config?.subtype || '');
    if (resource.kind === 'AIProvider') return 'Provider';
    if (['Host', 'Docker', 'Kubernetes', 'Redis', 'TongRDS', 'Kafka', 'RabbitMQ', 'Elasticsearch', 'OceanBase', 'Oracle', 'MySQL', 'PostgreSQL'].includes(resource.kind)) {
      return explicit || 'Direct';
    }
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
      Application: '⌘',
      Artifact: '▤',
      Repository: '⌘',
      Host: '▣',
      Docker: '◈',
      Kubernetes: '⬡',
      Redis: '◒',
      TongRDS: '◒',
      Kafka: '◒',
      RabbitMQ: '◒',
      Elasticsearch: '◒',
      OceanBase: '◉',
      Oracle: '◉',
      MySQL: '◉',
      PostgreSQL: '◉',
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

</script>

<svelte:head>
  <meta name="description" content="OpsKeeper platform control plane" />
</svelte:head>

<svelte:window on:keydown={handleGlobalKeydown} />

<AuthGate
  bind:authState={authState}
  currentUser={currentUser}
  bind:loginIdentifier={loginIdentifier}
  bind:password={password}
  bind:passwordVisible={passwordVisible}
  loginError={loginError}
  errorMessage={errorMessage}
  bind:requiredNewPassword={requiredNewPassword}
  bind:requiredConfirmPassword={requiredConfirmPassword}
  bind:requiredNewPasswordVisible={requiredNewPasswordVisible}
  bind:requiredConfirmPasswordVisible={requiredConfirmPasswordVisible}
  busy={busy}
  onLogin={login}
  onChangePassword={changeOwnPassword}
/>
{#if authState !== 'loading' && authState !== 'login' && !currentUser?.must_change_password}
  <AppShell
    sidebarCompact={sidebarCompact}
    sidebarHoverMode={preferences.sidebar_mode === 'hover'}
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
      <Topbar
        breadcrumb={view === 'access' ? viewBreadcrumb(view) : `${activeScope?.name ?? '平台'} / ${viewBreadcrumb(view)}`}
        title={viewTitle(view)}
        activeMessage={activeMessage}
        activeMessageTone={activeMessageTone}
        messageInChildSurface={messageInChildSurface}
        hasPlatformRole={hasPlatformRole}
        selectedTeamId={selectedTeamId}
        selectedProjectId={selectedProjectId}
        teams={teams}
        workspaceProjects={workspaceProjects}
        chooseTeam={chooseTeam}
        chooseProject={chooseProject}
      />

      {#if view === 'overview'}
        <OverviewPage
          teamCount={teams.length}
          projectCount={visibleProjects.length}
          resourceCount={visibleResources.length}
          healthStatus={health?.status}
          rows={rows}
          visibleResources={visibleResources}
          resourceSchemaName={resourceSchemaName}
          scopeName={scopeName}
          resourceIcon={resourceIcon}
          onOpenResources={() => chooseView('resources')}
          onOpenResource={(resource) => {
            selectedResourceId = resource.id;
            chooseView('resources');
            void loadResourceDetails(resource.id);
          }}
        />
      {:else if view === 'profile'}
        <ProfilePage
          currentUser={currentUser}
          avatarURL={avatarURL}
          bind:avatarBusy={avatarBusy}
          bind:profileDisplayName={profileDisplayName}
          bind:profileEmail={profileEmail}
          bind:profilePhone={profilePhone}
          bind:profileCurrentPassword={profileCurrentPassword}
          bind:profileNewPassword={profileNewPassword}
          bind:profileConfirmPassword={profileConfirmPassword}
          bind:preferences={preferences}
          busy={busy}
          onSaveProfile={saveProfile}
          onChangePassword={changeOwnPassword}
          onUploadAvatar={uploadAvatar}
          onApplyTheme={applyTheme}
        />
      {:else if view === 'organization'}
        <OrganizationPage
          teams={teams}
          visibleProjects={visibleProjects}
          bind:selectedScopeId={selectedScopeId}
          bind:projectTeamId={projectTeamId}
          bind:projectName={projectName}
          bind:projectCode={projectCode}
          bind:projectIcon={projectIcon}
          busy={busy}
          teamIconComponent={teamIconComponent}
          iconGlyph={iconGlyph}
          scopeName={scopeName}
          onSelectTeam={(team) => { selectedScopeId = team.scope.id; projectTeamId = team.id; }}
          onSelectProject={(project) => (selectedScopeId = project.scope.id)}
          onOpenTeamDialog={openTeamDialog}
          onCreateProject={createProject}
        />
      {:else if view === 'discovery'}
        <DiscoveryPage
          kubernetesClusters={kubernetesClusters}
          bind:selectedClusterId={selectedClusterId}
          activeDiscovery={activeDiscovery}
          discoveryRuns={discoveryRuns}
          namespaceCandidates={namespaceCandidates}
          applicationCandidates={applicationCandidates}
          bind:projectMappingDrafts={projectMappingDrafts}
          bind:selectedDiscoveryItems={selectedDiscoveryItems}
          teams={teams}
          busy={busy}
          scopeName={scopeName}
          formatDate={formatDate}
          payloadCount={payloadCount}
          allowedTeamsForCluster={allowedTeamsForCluster}
          allowedProjectsForCluster={allowedProjectsForCluster}
          onSelectCluster={selectDiscoveryCluster}
          onStartDiscovery={startDiscovery}
          onOpenDiscovery={openDiscovery}
          onImportDiscovery={importDiscovery}
        />
      {:else if view === 'resources'}
        <section class="resources-layout">
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
                    <div class="mcp-resource-form editor-mcp-form">
                      <div class="form-row"><label><span>资源名称</span><input bind:value={editResourceName} required /></label><label><span>状态</span><select bind:value={editResourceStatus}><option value="active">正常</option><option value="disabled">停用</option><option value="unknown">未知</option></select></label></div>
                      <label><span>标签</span><input bind:value={editResourceLabels} placeholder="env=prod, owner=platform" /></label>
                      <label><span>Server 地址</span><input bind:value={mcpURL} type="url" placeholder="https://mcp.example.com/mcp" /></label>
                      <div class="mcp-number-grid"><label><span>超时时间（秒）</span><input bind:value={mcpTimeoutSeconds} type="number" min="1" max="600" /></label><label><span>响应体大小限制（字节）</span><input bind:value={mcpMaxResponseBytes} type="number" min="1" max="16777216" step="1024" /></label></div>
                      <label><span>Token</span><input bind:value={mcpToken} type="password" placeholder="留空保持原凭据" /></label>
                      <label><span>请求 Header</span><textarea bind:value={mcpRequestHeaders} rows="3" placeholder="每行一个 Header，例如 X-Tenant: production"></textarea></label>
                      <label><span>工具白名单</span><textarea bind:value={mcpToolAllowlist} rows="5" placeholder="支持通配符，例如 docker:*&#10;为空表示允许全部工具"></textarea></label>
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
      {:else if view === 'inspection'}
        <InspectionPage
          bind:policies={inspectionPolicies}
          bind:runs={inspectionRuns}
          bind:findings={inspectionFindings}
          bind:channels={notificationChannels}
          executableTargets={executableTargets}
          agentProfiles={agentProfileResources}
          bind:policyName={inspectionPolicyName}
          bind:cron={inspectionCron}
          bind:timezone={inspectionTimezone}
          bind:targetIds={inspectionTargetIds}
          bind:agentProfileId={inspectionAgentProfileId}
          bind:targetLabels={inspectionTargetLabels}
          bind:timeoutSeconds={inspectionTimeoutSeconds}
          bind:retries={inspectionRetries}
          bind:maxConcurrent={inspectionMaxConcurrent}
          bind:maxToolCalls={inspectionMaxToolCalls}
          bind:maxTokens={inspectionMaxTokens}
          bind:channelName={channelName}
          bind:channelWebhookURL={channelWebhookURL}
          bind:channelRateLimit={channelRateLimit}
          busy={busy}
          scopeName={scopeName}
          resourceInActiveWorkspace={resourceInActiveWorkspace}
          toggleSelection={toggleInspectionSelection}
          onCreatePolicy={createInspectionPolicy}
          onRerun={rerunInspection}
          onSetPolicyStatus={setInspectionPolicyStatus}
          onCreateChannel={createNotificationChannel}
        />
      {:else if view === 'operations'}
        <OperationsPage
          resources={resources}
          selectedScopeId={selectedScopeId}
          bind:operationTargetId={operationTargetId}
          bind:operationName={operationName}
          bind:operationRisk={operationRisk}
          bind:operationParameters={operationParameters}
          bind:operationImpact={operationImpact}
          bind:operationRollback={operationRollback}
          operationSnapshots={operationSnapshots}
          operationRequests={operationRequests}
          busy={busy}
          resourceSchemaName={resourceSchemaName}
          formatDate={formatDate}
          onCreateRequest={createOperationRequest}
          onDiscoverMCP={discoverMCP}
          onApprove={approveOperation}
          onStart={startOperation}
        />
      {:else if view === 'diagnosis'}
        <section
          class="diagnosis-workbench-f"
          class:history-collapsed={diagnosisHistoryCollapsed}
          class:context-collapsed={diagnosisContextCollapsed}
          style={`--diagnosis-history-width:${diagnosisHistoryWidth}px;--diagnosis-context-width:${diagnosisContextWidth}px`}
        >
          {#if !diagnosisHistoryCollapsed}
            <DiagnosisSessionList
              sessions={diagnosisSessions}
              selectedSessionId={selectedDiagnosisId}
              bind:search={diagnosisSessionSearch}
              statusLabel={diagnosisStatusLabel}
              formatDate={formatDate}
              onClear={clearDiagnosisHistory}
              onCreate={newDiagnosisSession}
              onOpen={(sessionID) => void openDiagnosis(sessionID)}
              onRename={renameDiagnosisSession}
              onDelete={deleteDiagnosisSession}
            />
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
            <DiagnosisConversationHeader
              title={diagnosisSnapshot?.session.title || '新建诊断会话'}
              scopeLabel={activeScope?.name ?? '当前级别'}
              diagnosisTargets={diagnosisTargets}
              diagnosisTargetIds={diagnosisTargetIds}
              generating={diagnosisGenerating}
              snapshot={diagnosisSnapshot}
              statusLabel={diagnosisStatusLabel}
              onCreate={newDiagnosisSession}
            />
            <DiagnosisConversationMessages
              bind:diagnosisMessageListElement={diagnosisMessageListElement}
              diagnosisSnapshot={diagnosisSnapshot}
              busy={busy}
              diagnosisGenerating={diagnosisGenerating}
              diagnosisAnswerCompleted={diagnosisAnswerCompleted}
              diagnosisStreamingText={diagnosisStreamingText}
              diagnosisInterruptedReason={diagnosisInterruptedReason}
              bind:diagnosisEditingMessageId={diagnosisEditingMessageId}
              bind:diagnosisEditDraft={diagnosisEditDraft}
              bind:diagnosisProcessExpanded={diagnosisProcessExpanded}
              bind:diagnosisActionExpanded={diagnosisActionExpanded}
              bind:diagnosisLiveProcessExpanded={diagnosisLiveProcessExpanded}
              diagnosisStreamingStartedAt={diagnosisStreamingStartedAt}
              formatDate={formatDate}
              renderMarkdown={renderDiagnosisMarkdownShared}
              diagnosisLiveTimeline={diagnosisLiveTimeline}
              diagnosisProcessDuration={diagnosisProcessDuration}
              diagnosisProcessActionCount={diagnosisProcessActionCount}
              diagnosisStatusLabel={diagnosisStatusLabel}
              diagnosisHasRunningActions={diagnosisHasRunningActions}
              diagnosisHasPersistedNewAnswer={diagnosisHasPersistedNewAnswer}
              diagnosisActionLabel={diagnosisActionLabel}
              deferFinalDiagnosisMessage={deferFinalDiagnosisMessage}
              isLastDiagnosisUser={isLastDiagnosisUser}
              beginDiagnosisEdit={beginDiagnosisEdit}
              saveDiagnosisEdit={saveDiagnosisEdit}
              copyDiagnosisMessage={copyDiagnosisMessage}
              copyStreamingAnswer={() => writeDiagnosisClipboard(
                `${diagnosisStreamingText}\n\n执行过程\n\n${diagnosisProcessText(diagnosisSnapshot)}`,
                '回答已复制。'
              )}
            />
            <DiagnosisComposer
              text={diagnosisComposerText}
              busy={busy}
              generating={diagnosisGenerating}
              providers={diagnosisAvailableProviders}
              selectedProviderId={selectedProviderId}
              modelName={llmModelName}
              modelMenuOpen={diagnosisModelMenuOpen}
              modelMenuProviderId={diagnosisModelMenuProviderId}
              onTextChange={(value) => (diagnosisComposerText = value)}
              onKeydown={handleDiagnosisComposerKeydown}
              onSubmit={() => void submitDiagnosisMessage()}
              onStop={stopDiagnosisGeneration}
              onNotice={(message) => (notice = message)}
              onModelToggle={toggleDiagnosisModelMenu}
              onProvider={chooseDiagnosisModelProvider}
              onModel={chooseDiagnosisModel}
            />
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
            <DiagnosisContextPanel
              diagnosisSnapshot={diagnosisSnapshot}
              diagnosisTargets={diagnosisTargets}
              diagnosisTargetIds={diagnosisTargetIds}
              bind:diagnosisContextTab={diagnosisContextTab}
              toggleDiagnosisContext={toggleDiagnosisContext}
              resourceIcon={resourceIcon}
              resourceSchemaName={resourceSchemaName}
              scopeName={scopeName}
              diagnosisActiveCausalChain={diagnosisActiveCausalChain}
              diagnosisCausalNodes={diagnosisCausalNodes}
              diagnosisCausalEvidenceIDs={diagnosisCausalEvidenceIDs}
              scrollToDiagnosisEvidence={scrollToDiagnosisEvidence}
              diagnosisEvidenceTimeline={diagnosisEvidenceTimeline}
              diagnosisEvidenceSourceTools={diagnosisEvidenceSourceTools}
              diagnosisEvidenceSummary={diagnosisEvidenceSummary}
              diagnosisResourceName={diagnosisResourceName}
              formatDate={formatDate}
            />
          {/if}
        </section>
      {:else if view === 'agent'}
        <AgentProfilesPage
          profiles={agentProfileResources}
          bind:selectedProfileId={selectedAgentProfileId}
          bind:selectedVersionId={selectedAgentProfileVersionId}
          bind:versions={agentProfileVersions}
          bind:profileName={agentProfileName}
          bind:profileInstruction={agentProfileInstruction}
          bind:profileCapabilities={agentProfileCapabilities}
          bind:profileAllowedTools={agentProfileAllowedTools}
          bind:profileTargetKinds={agentProfileTargetKinds}
          bind:profileInputSchema={agentProfileInputSchema}
          bind:profileOutputSchema={agentProfileOutputSchema}
          busy={busy}
          selectedScopeId={selectedScopeId}
          scopeName={scopeName}
          formatDate={formatDate}
          onLoadVersions={loadAgentProfileVersions}
          onPublish={publishAgentProfileVersion}
          onCreate={createAgentProfile}
        />
      {:else if view === 'skill'}
        <SkillRegistryPage
          resources={skillResources}
          bind:selectedSkillId={selectedSkillId}
          bind:selectedVersionId={selectedSkillVersionId}
          bind:versions={skillVersions}
          bind:instruction={skillInstruction}
          bind:targetKinds={skillTargetKinds}
          bind:selectedToolNames={selectedSkillToolNames}
          bind:inputSchema={skillInputSchema}
          bind:outputSchema={skillOutputSchema}
          toolOptions={skillToolOptions}
          busy={busy}
          scopeName={scopeName}
          onLoadVersions={loadSkillVersions}
          onSetDefault={setSkillDefault}
          onPublish={publishSkillVersion}
          onCreate={createSkillVersion}
          onToggleTool={toggleSkillTool}
        />
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
  </AppShell>
{/if}
