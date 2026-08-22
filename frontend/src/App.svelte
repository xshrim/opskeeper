<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Boxes,
    Building2,
    ChevronDown,
    ClipboardCheck,
    CloudDownload,
    Copy,
    Eye,
    EyeOff,
    FolderKanban,
    LayoutDashboard,
    LogOut,
    Monitor,
    Moon,
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
    Sun,
    Trash2,
    Upload,
    UserRound,
    UsersRound,
    icons as lucideIcons
  } from 'lucide-svelte';
  import { fetchHealth, toStatusRows, type HealthReport } from './lib/health';
  import BrandIcon from './lib/BrandIcon.svelte';
  import {
    api,
    ApiError,
    type ConnectionCheck,
    type ConnectorCapability,
    type DiagnosisEvidence,
    type DiagnosisSession,
    type DiagnosisSnapshot,
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
    type LLMConnectionResult,
    type InspectionPolicy,
    type InspectionRun,
    type InspectionFinding,
    type NotificationChannel,
    type OperationRequest,
    type MCPSnapshot,
    type SkillExecution,
    type SkillVersion,
    type TopologyNode,
    type User,
    type UserPreferences
  } from './lib/api';

  type View =
    | 'overview'
    | 'organization'
    | 'discovery'
    | 'resources'
    | 'ai'
    | 'skill'
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

  type AIEndpointDraft = {
    provider_type: string;
    base_url: string;
    model_name: string;
    credential: string;
    temperature: number;
    context_window: number;
    capabilities: string[];
    timeout_seconds: number;
    priority: number;
    enabled: boolean;
    testStatus: 'idle' | 'testing' | 'succeeded' | 'failed';
    testMessage?: string;
    latencyMs?: number;
  };
  type AIProviderOption = {
    value: string;
    label: string;
    icon: string;
    baseUrl: string;
  };

  const legacyTeamIconNames: Record<string, string> = {
    platform: 'Boxes',
    team: 'UsersRound',
    project: 'FolderKanban',
    application: 'AppWindow',
    api: 'Waypoints',
    building: 'Building2',
    cloud: 'Cloud',
    kubernetes: 'Boxes',
    endpoint: 'Network',
    middleware: 'ServerCog',
    postgresql: 'Database',
    redis: 'DatabaseZap',
    kafka: 'Radio',
    metrics: 'ChartNoAxesCombined',
    logs: 'FileText',
    traces: 'GitBranch',
    observability: 'ScanSearch',
    notification: 'Bell',
    schedule: 'Clock',
    search: 'Search',
    runbook: 'BookOpen',
    skill: 'Sparkles',
    llm: 'BrainCircuit',
    mcp: 'Bot',
    storage: 'HardDrive',
    credential: 'KeyRound',
    resource: 'Package'
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
    BrainCircuit: 'AI 引擎',
    Bot: 'MCP',
    HardDrive: '存储',
    KeyRound: '凭据',
    Package: '资源'
  };

  const aiProviderOptions: AIProviderOption[] = [
    { value: 'openai_compatible', label: 'OpenAI 兼容', icon: 'Waypoints', baseUrl: 'https://api.example.com/v1' },
    { value: 'openai', label: 'OpenAI', icon: 'Sparkles', baseUrl: 'https://api.openai.com/v1' },
    { value: 'anthropic', label: 'Anthropic', icon: 'BrainCircuit', baseUrl: 'https://api.anthropic.com/v1' },
    { value: 'gemini', label: 'Gemini', icon: 'Orbit', baseUrl: 'https://generativelanguage.googleapis.com/v1beta/openai' },
    { value: 'grok', label: 'Grok', icon: 'Bot', baseUrl: 'https://api.x.ai/v1' },
    { value: 'deepseek', label: 'DeepSeek', icon: 'Search', baseUrl: 'https://api.deepseek.com/v1' },
    { value: 'qwen', label: 'Qwen', icon: 'Cloud', baseUrl: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
    { value: 'kimi', label: 'Kimi', icon: 'Moon', baseUrl: 'https://api.moonshot.cn/v1' },
    { value: 'glm', label: 'GLM', icon: 'CircuitBoard', baseUrl: 'https://open.bigmodel.cn/api/paas/v4' },
    { value: 'minimax', label: 'MiniMax', icon: 'Boxes', baseUrl: 'https://api.minimax.chat/v1' },
    { value: 'mimo', label: 'MiMo', icon: 'Cpu', baseUrl: 'https://api.xiaomimimo.com/v1' },
    { value: 'longcat', label: 'LongCat', icon: 'Cat', baseUrl: 'https://api.longcat.chat/v1' },
    { value: 'doubao', label: 'Doubao', icon: 'CloudCog', baseUrl: 'https://ark.cn-beijing.volces.com/api/v3' },
    { value: 'openrouter', label: 'OpenRouter', icon: 'Network', baseUrl: 'https://openrouter.ai/api/v1' },
    { value: 'siliconflow', label: 'SiliconFlow', icon: 'Waves', baseUrl: 'https://api.siliconflow.cn/v1' },
    { value: 'ollama', label: 'Ollama', icon: 'ServerCog', baseUrl: 'http://localhost:11434/v1' }
  ];

  const teamIconOptions: TeamIconOption[] = Object.keys(lucideIcons)
    .sort((left, right) => left.localeCompare(right))
    .map((value) => ({
      value,
      label: commonTeamIconLabels[value] ?? formatIconName(value),
      keywords: `${value} ${formatIconName(value)} ${commonTeamIconLabels[value] ?? ''}`
    }));

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
  let notice = '';
  let noticeTimer: number | null = null;
  let errorMessage = '';
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
  let selectedAIEngineId = '';
  let selectedAIEngineResource: Resource | null = null;
  let expandedAIEngineId = '';
  let aiEngineSearch = '';
  let aiEngineStatusFilter = 'all';
  let aiEngineScopeFilter = 'all';
  let aiEngineCreateOpen = false;
  let aiEngineCreateStep = 1;
  let aiEngineCreateDescription = '';
  let aiEngineCreateFallback = ['timeout', 'rate_limit', 'server_error'];
  let aiEngineCreateEndpoints: AIEndpointDraft[] = [];
  let aiEngineCreateDraft = defaultAIEndpoint();
  let aiEngineEditingEndpointIndex = -1;
  let aiEngineIcon = 'BrainCircuit';
  let aiModelCredentialVisible = false;
  let aiProviderMenuOpen = false;
  let aiEngineCreateDefault = false;
  let aiEngineModelError = '';
  let aiEngineModelMissing: string[] = [];
  let aiEngineModelTesting = false;
  let aiEngineRefreshing = false;
  let aiEngineEndpointTestBusy = false;
  let aiEngineName = '';
  let aiEngineStrategy = 'priority';
  let aiEngineEditName = '';
  let aiEngineEditEndpoints = '[]';
  let aiEngineEditStatus = 'active';
  let aiEngineEndpoints =
    '[\n  {\n    "provider_type": "openai_compatible",\n    "base_url": "https://api.example.com/v1",\n    "model_name": "model-name",\n    "context_window": 32768,\n    "capabilities": ["chat", "stream"],\n    "timeout_seconds": 60,\n    "priority": 100,\n    "enabled": true\n  }\n]';
  let selectedSkillId = '';
  let selectedSkillVersionId = '';
  let llmModelName = '';
  let llmConnection: LLMConnectionResult | null = null;
  let skillVersions: SkillVersion[] = [];
  let skillExecutions: SkillExecution[] = [];
  let skillInstruction = '';
  let skillTargetKinds = 'Application';
  let skillInputSchema = '{"type":"object","additionalProperties":true}';
  let skillOutputSchema = '{"type":"object","additionalProperties":true}';
  let skillInput = '{}';
  let skillTargetResourceId = '';
  let skillOutput = '';
  let selectedSkillToolNames: string[] = [];
  let diagnosisLoaded = false;
  let diagnosisSessions: DiagnosisSession[] = [];
  let selectedDiagnosisId = '';
  let diagnosisSnapshot: DiagnosisSnapshot | null = null;
  let diagnosisQuestion = '';
  let diagnosisTargetIds: string[] = [];
  let diagnosisFollowup = '';
  let selectedEvidence: DiagnosisEvidence | null = null;
  let diagnosisEvents: EventSource | null = null;
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
  let inspectionSkillIds: string[] = [];
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
  let teamIcon = 'team';
  let projectTeamId = '';
  let projectName = '';
  let projectCode = '';
  let projectIcon = 'project';
  let resourceKind = '';
  let resourceCategory = '全部';
  let resourceSubtype = '全部';
  let resourceSearch = '';
  let resourceStatusFilter = 'all';
  let resourceLevelFilter = 'all';
  let expandedResourceCategory = '';
  let resourceEditorOpen = false;
  let resourceAddMenuOpen = false;
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
  let iconPickerTarget: 'create' | 'edit' | 'ai-engine' | null = null;
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
      `${resource.name} ${resource.kind} ${resourceSchemaName(resource.kind)} ${scopeName(resource.scope_id)}`
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
    ['Kubernetes', 'Prometheus', 'Loki'].includes(selectedResource.kind)
  );
  $: createSchema = schemas.find((schema) => schema.kind === resourceKind);
  $: kubernetesClusters = resources.filter(
    (resource) => resource.kind === 'Kubernetes'
  );
  $: namespaceCandidates = discoveryItems.filter(
    (item) => item.kind === 'Project'
  );
  $: applicationCandidates = discoveryItems.filter(
    (item) => item.kind === 'Application'
  );
  $: aiEngines = resources.filter(
    (item) => item.kind === 'AIEngine' || item.kind === 'LLMProvider'
  );
  $: visibleAIEngines = aiEngines.filter((engine) => {
    if (
      aiEngineStatusFilter !== 'all' &&
      engine.status !== aiEngineStatusFilter
    )
      return false;
    if (
      aiEngineScopeFilter !== 'all' &&
      scopeType(engine.scope_id) !== aiEngineScopeFilter
    )
      return false;
    const query = aiEngineSearch.trim().toLowerCase();
    return (
      !query ||
      `${engine.name} ${scopeName(engine.scope_id)} ${aiEngineDescription(engine)}`
        .toLowerCase()
        .includes(query)
    );
  });
  $: aiEngineHealthyCount = aiEngines.filter(
    (engine) => aiEngineStatus(engine) === 'healthy'
  ).length;
  $: aiEngineRepairCount = aiEngines.filter(
    (engine) => aiEngineStatus(engine) === 'repair'
  ).length;
  $: aiEngineEndpointCount = aiEngines.reduce(
    (count, engine) => count + aiEngineEndpointsFor(engine).length,
    0
  );
  $: aiEngineHealthyEndpointCount = aiEngines.reduce(
    (count, engine) =>
      count +
      aiEngineEndpointsFor(engine).filter(
        (endpoint) => endpointStatus(endpoint) === 'healthy'
      ).length,
    0
  );
  $: selectedAIEngineResource =
    aiEngines.find((item) => item.id === selectedAIEngineId) ?? null;
  $: llmProviders = aiEngines;
  $: skillResources = resources.filter((item) => item.kind === 'Skill');
  $: executableTargets = visibleResources.filter(
    (item) =>
      item.kind !== 'LLMProvider' &&
      item.kind !== 'AIEngine' &&
      item.kind !== 'Skill'
  );
  $: diagnosisTargets = visibleResources.filter(
    (item) => item.status === 'active'
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
      await loadResourceConnectionChecks(resources);
      const projectPages = await Promise.all(
        teams.map((team) => api.projects(team.id))
      );
      projects = projectPages.flatMap((page) => page.items);
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
    if ((nextView === 'ai' || nextView === 'skill') && !aiLoaded) void loadAI();
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
    if (accessMenuOpen && !target.closest('.nav-group')) {
      accessMenuOpen = false;
    }
    if (
      resourceAddMenuOpen &&
      !target.closest('.resource-add-menu, .resource-add-menu-trigger')
    ) {
      resourceAddMenuOpen = false;
    }
  }

  function chooseTeam(teamID: string) {
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
    const project = selectedTeamProjects.find((item) => item.id === projectID);
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
        skill_resource_ids: inspectionSkillIds,
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
    try {
      diagnosisSnapshot = await api.diagnosisSession(id);
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
    stream.onmessage = () => void refreshDiagnosis(id);
    for (const type of [
      'session.created',
      'phase.changed',
      'plan.created',
      'tool.completed',
      'evidence.collected',
      'report.ready',
      'diagnosis.failed',
      'message.created',
      'target.added'
    ]) {
      stream.addEventListener(type, (event) => {
        diagnosisEventCursor =
          Number((event as MessageEvent).lastEventId) || diagnosisEventCursor;
        void refreshDiagnosis(id);
      });
    }
    stream.onerror = () => {
      // Native EventSource reconnects with the last received event id.
    };
  }

  function closeDiagnosisEvents() {
    diagnosisEvents?.close();
    diagnosisEvents = null;
  }

  async function refreshDiagnosis(id = selectedDiagnosisId) {
    if (!id || id !== selectedDiagnosisId) return;
    try {
      diagnosisSnapshot = await api.diagnosisSession(id);
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
    if (!selectedScopeId || diagnosisTargetIds.length === 0) return;
    await action(async () => {
      const session = await api.startDiagnosis({
        scope_id: selectedScopeId,
        question: diagnosisQuestion,
        target_resource_ids: diagnosisTargetIds
      });
      diagnosisSessions = [session, ...diagnosisSessions];
      diagnosisQuestion = '';
      diagnosisTargetIds = [];
      notice = '诊断会话已创建，正在建立受控证据链。';
      await openDiagnosis(session.id);
    });
  }

  async function sendDiagnosisFollowup() {
    if (!selectedDiagnosisId || !diagnosisFollowup.trim()) return;
    await action(async () => {
      await api.askDiagnosis(selectedDiagnosisId, diagnosisFollowup);
      diagnosisFollowup = '';
      notice = '追问已提交，正在重新诊断。';
      await refreshDiagnosis();
      openDiagnosisEvents(selectedDiagnosisId);
    });
  }

  async function loadAI() {
    aiLoaded = true;
    selectedAIEngineId = selectedAIEngineId || aiEngines[0]?.id || '';
    selectedProviderId = selectedAIEngineId;
    selectedSkillId = selectedSkillId || skillResources[0]?.id || '';
    const selectedEngine = aiEngines.find(
      (item) => item.id === selectedAIEngineId
    );
    const endpoints = selectedEngine?.config.endpoints as
      Array<{ model_name?: string }> | undefined;
    const legacyModels = selectedEngine?.config.models as
      Array<{ name?: string }> | undefined;
    llmModelName =
      llmModelName ||
      endpoints?.[0]?.model_name ||
      legacyModels?.[0]?.name ||
      '';
    syncAIEngineEditor(selectedEngine ?? null);
    if (selectedSkillId) await loadSkillVersions();
    if (selectedScopeId) {
      try {
        skillExecutions = await api.skillExecutions(selectedScopeId);
      } catch {
        skillExecutions = [];
      }
    }
  }

  async function refreshAIEngines() {
    if (aiEngineRefreshing) return;
    aiEngineRefreshing = true;
    errorMessage = '';
    try {
      const resourcePage = await api.resources();
      resources = resourcePage.items;
      await loadResourceConnectionChecks(resources);
    } catch (error) {
      errorMessage = describeError(error, 'AI 引擎状态刷新失败');
    } finally {
      aiEngineRefreshing = false;
    }
  }

  function syncAIEngineEditor(engine: Resource | null) {
    if (!engine) {
      aiEngineEditName = '';
      aiEngineEditEndpoints = '[]';
      aiEngineEditStatus = 'active';
      return;
    }
    aiEngineEditName = engine.name;
    aiEngineEditEndpoints = JSON.stringify(
      engine.config.endpoints ?? engine.config.models ?? [],
      null,
      2
    );
    aiEngineEditStatus = engine.status;
  }

  function aiEngineDescription(engine: Resource) {
    return String(
      engine.config.description ?? engine.labels?.description ?? ''
    );
  }

  function aiEngineIconFor(engine: Resource) {
    return String(engine.config.icon ?? 'BrainCircuit');
  }

  function aiEngineEndpointsFor(engine: Resource | null): AIEndpointDraft[] {
    if (!engine) return [];
    const raw = Array.isArray(engine.config.endpoints)
      ? engine.config.endpoints
      : Array.isArray(engine.config.models)
        ? engine.config.models
        : [];
    return raw.map((value, index) => {
      const endpoint = (value ?? {}) as Record<string, unknown>;
      return {
        provider_type: String(endpoint.provider_type ?? 'openai_compatible'),
        base_url: String(endpoint.base_url ?? ''),
        model_name: String(endpoint.model_name ?? endpoint.name ?? ''),
        credential: '',
        context_window: Number(endpoint.context_window ?? 128000),
        temperature: Number(endpoint.temperature ?? 0.7),
        capabilities: Array.isArray(endpoint.capabilities)
          ? endpoint.capabilities.map(String)
          : ['chat', 'tool_calling', 'structured_output', 'stream', 'deep_thinking'],
        timeout_seconds: Number(endpoint.timeout_seconds ?? 60),
        priority: Number(endpoint.priority ?? 100 - index),
        enabled: endpoint.enabled !== false,
        testStatus: 'idle'
      };
    });
  }

  function endpointStatus(endpoint: AIEndpointDraft) {
    if (!endpoint.enabled) return 'disabled';
    if (endpoint.testStatus === 'failed') return 'repair';
    if (endpoint.testStatus === 'succeeded') return 'healthy';
    return 'pending';
  }

  function aiEngineStatus(engine: Resource) {
    if (engine.status !== 'active') return 'disabled';
    const endpoints = aiEngineEndpointsFor(engine);
    if (
      endpoints.length === 0 ||
      endpoints.every((endpoint) => !endpoint.enabled)
    )
      return 'repair';
    return 'healthy';
  }

  function aiEngineStatusLabel(engine: Resource) {
    const status = aiEngineStatus(engine);
    return status === 'healthy'
      ? '正常'
      : status === 'repair'
        ? '异常'
        : '已停用';
  }

  function aiEngineHealthRatio(engine: Resource) {
    const endpoints = aiEngineEndpointsFor(engine);
    const healthy =
      engine.status === 'active'
        ? endpoints.filter(
            (endpoint) => endpoint.enabled && endpoint.testStatus !== 'failed'
          ).length
        : 0;
    return `${healthy}/${endpoints.length}`;
  }

  function aiEngineStatusClass(engine: Resource) {
    const status = aiEngineStatus(engine);
    return status === 'healthy'
      ? 'healthy'
      : status === 'repair'
        ? 'warning'
        : 'disabled';
  }

  function aiEngineCapabilities(engine: Resource) {
    return aiCapabilitiesForEndpoints(aiEngineEndpointsFor(engine));
  }

  function aiCapabilitiesForEndpoints(endpoints: AIEndpointDraft[]) {
    endpoints = endpoints.filter((endpoint) => endpoint.enabled);
    if (endpoints.length === 0) return [];
    return endpoints.reduce<string[]>(
      (intersection, endpoint) =>
        intersection.filter((capability) =>
          endpoint.capabilities.includes(capability)
        ),
      [...endpoints[0].capabilities]
    );
  }

  function deriveAIEndpointCapabilities(endpoint: AIEndpointDraft) {
    const model =
      `${endpoint.provider_type} ${endpoint.model_name}`.toLowerCase();
    const capabilities = new Set<string>(['chat', 'stream']);
    if (/vision|multimodal|gpt-4o|gpt-4\.1|gemini|claude-3|qwen-vl/.test(model))
      capabilities.add('vision');
    if (/audio|omni|gemini|gpt-4o/.test(model)) capabilities.add('audio');
    if (/tool|function|gpt-|claude|gemini|qwen/.test(model))
      capabilities.add('tool_calling');
    if (/json|structured|gpt-|claude|gemini|qwen/.test(model))
      capabilities.add('structured_output');
    if (/reason|think|o1|o3|r1|deepseek-r1/.test(model))
      capabilities.add('deep_thinking');
    if (endpoint.context_window >= 128000 || /long|128k|200k|1m/.test(model))
      capabilities.add('long_context');
    return [...capabilities];
  }

  function aiCapabilityLabel(capability: string) {
    const labels: Record<string, string> = {
      chat: '文本',
      vision: '视觉',
      audio: '音频',
      tool_calling: '工具调用',
      structured_output: '结构化输出',
      stream: '流式输出',
      long_context: '长上下文',
      deep_thinking: '深度思考'
    };
    return labels[capability] ?? capability;
  }

  function aiEndpointHealthLabel(endpoint: AIEndpointDraft) {
    const status = endpointStatus(endpoint);
    return status === 'healthy'
      ? '健康'
      : status === 'repair'
        ? '失败'
        : status === 'disabled'
          ? '已停用'
          : '未测试';
  }

  function aiEndpointHealthClass(endpoint: AIEndpointDraft) {
    const status = endpointStatus(endpoint);
    return status === 'healthy'
      ? 'healthy'
      : status === 'repair'
        ? 'warning'
        : status === 'disabled'
          ? 'disabled'
          : 'pending';
  }

  function aiEndpointConnectionLabel(endpoint: AIEndpointDraft) {
    if (endpoint.testStatus === 'testing') return '连接中...';
    if (endpoint.testStatus === 'succeeded')
      return `正常 · ${endpoint.latencyMs ?? 35} ms`;
    if (endpoint.testStatus === 'failed') return '连接失败';
    if (!endpoint.model_name.trim() || !endpoint.base_url.trim())
      return '待配置';
    return '待测试';
  }

  function defaultAIEndpoint(): AIEndpointDraft {
    return {
      provider_type: 'openai_compatible',
      base_url: 'https://api.example.com/v1',
      model_name: '',
      credential: '',
      context_window: 128000,
      capabilities: ['chat', 'tool_calling', 'structured_output', 'stream', 'deep_thinking'],
      temperature: 0.7,
      timeout_seconds: 60,
      priority: 100,
      enabled: true,
      testStatus: 'idle'
    };
  }

  function aiProviderOption(value: string) {
    return aiProviderOptions.find((option) => option.value === value) ?? aiProviderOptions[0];
  }

  function selectAIProvider(value: string) {
    const option = aiProviderOption(value);
    aiEngineCreateDraft = {
      ...aiEngineCreateDraft,
      provider_type: option.value,
      base_url: option.baseUrl
    };
    aiProviderMenuOpen = false;
    clearAIEngineModelFieldError('provider');
  }

  function openAIEngineCreate() {
    aiEngineCreateOpen = true;
    aiEngineCreateStep = 1;
    aiEngineName = '';
    aiEngineCreateDescription = '';
    aiEngineCreateFallback = ['timeout', 'rate_limit', 'server_error'];
    aiEngineCreateEndpoints = [];
    aiEngineCreateDraft = defaultAIEndpoint();
    aiEngineEditingEndpointIndex = -1;
    aiEngineIcon = 'BrainCircuit';
    aiModelCredentialVisible = false;
    aiProviderMenuOpen = false;
    aiEngineCreateDefault = false;
    aiEngineModelError = '';
    aiEngineModelMissing = [];
    aiEngineModelTesting = false;
  }

  function closeAIEngineCreate() {
    aiEngineCreateOpen = false;
    aiEngineCreateStep = 1;
  }

  function toggleAIEngineFallback(value: string) {
    aiEngineCreateFallback = aiEngineCreateFallback.includes(value)
      ? aiEngineCreateFallback.filter((item) => item !== value)
      : [...aiEngineCreateFallback, value];
  }

  function aiProviderLabel(provider: string) {
    return aiProviderOption(provider)?.label ?? provider;
  }

  function toggleAIEndpointCapability(capability: string) {
    const selected = aiEngineCreateDraft.capabilities;
    aiEngineCreateDraft = {
      ...aiEngineCreateDraft,
      capabilities: selected.includes(capability)
        ? selected.filter((item) => item !== capability)
        : [...selected, capability]
    };
  }

  function aiEngineCreateDraftIsComplete() {
    return Boolean(
      aiEngineCreateDraft.base_url.trim() &&
      aiEngineCreateDraft.model_name.trim() &&
      aiEngineCreateDraft.credential.trim()
    );
  }

  function aiEngineModelFieldMissing(field: string) {
    return aiEngineModelMissing.includes(field);
  }

  function clearAIEngineModelFieldError(field: string) {
    if (!aiEngineModelMissing.includes(field)) return;
    const value =
      field === 'provider'
        ? aiEngineCreateDraft.provider_type
        : field === 'base_url'
          ? aiEngineCreateDraft.base_url
          : field === 'model_name'
            ? aiEngineCreateDraft.model_name
            : aiEngineCreateDraft.credential;
    if (!value.trim()) return;
    aiEngineModelMissing = aiEngineModelMissing.filter(
      (item) => item !== field
    );
    if (aiEngineModelMissing.length === 0) aiEngineModelError = '';
  }

  async function addAIEndpoint() {
    if (!aiEngineCreateDraftIsComplete()) {
      const missing: string[] = [];
      if (!aiEngineCreateDraft.provider_type.trim()) missing.push('provider');
      if (!aiEngineCreateDraft.base_url.trim()) missing.push('base_url');
      if (!aiEngineCreateDraft.model_name.trim()) missing.push('model_name');
      if (!aiEngineCreateDraft.credential.trim()) missing.push('credential');
      aiEngineModelMissing = missing;
      const labels = missing.map((field) =>
        field === 'provider'
          ? '模型厂商'
          : field === 'base_url'
            ? '模型地址'
            : field === 'model_name'
              ? '模型名称'
              : '模型凭证'
      );
      aiEngineModelError = `请填写${labels.join('、')}`;
      return;
    }
    aiEngineModelTesting = true;
    aiEngineModelError = '正在测试模型连接...';
    try {
      const connection = await api.testDraftLLM({
        scope_id: selectedScopeId,
        provider_type: aiEngineCreateDraft.provider_type,
        base_url: aiEngineCreateDraft.base_url,
        model_name: aiEngineCreateDraft.model_name,
        api_key: aiEngineCreateDraft.credential,
        context_window: aiEngineCreateDraft.context_window,
        temperature: aiEngineCreateDraft.temperature,
        capabilities: aiEngineCreateDraft.capabilities,
        stream: true
      });
      addAIEndpointAfterTest(connection.latency_ms);
    } catch (error) {
      aiEngineModelError = describeError(
        error,
        '模型连接测试失败，请检查地址、凭证和模型名称'
      );
    } finally {
      aiEngineModelTesting = false;
    }
  }

  function addAIEndpointAfterTest(latencyMs: number) {
    const nextPriority =
      Math.max(
        0,
        ...aiEngineCreateEndpoints.map((endpoint) => endpoint.priority)
      ) + 10;
    const endpoint = {
      ...aiEngineCreateDraft,
      priority:
        aiEngineEditingEndpointIndex >= 0
          ? (aiEngineCreateEndpoints[aiEngineEditingEndpointIndex]?.priority ??
            nextPriority)
          : nextPriority,
      capabilities: aiEngineCreateDraft.capabilities,
      testStatus: 'succeeded' as const,
      latencyMs
    };
    if (aiEngineEditingEndpointIndex >= 0) {
      aiEngineCreateEndpoints = aiEngineCreateEndpoints.map((item, index) =>
        index === aiEngineEditingEndpointIndex ? endpoint : item
      );
    } else {
      aiEngineCreateEndpoints = [...aiEngineCreateEndpoints, endpoint];
    }
    aiEngineCreateDraft = defaultAIEndpoint();
    aiEngineEditingEndpointIndex = -1;
    aiModelCredentialVisible = false;
    aiEngineModelError = '';
    aiEngineModelMissing = [];
  }

  function removeAIEndpoint(index: number) {
    aiEngineCreateEndpoints = aiEngineCreateEndpoints.filter(
      (_, itemIndex) => itemIndex !== index
    );
    if (aiEngineEditingEndpointIndex === index) {
      aiEngineCreateDraft = defaultAIEndpoint();
      aiEngineEditingEndpointIndex = -1;
    }
  }

  function editAIEndpoint(index: number) {
    const endpoint = aiEngineCreateEndpoints[index];
    if (!endpoint) return;
    aiEngineCreateDraft = { ...endpoint, testStatus: 'idle' };
    aiEngineEditingEndpointIndex = index;
    aiModelCredentialVisible = Boolean(endpoint.credential);
    aiEngineModelError = '';
    aiEngineModelMissing = [];
  }

  function updateAIEndpoint(index: number, patch: Partial<AIEndpointDraft>) {
    const next = aiEngineCreateEndpoints.map((endpoint, itemIndex) => {
      if (itemIndex !== index) return endpoint;
      const updated = { ...endpoint, ...patch, testStatus: 'idle' as const };
      return {
        ...updated,
        capabilities: deriveAIEndpointCapabilities(updated)
      };
    });
    aiEngineCreateEndpoints = next;
    const endpoint = next[index];
    if (endpoint?.model_name.trim() && endpoint.base_url.trim()) {
      window.setTimeout(() => {
        if (
          aiEngineCreateEndpoints[index]?.model_name === endpoint.model_name &&
          aiEngineCreateEndpoints[index]?.base_url === endpoint.base_url
        ) {
          aiEngineCreateEndpoints = aiEngineCreateEndpoints.map(
            (item, itemIndex) =>
              itemIndex === index
                ? {
                    ...item,
                    testStatus: 'succeeded',
                    latencyMs: 35 + index * 8
                  }
                : item
          );
        }
      }, 250);
    }
  }

  function aiEndpointPayload(endpoint: AIEndpointDraft, credentialId = '') {
    return {
      provider_type: endpoint.provider_type,
      base_url: endpoint.base_url,
      model_name: endpoint.model_name,
      context_window: endpoint.context_window,
      temperature: endpoint.temperature,
      capabilities: endpoint.capabilities,
      timeout_seconds: endpoint.timeout_seconds,
      priority: endpoint.priority,
      enabled: endpoint.enabled,
      ...(credentialId ? { credential_id: credentialId } : {})
    };
  }

  function editAIEngine(engine: Resource) {
    selectedAIEngineId = engine.id;
    selectedAIEngineResource = engine;
    syncAIEngineEditor(engine);
    expandedAIEngineId = engine.id;
  }

  async function testAIEngine(engine: Resource) {
    const endpoints = aiEngineEndpointsFor(engine).filter(
      (endpoint) => endpoint.enabled
    );
    const first = endpoints.sort(
      (left, right) => right.priority - left.priority
    )[0];
    if (!first || !selectedScopeId) return;
    aiEngineEndpointTestBusy = true;
    try {
      const result = await api.testLLMProvider(engine.id, {
        scope_id: selectedScopeId,
        model_name: first.model_name,
        stream: true
      });
      notice = `${engine.name} 首选模型测试通过，耗时 ${result.latency_ms} ms`;
    } catch (error) {
      errorMessage = describeError(error, `${engine.name} 连接测试失败`);
    } finally {
      aiEngineEndpointTestBusy = false;
    }
  }

  async function saveAIEngine() {
    if (
      !selectedAIEngineResource ||
      !selectedScopeId ||
      !aiEngineEditName.trim()
    )
      return;
    await action(async () => {
      const currentEngine = selectedAIEngineResource as Resource;
      const endpoints = JSON.parse(aiEngineEditEndpoints) as unknown;
      if (!Array.isArray(endpoints) || endpoints.length === 0) {
        throw new Error('至少需要配置一个模型连接');
      }
      const updated = await api.updateResource(currentEngine.id, {
        name: aiEngineEditName.trim(),
        status: aiEngineEditStatus,
        config: {
          ...currentEngine.config,
          strategy: 'priority',
          fallback_on: ['timeout', 'rate_limit', 'server_error'],
          endpoints
        }
      });
      resources = resources.map((resource) =>
        resource.id === updated.id ? updated : resource
      );
      syncAIEngineEditor(updated);
      notice = `AI 引擎“${updated.name}”已更新`;
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

  async function testLLMProvider() {
    if (!selectedAIEngineId || !selectedScopeId || !llmModelName) return;
    await action(async () => {
      llmConnection = await api.testLLMProvider(selectedAIEngineId, {
        scope_id: selectedScopeId,
        model_name: llmModelName,
        stream: true
      });
      notice = `${llmConnection.message}，耗时 ${llmConnection.latency_ms} ms`;
    });
  }

  async function createAIEngine() {
    if (!selectedScopeId || !aiEngineName.trim()) return;
    await action(async () => {
      const normalizedName = aiEngineName.trim().toLocaleLowerCase();
      const duplicate = resources.some(
        (item) =>
          item.kind === 'AIEngine' &&
          item.scope_id === selectedScopeId &&
          item.name.trim().toLocaleLowerCase() === normalizedName
      );
      if (duplicate) {
        throw new Error('当前级别下已有同名 AI 引擎，请更换名称后再发布');
      }
      if (
        aiEngineCreateEndpoints.length === 0 ||
        aiEngineCreateEndpoints.some(
          (endpoint) => !endpoint.model_name.trim() || !endpoint.base_url.trim()
        )
      ) {
        throw new Error('至少需要配置一个模型连接');
      }
      const endpoints: Record<string, unknown>[] = [];
      for (const [index, endpoint] of aiEngineCreateEndpoints.entries()) {
        let credentialId = '';
        if (endpoint.credential.trim()) {
          const credential = await api.createCredential({
            scope_id: selectedScopeId,
            name: `${aiEngineName.trim()} ${endpoint.provider_type} API Token ${index + 1}`,
            purpose: 'AIEngine LLMEndpoint',
            secret: JSON.stringify({ token: endpoint.credential.trim() })
          });
          credentialId = credential.id;
        }
        endpoints.push(aiEndpointPayload(endpoint, credentialId));
      }
      const created = await api.createResource({
        scope_id: selectedScopeId,
        kind: 'AIEngine',
        name: aiEngineName.trim(),
        status: 'active',
        labels: aiEngineCreateDescription.trim()
          ? { description: aiEngineCreateDescription.trim() }
          : {},
        config: {
          strategy: aiEngineStrategy,
          fallback_on: aiEngineCreateFallback,
          icon: aiEngineIcon,
          default: false,
          endpoints
        }
      });
      const finalized = aiEngineCreateDefault
        ? await api.setAIEngineDefault(created.id, true)
        : created;
      resources = [finalized, ...resources];
      selectedAIEngineId = finalized.id;
      selectedProviderId = finalized.id;
      aiEngineName = '';
      aiEngineCreateOpen = false;
      aiEngineCreateStep = 1;
      notice = `AI 引擎“${finalized.name}”已创建`;
    });
  }

  async function toggleAIEngineDefault(engine: Resource) {
    await action(async () => {
      const updated = await api.setAIEngineDefault(engine.id, !Boolean(engine.config.default));
      resources = resources.map((item) =>
        item.kind === 'AIEngine' && item.scope_id === engine.scope_id
          ? item.id === updated.id
            ? updated
            : { ...item, config: { ...item.config, default: false } }
          : item
      );
      notice = updated.config.default ? `已将“${updated.name}”设为默认引擎` : `已取消“${updated.name}”的默认设置`;
    });
  }

  async function setLLMDefault() {
    if (!selectedAIEngineId || !selectedScopeId || !llmModelName) return;
    await action(async () => {
      await api.setLLMDefault({
        scope_id: selectedScopeId,
        provider_resource_id: selectedAIEngineId,
        model_name: llmModelName
      });
      notice = '默认模型已更新';
    });
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

  async function executeSkill() {
    if (!selectedScopeId || !selectedSkillId || !selectedSkillVersionId) return;
    await action(async () => {
      const result = await api.executeSkill({
        scope_id: selectedScopeId,
        target_resource_id: skillTargetResourceId || undefined,
        skill_resource_id: selectedSkillId,
        skill_version_id: selectedSkillVersionId,
        provider_resource_id: selectedProviderId || undefined,
        model_name: llmModelName || undefined,
        input: JSON.parse(skillInput),
        max_tool_calls: 8,
        max_tokens: 12000,
        timeout_seconds: 120
      });
      skillOutput = result.output;
      skillExecutions = [result.execution, ...skillExecutions];
      notice = 'Skill 执行完成';
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
      teamIcon = 'team';
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

  function openAIEngineIconPicker() {
    teamIconSearch = '';
    iconPickerTarget = 'ai-engine';
  }

  function selectTeamIcon(icon: string) {
    if (iconPickerTarget === 'create') teamIcon = icon;
    if (iconPickerTarget === 'edit') editTeamIcon = icon;
    if (iconPickerTarget === 'ai-engine') aiEngineIcon = icon;
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
      projectIcon = 'project';
      notice = `项目“${created.name}”已创建`;
    });
  }

  async function createResource() {
    await action(async () => {
      const config = buildSchemaConfig(
        createSchema,
        resourceConfigValues,
        resourceConfig
      );
      const credentialId = await createResourceCredential(
        createSchema,
        resourceSensitiveValues
      );
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
      resources = [created, ...resources];
      selectedResourceId = created.id;
      resourceName = '';
      resourceLabels = '';
      resourceConfig = '{}';
      resourceConfigValues = {};
      resourceSensitiveValues = {};
      notice = `资源“${created.name}”已创建`;
      resourceEditorOpen = false;
      await loadResourceDetails(created.id);
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
    if (!current || !resourceHasConnector(current)) {
      resourceConnectionChecks = { ...resourceConnectionChecks, [id]: null };
      return;
    }
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
    const connectorItems = items.filter(resourceHasConnector);
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
    resourceConnectionChecks = Object.fromEntries(checks);
  }

  async function testSelectedResourceConnection() {
    if (!selectedResource || !selectedResourceHasConnector) return;
    connectionBusy = true;
    errorMessage = '';
    try {
      connectionCheck = await api.testResourceConnection(selectedResource.id);
      resourceConnectionChecks = {
        ...resourceConnectionChecks,
        [selectedResource.id]: connectionCheck
      };
      notice =
        connectionCheck.status === 'succeeded'
          ? `资源“${selectedResource.name}”连接测试通过`
          : `资源“${selectedResource.name}”连接测试失败`;
    } catch (error) {
      errorMessage = describeError(error, '连接测试失败');
    } finally {
      connectionBusy = false;
    }
  }

  async function testResourceRowConnection(resource: Resource) {
    await loadResourceDetails(resource.id);
    await testSelectedResourceConnection();
  }

  function resourceHasConnector(resource: Resource) {
    return ['Kubernetes', 'Prometheus', 'Loki'].includes(resource.kind);
  }

  function resourceEndpointFor(resource: Resource) {
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

  async function openResourceEditor(resource: Resource) {
    await loadResourceDetails(resource.id);
    resourceEditorOpen = true;
  }

  async function updateSelectedResource() {
    if (!selectedResource) return;
    await action(async () => {
      const config = buildSchemaConfig(
        selectedSchema,
        resourceConfigValues,
        editResourceConfig
      );
      const credentialId = await createResourceCredential(
        selectedSchema,
        editResourceSensitiveValues
      );
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
  }

  function resetResourceConfig() {
    resourceConfigValues = {};
    resourceSensitiveValues = {};
    resourceConfig = '{}';
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
    if (resourceAddMenuOpen) {
      resourceAddCategory =
        resourceCategory === '全部'
          ? (Object.keys(resourceCategoryOptions).find(
              (item) => item !== '全部'
            ) ?? '')
          : resourceCategory;
      resourceAddSubtype = '';
    }
  }

  function chooseResourceAddSubtype(category: string, subtype: string) {
    const schema = resourceSchemaForSelection(category, subtype);
    resourceAddCategory = category;
    resourceAddSubtype = subtype;
    resourceKind = schema?.kind ?? '';
    resourceCategory = category;
    resourceSubtype = subtype;
    resetResourceConfig();
    resourceEditorOpen = true;
    resourceAddMenuOpen = false;
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
        return '当前级别下已有同名 AI 引擎或凭据冲突，请更换名称后重试。';
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
    'engine:manage': '设置或取消对应级别的默认 AI 引擎',
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
      ai: 'AI 引擎',
      skill: 'Skill',
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
      ai: 'AI 引擎',
      skill: 'Skill',
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
    'AI 引擎': ['统一引擎'],
    监控: ['指标', '日志', '链路', '告警']
  };

  function resourceCategoryFor(resource: {
    kind: string;
    config?: Record<string, unknown>;
    subtype?: string;
  }) {
    if (resource.kind === 'LLMProvider' || resource.kind === 'AIEngine')
      return 'AI 引擎';
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
      LLMProvider: 'OpenAI',
      AIEngine: '统一引擎',
      Prometheus: '指标',
      Loki: '日志',
      Tempo: '链路',
      Alertmanager: '告警'
    };
    const explicit = String(resource.subtype || resource.config?.subtype || '');
    if (resource.kind === 'AIEngine') {
      return '统一引擎';
    }
    return String(
      explicit ||
        resource.config?.provider ||
        labelMap[resource.kind] ||
        resource.kind
    );
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
      'AI 引擎': '✦',
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
    const key = legacyTeamIconNames[icon ?? ''] ?? icon ?? 'UsersRound';
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
      </header>
      {#if loginError}<div class="alert error" role="alert">
          {loginError}
        </div>{/if}
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
      </header>
      {#if errorMessage}<div class="alert error" role="alert">
          {errorMessage}
        </div>{/if}
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
          aria-label="AI 引擎"
          class:active={view === 'ai'}
          class="nav-item"
          on:click={() => chooseView('ai')}
          data-tooltip={sidebarCompact ? 'AI 引擎' : undefined}
          ><Sparkles size={18} strokeWidth={1.8} aria-hidden="true" /><span
            class="nav-item-label">AI 引擎</span
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

    <main class="main-content">
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
          {#if !hasPlatformRole}<div
              class="workspace-switcher topbar-workspace-switcher"
            >
              <div class="workspace-team-wrap">
                {#if teams.length > 1}
                  <button
                    class="workspace-team"
                    aria-label="切换团队"
                    aria-expanded={teamMenuOpen}
                    on:click={() => (teamMenuOpen = !teamMenuOpen)}
                  >
                    <span>{selectedTeam?.name ?? '暂无可见团队'}</span
                    ><ChevronDown
                      size={15}
                      strokeWidth={1.8}
                      aria-hidden="true"
                    />
                  </button>
                {:else}
                  <span class="workspace-team workspace-team-static"
                    >{selectedTeam?.name ?? '暂无可见团队'}</span
                  >
                {/if}
                {#if teamMenuOpen}<div class="team-menu" role="menu">
                    {#each teams as team}<button
                        role="menuitem"
                        class:selected={team.id === selectedTeamId}
                        on:click={() => chooseTeam(team.id)}>{team.name}</button
                      >{/each}
                  </div>{/if}
              </div>
              <label class="workspace-project"
                ><select
                  aria-label="切换项目"
                  value={selectedProjectId}
                  disabled={!selectedTeamProjects.length}
                  on:change={(event) =>
                    chooseProject(
                      (event.currentTarget as HTMLSelectElement).value
                    )}
                >
                  <option value="">全部项目</option>
                  {#each selectedTeamProjects as project}<option
                      value={project.id}>{project.name}</option
                    >{/each}
                </select></label
              >
            </div>{/if}
        </div>
      </header>

      {#if notice}<div class="alert success" role="status">{notice}</div>{/if}
      {#if errorMessage}<div class="alert error" role="alert">
          {errorMessage}
        </div>{/if}

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
                  <details
                    class:selected={selectedResourceId === resource.id}
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
                          ><strong>{resource.name}</strong><small
                            >{resourceEndpointFor(resource)}</small
                          ></span
                        ></span
                      >
                      <span class="resource-cell resource-category-cell"
                        ><strong>{resourceCategoryFor(resource)}</strong><small
                          >{resourceSubtypeFor(resource)}</small
                        ></span
                      >
                      <span class="resource-cell resource-scope-cell"
                        ><strong
                          class="scope-pill {scopeType(resource.scope_id)}"
                          >{resourceScopeLabel(resource)}</strong
                        ><small>管理范围</small></span
                      >
                      <span class="resource-tags" aria-label="资源标签">
                        {#each Object.entries(resource.labels ?? {}) as [key, value]}
                          <span class="resource-tag"
                            >{key}{value ? `=${value}` : ''}</span
                          >
                        {:else}
                          <small class="resource-tags-empty">未设置标签</small>
                        {/each}
                      </span>
                      <span class="resource-cell resource-connection-cell"
                        ><span
                          class="status-label {resourceConnectionClass(
                            resource
                          )}">{resourceConnectionLabel(resource)}</span
                        ><small>连接状态</small></span
                      >
                      <span class="resource-row-actions" aria-label="资源操作">
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
                  </details>
                {:else}<div class="empty-state">没有匹配的资源。</div>{/each}
              </div>
            </section>
            {#if resourceAddMenuOpen}
              <div class="resource-add-menu" role="menu">
                <div class="resource-add-menu-heading">
                  选择要添加的资源子类
                </div>
                {#each Object.entries(resourceCategoryOptions).filter(([name]) => name !== '全部' && (resourceCategory === '全部' || name === resourceCategory)) as [category, subtypes]}
                  <div class="resource-add-menu-group">
                    <strong>{category}</strong>
                    <div>
                      {#each subtypes as subtype}
                        <button
                          type="button"
                          on:click={() =>
                            chooseResourceAddSubtype(category, subtype)}
                          >{subtype}</button
                        >
                      {/each}
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
            {#if resourceEditorOpen}
              <div
                class="resource-add-dialog-backdrop"
                role="presentation"
                on:click={() => (resourceEditorOpen = false)}
              >
                <section
                  class="panel resource-add-dialog"
                  aria-labelledby="resource-add-title"
                >
                  <div class="panel-heading">
                    <div>
                      <p class="eyebrow">ADD RESOURCE</p>
                      <h2 id="resource-add-title">
                        添加{resourceAddCategory} · {resourceAddSubtype}
                      </h2>
                    </div>
                    <button
                      class="quiet-button"
                      type="button"
                      on:click={() => (resourceEditorOpen = false)}>关闭</button
                    >
                  </div>
                  <form
                    class="stack-form"
                    on:submit|preventDefault={createResource}
                  >
                    <label
                      >名称<input
                        bind:value={resourceName}
                        required
                        placeholder="例如 production-postgres"
                      /></label
                    >
                    <div class="form-row">
                      <label
                        >状态<select bind:value={resourceStatus}
                          ><option value="active">active</option><option
                            value="disabled">disabled</option
                          ><option value="unknown">unknown</option></select
                        ></label
                      ><label
                        >标签<input
                          bind:value={resourceLabels}
                          placeholder="env=prod, owner=platform"
                          autocomplete="off"
                        /></label
                      >
                    </div>
                    {#if createSchema?.schema.properties}
                      <div class="schema-inputs">
                        <p class="eyebrow">SCHEMA FIELDS</p>
                        {#each Object.entries(createSchema.schema.properties) as [key, field]}
                          <label
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
                          >
                        {/each}
                      </div>
                    {:else}
                      <label
                        >配置 JSON<textarea
                          bind:value={resourceConfig}
                          rows="4"
                          spellcheck="false"
                        ></textarea></label
                      >
                    {/if}<button
                      class="primary"
                      disabled={busy || !selectedScopeId}>创建资源</button
                    >
                  </form>
                </section>
              </div>
            {/if}
            {#if selectedResource}<section class="panel detail-panel">
                <div class="panel-heading">
                  <div>
                    <p class="eyebrow">RESOURCE DETAIL</p>
                    <h2>{selectedResource.name}</h2>
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
                {#if selectedResourceHasConnector}
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
                  <div class="form-row">
                    <label
                      >名称<input
                        bind:value={editResourceName}
                        required
                      /></label
                    ><label
                      >状态<select bind:value={editResourceStatus}
                        ><option value="active">active</option><option
                          value="disabled">disabled</option
                        ><option value="unknown">unknown</option></select
                      ></label
                    >
                  </div>
                  <label
                    >标签<input
                      bind:value={editResourceLabels}
                      placeholder="env=prod, owner=platform"
                      autocomplete="off"
                    /></label
                  >
                  {#if selectedSchema?.schema.properties}
                    <div class="schema-inputs">
                      <p class="eyebrow">SCHEMA FIELDS</p>
                      {#each Object.entries(selectedSchema.schema.properties) as [key, field]}
                        <label
                          >{field.title || key}{#if field.sensitive}<input
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
              <fieldset>
                <legend>诊断 Skill</legend>
                <div class="check-grid">
                  {#each skillResources.filter( (item) => resourceInActiveWorkspace(item) ) as skill}
                    <label class="check-row"
                      ><input
                        type="checkbox"
                        checked={inspectionSkillIds.includes(skill.id)}
                        on:change={() =>
                          (inspectionSkillIds = toggleInspectionSelection(
                            inspectionSkillIds,
                            skill.id
                          ))}
                      />{skill.name}</label
                    >
                  {/each}
                </div>
              </fieldset>
              <button
                class="primary"
                disabled={busy ||
                  !inspectionPolicyName ||
                  inspectionSkillIds.length === 0 ||
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
        <section class="content-grid diagnosis-workbench">
          <section class="panel diagnosis-start">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">READ-ONLY INVESTIGATION</p>
                <h2>开始诊断</h2>
              </div>
              <span class="scope-type">{activeScope?.name ?? 'Scope'}</span>
            </div>
            <form class="stack-form" on:submit|preventDefault={startDiagnosis}>
              <label
                >诊断问题<textarea
                  bind:value={diagnosisQuestion}
                  rows="4"
                  required
                  placeholder="例如：为什么该应用在最近一小时的错误率升高？"
                ></textarea></label
              >
              <fieldset class="diagnosis-target-picker">
                <legend>目标资源（最多 20 个）</legend>
                <p>
                  只会调用已发布 Skill 中声明的只读 Connector
                  工具；外部返回内容均按不可信证据处理。
                </p>
                <div class="diagnosis-target-list">
                  {#each diagnosisTargets as resource}
                    <label
                      class:selected={diagnosisTargetIds.includes(resource.id)}
                    >
                      <input
                        type="checkbox"
                        checked={diagnosisTargetIds.includes(resource.id)}
                        on:change={() => toggleDiagnosisTarget(resource.id)}
                      />
                      <span class="resource-kind-icon"
                        >{resourceIcon(resource.kind)}</span
                      >
                      <span
                        ><strong>{resource.name}</strong><small
                          >{resourceSchemaName(resource.kind)} · {scopeName(
                            resource.scope_id
                          )}</small
                        ></span
                      >
                    </label>
                  {:else}
                    <div class="empty-state">
                      当前作用域没有可用于诊断的活动资源。
                    </div>
                  {/each}
                </div>
              </fieldset>
              <button
                class="primary"
                disabled={busy || diagnosisTargetIds.length === 0}
                >建立诊断证据链</button
              >
            </form>
          </section>

          <section class="panel diagnosis-history">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">SESSION HISTORY</p>
                <h2>会话历史</h2>
              </div>
              <span class="count">{diagnosisSessions.length}</span>
            </div>
            <div class="table-list diagnosis-session-list">
              {#each diagnosisSessions as session}
                <button
                  class:active={selectedDiagnosisId === session.id}
                  class="list-row diagnosis-session-row"
                  on:click={() => void openDiagnosis(session.id)}
                >
                  <span
                    ><strong>{session.title || '未命名诊断'}</strong><small
                      >{formatDate(session.created_at)}</small
                    ></span
                  >
                  <span class="status-label {session.status}"
                    >{session.status}</span
                  >
                </button>
              {:else}<div class="empty-state">还没有诊断会话。</div>{/each}
            </div>
          </section>

          {#if diagnosisSnapshot}
            <section class="panel wide-panel diagnosis-timeline">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">PLAN · EXECUTE · VERIFY · SUMMARIZE</p>
                  <h2>{diagnosisSnapshot.session.title}</h2>
                </div>
                <span class="status-label {diagnosisSnapshot.session.status}"
                  >{diagnosisSnapshot.session.status}</span
                >
              </div>
              {#if diagnosisSnapshot.plan}
                <p class="plan-summary">{diagnosisSnapshot.plan.summary}</p>
                <ol class="plan-steps">
                  {#each diagnosisSnapshot.plan.steps as step}
                    <li class={step.status}>
                      <span>{step.sequence}</span>
                      <div>
                        <strong>{step.title}</strong><small>{step.detail}</small
                        >
                      </div>
                      <em>{step.phase} · {step.status}</em>
                    </li>
                  {/each}
                </ol>
              {:else}<div class="empty-state">
                  诊断正在排定受控执行计划…
                </div>{/if}
            </section>

            <section class="panel diagnosis-conversation">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">CONVERSATION</p>
                  <h2>诊断对话</h2>
                </div>
              </div>
              <div class="diagnosis-messages">
                {#each diagnosisSnapshot.messages as message}
                  <article class="diagnosis-message {message.role}">
                    <span>{message.role === 'assistant' ? 'AI' : '你'}</span>
                    <div>
                      <p>{message.content}</p>
                      <small>{formatDate(message.created_at)}</small>
                    </div>
                  </article>
                {/each}
              </div>
              <form
                class="followup-form"
                on:submit|preventDefault={sendDiagnosisFollowup}
              >
                <input
                  bind:value={diagnosisFollowup}
                  placeholder="补充问题或继续追问…"
                  maxlength="16000"
                />
                <button
                  class="primary"
                  disabled={busy || !diagnosisFollowup.trim()}>发送</button
                >
              </form>
            </section>

            <section class="panel diagnosis-evidence">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">EVIDENCE DRAWER</p>
                  <h2>证据</h2>
                </div>
                <span class="count">{diagnosisSnapshot.evidence.length}</span>
              </div>
              <div class="evidence-list">
                {#each diagnosisSnapshot.evidence as evidence}
                  <button
                    class:active={selectedEvidence?.id === evidence.id}
                    class="evidence-row"
                    on:click={() => (selectedEvidence = evidence)}
                  >
                    <span class="evidence-id">#{evidence.id.slice(0, 8)}</span
                    ><span
                      ><strong
                        >{evidence.capability || 'connector result'}</strong
                      ><small
                        >{formatDate(evidence.collected_at)} · {evidence.partial
                          ? '部分结果'
                          : '完整结果'} · 不可信输入</small
                      ></span
                    >
                  </button>
                {:else}<div class="empty-state">
                    执行完成后，Connector 返回的只读结果会作为 Evidence
                    保存到这里。
                  </div>{/each}
              </div>
              {#if selectedEvidence}
                <div class="evidence-detail">
                  <div class="detail-meta">
                    <span
                      >Hash <code>{selectedEvidence.content_hash}</code></span
                    ><span
                      >来源 {selectedEvidence.source_resource_id || '—'}</span
                    >
                  </div>
                  <pre class="config-preview">{JSON.stringify(
                      selectedEvidence.content,
                      null,
                      2
                    )}</pre>
                </div>
              {/if}
            </section>

            <section class="panel wide-panel diagnosis-report">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">TRACEABLE REPORT</p>
                  <h2>诊断报告</h2>
                </div>
                {#if diagnosisSnapshot.report}<span
                    class="status-label {diagnosisSnapshot.report.status}"
                    >{diagnosisSnapshot.report.status}</span
                  >{/if}
              </div>
              {#if diagnosisSnapshot.report}
                <p class="report-conclusion">
                  {diagnosisSnapshot.report.conclusion}
                </p>
                <div class="evidence-references">
                  <strong>Evidence 引用：</strong
                  >{#each diagnosisSnapshot.report.evidence_ids as evidenceID}<button
                      class="text-button"
                      on:click={() =>
                        (selectedEvidence =
                          diagnosisSnapshot?.evidence.find(
                            (item) => item.id === evidenceID
                          ) ?? null)}>#{evidenceID.slice(0, 8)}</button
                    >{:else}<span>无。报告中的内容仅为待验证假设。</span>{/each}
                </div>
                {#if diagnosisSnapshot.hypotheses.length}<div
                    class="hypothesis-list"
                  >
                    {#each diagnosisSnapshot.hypotheses as hypothesis}<div>
                        <span class="status-label {hypothesis.status}"
                          >{hypothesis.status}</span
                        >
                        <p>{hypothesis.statement}</p>
                      </div>{/each}
                  </div>{/if}
              {:else if diagnosisSnapshot.session.status === 'failed'}<div
                  class="alert error"
                >
                  {diagnosisSnapshot.session.error_message || '诊断执行失败。'}
                </div>{:else}<div class="empty-state">
                  正在归纳结果；没有 Evidence 引用时不会生成确定性结论。
                </div>{/if}
            </section>
          {:else}
            <section class="panel wide-panel empty-detail">
              <div class="empty-state">
                <span class="empty-icon">⌁</span>
                <h2>选择或创建诊断会话</h2>
                <p>会话将固定目标资源和作用域，并通过流式事件展示执行进度。</p>
              </div>
            </section>
          {/if}
        </section>
      {:else if view === 'ai' || view === 'skill'}
        {#if view === 'ai'}
          <section class="ai-health-page">
            <div class="ai-health-metrics">
              <div class="ai-health-metric">
                <span>可用引擎</span><strong>{aiEngineHealthyCount}</strong
                ><small>可被业务调用</small>
              </div>
              <div class="ai-health-metric">
                <span>健康 Endpoint</span><strong
                  >{aiEngineHealthyEndpointCount} / {aiEngineEndpointCount}</strong
                ><small>当前 Scope 可见成员</small>
              </div>
              <div class="ai-health-metric">
                <span>默认引擎</span><strong
                  >{aiEngines.filter((engine) => Boolean(engine.config.default))
                    .length}</strong
                ><small>平台、团队或项目</small>
              </div>
              <div class="ai-health-metric">
                <span>待处理</span><strong>{aiEngineRepairCount}</strong><small
                  >需要重新测试</small
                >
              </div>
            </div>
            {#if !aiEngineCreateOpen}
              <section class="panel ai-engine-list-panel">
                <div class="ai-engine-toolbar">
                  <div>
                    <h2>引擎列表</h2>
                    <small
                      >AI 引擎是可包含多个大模型Endpoint的统一能力入口.</small
                    >
                  </div>
                  <div class="ai-engine-toolbar-actions">
                    <div class="ai-engine-filters">
                      <input
                        class="ai-engine-search"
                        bind:value={aiEngineSearch}
                        placeholder="搜索名称、级别或用途"
                        aria-label="搜索 AI 引擎"
                      /><select
                        bind:value={aiEngineStatusFilter}
                        aria-label="引擎状态"
                        ><option value="all">全部状态</option><option
                          value="active">启用</option
                        ><option value="disabled">已停用</option></select
                      ><select
                        bind:value={aiEngineScopeFilter}
                        aria-label="引擎级别"
                        ><option value="all">全部级别</option><option
                          value="platform">平台</option
                        ><option value="team">团队</option><option
                          value="project">项目</option
                        ></select
                      >
                    </div>
                    <button
                      class="secondary"
                      type="button"
                      on:click={() => void refreshAIEngines()}
                      disabled={busy || aiEngineRefreshing}
                      ><RefreshCw
                        size={14}
                        aria-hidden="true"
                      />刷新状态</button
                    ><button
                      class="primary"
                      type="button"
                      on:click={openAIEngineCreate}
                      ><Plus size={14} aria-hidden="true" />添加 AI 引擎</button
                    >
                  </div>
                </div>
                <div class="ai-engine-list">
                  {#each visibleAIEngines as engine}
                    {@const endpoints = aiEngineEndpointsFor(engine)}
                    {@const capabilities = aiEngineCapabilities(engine)}
                    <article
                      class="ai-engine-row"
                      class:expanded={expandedAIEngineId === engine.id}
                    >
                      <div
                        class="ai-engine-row-main"
                        role="button"
                        tabindex="0"
                        on:click={() =>
                          (expandedAIEngineId =
                            expandedAIEngineId === engine.id ? '' : engine.id)}
                        on:keydown={(event) => {
                          if (event.key === 'Enter' || event.key === ' ') {
                            event.preventDefault();
                            expandedAIEngineId =
                              expandedAIEngineId === engine.id ? '' : engine.id;
                          }
                        }}
                        aria-expanded={expandedAIEngineId === engine.id}
                      >
                        <span class="ai-engine-icon"
                          ><svelte:component
                            this={teamIconComponent(aiEngineIconFor(engine))}
                            size={18}
                            aria-hidden="true"
                          /></span
                        ><span class="ai-engine-identity"
                          ><small class="ai-engine-column-label">引擎</small><strong
                            >{engine.name}</strong
                          ><small class="ai-engine-description"
                            >大模型统一能力入口</small
                          ></span
                        ><span class="ai-engine-scope"
                          ><small class="ai-engine-column-label">级别</small><b
                            class="ai-scope-badge {scopeType(engine.scope_id)}"
                            >{scopeLevelLabel(scopeType(engine.scope_id))}</b
                          ><span class="ai-engine-scope-name"
                            >{scopeName(engine.scope_id)}</span
                          ></span
                        ><span class="ai-engine-default"
                          ><small class="ai-engine-column-label">默认</small><button
                            class="ai-switch"
                            class:on={Boolean(engine.config.default)}
                            type="button"
                            role="switch"
                            aria-checked={Boolean(engine.config.default)}
                            aria-label={engine.config.default ? `取消 ${engine.name} 默认` : `设为 ${engine.name} 默认`}
                            on:click={(event) => {
                              event.stopPropagation();
                              void toggleAIEngineDefault(engine);
                            }}
                            ><span></span></button><small
                            class="ai-engine-column-hint">级别内唯一</small></span
                        ><span class="ai-engine-capabilities"
                          ><small class="ai-engine-column-label">能力</small><span
                            >{#each capabilities as capability}<em
                                >{aiCapabilityLabel(capability)}</em
                              >{:else}<em class="muted-chip">待计算</em
                              >{/each}</span><small
                            class="ai-engine-column-hint"
                            >大模型Endpoint能力交集</small
                          ></span
                        ><span class="ai-engine-health">
                          <small class="ai-engine-column-label">状态</small>
                          {#if aiEngineRefreshing}
                            <span class="ai-engine-refreshing-state">
                              <RefreshCw
                                class="ai-refreshing-icon"
                                size={15}
                                aria-label="正在刷新"
                              /><span class="sr-only">正在刷新</span>
                            </span>
                          {:else}
                            <span class="status-label {aiEngineStatusClass(engine)}">
                              {aiEngineStatusLabel(engine)} {aiEngineHealthRatio(engine)}
                            </span>
                          {/if}
                          <span class="ai-engine-health-meta">
                            {endpoints.length} 个成员 · {String(engine.config.strategy || 'priority')} 路由
                          </span>
                        </span>
                        <span
                          class="ai-engine-chevron"
                          data-tooltip={expandedAIEngineId === engine.id
                            ? '收起模型详情'
                            : '展开模型详情'}
                        >
                          <ChevronDown size={17} aria-hidden="true" />
                          <span class="sr-only">
                            {expandedAIEngineId === engine.id
                              ? '收起模型详情'
                              : '展开模型详情'}
                          </span>
                        </span>
                      </div>
                      {#if expandedAIEngineId === engine.id}
                        <div class="ai-engine-row-details">
                          <div class="ai-endpoint-list">
                            {#each endpoints as endpoint, endpointIndex}<div
                                class="ai-endpoint-row"
                              >
                                <span
                                  class="endpoint-health-dot {aiEndpointHealthClass(
                                    endpoint
                                  )}"
                                ></span><span
                                  ><strong
                                    >{endpoint.provider_type} / {endpoint.model_name}</strong
                                  ><small
                                    >优先级 {endpoint.priority} · {endpoint.context_window.toLocaleString()}
                                    上下文 · {endpoint.base_url}</small
                                  ></span
                                ><span class="ai-endpoint-caps"
                                  >{#each endpoint.capabilities as capability}<em
                                      >{aiCapabilityLabel(capability)}</em
                                    >{/each}</span
                                ><span
                                  class="status-label {aiEndpointHealthClass(
                                    endpoint
                                  )}">{aiEndpointHealthLabel(endpoint)}</span
                                >
                              </div>{/each}
                          </div>
                          <aside class="ai-engine-detail-aside">
                            <h3>路由与权限</h3>
                            <p>
                              按优先级调用。超时、限流和服务端错误允许切换；已经开始流式输出时不会切换。
                            </p>
                            <div>
                              <span>默认状态</span><strong
                                >{engine.config.default
                                  ? '当前 Scope 默认'
                                  : '未设置'}</strong
                              >
                            </div>
                            <div>
                              <span>故障切换</span><strong
                                >受限可恢复错误</strong
                              >
                            </div>
                            <div>
                              <span>凭据</span><strong
                                >加密引用，不展示明文</strong
                              >
                            </div>
                            <div class="ai-detail-actions">
                              <button
                                class="secondary"
                                type="button"
                                on:click={() => void testAIEngine(engine)}
                                disabled={aiEngineEndpointTestBusy ||
                                  endpoints.length === 0}
                                ><PlugZap
                                  size={14}
                                  aria-hidden="true"
                                />测试首选成员</button
                              ><button
                                class="quiet-button"
                                type="button"
                                on:click={() => editAIEngine(engine)}
                                ><Pencil
                                  size={14}
                                  aria-hidden="true"
                                />编辑配置</button
                              >
                            </div>
                          </aside>
                        </div>
                      {/if}
                    </article>
                  {:else}<div class="empty-state">
                      <span class="empty-icon">✦</span>
                      <h2>暂无 AI 引擎</h2>
                      <p>当前 Scope 还没有可用的统一模型入口。</p>
                    </div>{/each}
                </div>
              </section>
            {:else}
              <section
                class="panel ai-engine-create-panel"
                aria-labelledby="ai-engine-create-title"
              >
                <div class="panel-heading">
                  <div>
                    <p class="eyebrow">NEW AI ENGINE</p>
                    <h2 id="ai-engine-create-title">创建 AI 引擎</h2>
                  </div>
                  <button
                    class="quiet-button"
                    type="button"
                    on:click={closeAIEngineCreate}>关闭</button
                  >
                </div>
                <div class="ai-wizard">
                  <nav class="ai-wizard-steps" aria-label="创建步骤">
                    {#each ['基础信息', '添加模型', '路由策略', '发布总结'] as step, index}<button
                        type="button"
                        class:active={aiEngineCreateStep === index + 1}
                        class:done={aiEngineCreateStep > index + 1}
                        on:click={() => (aiEngineCreateStep = index + 1)}
                        ><b
                          >{aiEngineCreateStep > index + 1 ? '✓' : index + 1}</b
                        ><span>{step}</span></button
                      >{/each}
                  </nav>
                  <form
                    class="ai-wizard-content"
                    on:submit|preventDefault={() =>
                      aiEngineCreateStep < 4
                        ? (aiEngineCreateStep += 1)
                        : void createAIEngine()}
                  >
                    <div class="ai-step-heading">
                      <h3>
                        {aiEngineCreateStep === 1
                          ? '基础信息'
                          : aiEngineCreateStep === 2
                            ? '添加模型'
                            : aiEngineCreateStep === 3
                              ? '路由策略'
                              : '发布总结'}
                      </h3>
                      {#if aiEngineCreateStep === 2 && aiEngineModelError}
                        <span
                          class="ai-step-error"
                          class:testing={aiEngineModelTesting}
                          role="alert">{aiEngineModelError}</span
                        >
                      {/if}
                      <div class="ai-step-actions">
                        <button
                          class="secondary"
                          type="button"
                          on:click={() =>
                            aiEngineCreateStep > 1
                              ? (aiEngineCreateStep -= 1)
                              : closeAIEngineCreate()}
                          >{aiEngineCreateStep > 1 ? '上一步' : '取消'}</button
                        ><button
                          class="primary"
                          type="submit"
                          disabled={busy ||
                            (aiEngineCreateStep === 1 &&
                              !aiEngineName.trim()) ||
                            (aiEngineCreateStep === 2 &&
                              aiEngineCreateEndpoints.length === 0)}
                          >{aiEngineCreateStep < 4
                            ? '下一步'
                            : '发布 AI 引擎'}</button
                        >
                      </div>
                    </div>
                    {#if aiEngineCreateStep === 1}
                      <p>
                        定义用户大模型引擎的统一能力入口；AI
                        引擎可从多个大模型Endpoint中进行调度。
                      </p>
                      <div class="ai-form-grid">
                        <div class="ai-engine-name-field">
                          <button
                            class="team-icon-picker-trigger"
                            type="button"
                            aria-label="选择 AI 引擎图标"
                            data-tooltip="选择 AI 引擎图标"
                            on:click={openAIEngineIconPicker}
                            ><span class="entity-icon team-icon"
                              ><svelte:component
                                this={teamIconComponent(aiEngineIcon)}
                                size={16}
                                strokeWidth={1.8}
                              /></span
                            ></button
                          >
                          <label
                            ><span
                              >名称<span
                                class="required-mark"
                                aria-hidden="true">*</span
                              ></span
                            ><input
                              bind:value={aiEngineName}
                              required
                              placeholder="例如：生产诊断中枢"
                            /></label
                          >
                        </div>
                        <div class="ai-engine-scope-default-fields">
                          <label
                            ><span
                              >级别<span class="required-mark" aria-hidden="true"
                                >*</span
                              ></span
                            ><select bind:value={selectedScopeId} required
                              >{#each scopeChoices as scope}<option
                                  value={scope.id}
                                  >{scope.type === 'platform'
                                    ? scopeLevelLabel(scope.type)
                                    : `${scope.name} · ${scopeLevelLabel(scope.type)}`}</option
                                >{/each}</select
                            ></label
                          ><label class="ai-default-field"
                            ><span>默认引擎</span><button
                              class="ai-switch"
                              class:on={aiEngineCreateDefault}
                              type="button"
                              role="switch"
                              aria-checked={aiEngineCreateDefault}
                              aria-label="设置为默认 AI 引擎"
                              on:click={() => (aiEngineCreateDefault = !aiEngineCreateDefault)}
                              ><span></span></button></label>
                        </div>
                        <label class="full"
                          >用途说明<textarea
                            bind:value={aiEngineCreateDescription}
                            placeholder="例如：截图诊断、日志分析和受控 Skill"
                          ></textarea></label
                        >
                      </div>{:else if aiEngineCreateStep === 2}
                      <p>
                        先填写模型连接信息，再添加到引擎；添加后会自动计算能力并检查连接状态。
                      </p>
                      <div class="ai-model-entry-form">
                        <div class="ai-model-form-row">
                          <label
                            ><span
                              >模型厂商<span
                                class="required-mark"
                                aria-hidden="true">*</span
                              ></span
                            ><div
                              class="ai-provider-picker"
                              class:ai-field-invalid={aiEngineModelFieldMissing('provider')}
                            ><button
                                class="ai-provider-trigger"
                                type="button"
                                aria-label="选择模型厂商"
                                aria-expanded={aiProviderMenuOpen}
                                on:click={() => (aiProviderMenuOpen = !aiProviderMenuOpen)}
                                ><svelte:component this={teamIconComponent(aiProviderOption(aiEngineCreateDraft.provider_type).icon)} size={15} aria-hidden="true" /><span>{aiProviderOption(aiEngineCreateDraft.provider_type).label}</span><ChevronDown size={14} aria-hidden="true" /></button
                              >{#if aiProviderMenuOpen}<div class="ai-provider-menu" role="listbox">
                                {#each aiProviderOptions as provider}<button
                                    type="button"
                                    role="option"
                                    aria-selected={provider.value === aiEngineCreateDraft.provider_type}
                                    on:click={() => selectAIProvider(provider.value)}
                                    ><svelte:component this={teamIconComponent(provider.icon)} size={15} aria-hidden="true" /><span>{provider.label}</span></button
                                  >{/each}
                              </div>{/if}</div
                            ></label
                          ><label
                            ><span
                              >模型地址<span
                                class="required-mark"
                                aria-hidden="true">*</span
                              ></span
                            ><input
                              class:ai-field-invalid={aiEngineModelFieldMissing(
                                'base_url'
                              )}
                              aria-label="模型地址"
                              bind:value={aiEngineCreateDraft.base_url}
                              on:input={() =>
                                clearAIEngineModelFieldError('base_url')}
                              placeholder="https://api.example.com/v1"
                            /></label
                          ><label
                            ><span
                              >模型凭证<span
                                class="required-mark"
                                aria-hidden="true">*</span
                              ></span
                            ><span class="password-control"
                              ><input
                                class:ai-field-invalid={aiEngineModelFieldMissing('credential')}
                                aria-label="模型凭证"
                                type={aiModelCredentialVisible ? 'text' : 'password'}
                                bind:value={aiEngineCreateDraft.credential}
                                on:input={() => clearAIEngineModelFieldError('credential')}
                                placeholder="输入 API Token"
                              /><button
                                class="password-toggle"
                                type="button"
                                aria-label={aiModelCredentialVisible ? '隐藏模型凭证' : '显示模型凭证'}
                                on:click={() => (aiModelCredentialVisible = !aiModelCredentialVisible)}
                                >{#if aiModelCredentialVisible}<EyeOff size={15} aria-hidden="true" />{:else}<Eye size={15} aria-hidden="true" />{/if}</button
                              ></span
                            ></label
                          >
                        </div>
                        <div class="ai-model-form-row">
                          <label
                            ><span>模型名称<span class="required-mark" aria-hidden="true">*</span></span><input
                              class:ai-field-invalid={aiEngineModelFieldMissing('model_name')}
                              aria-label="模型名称"
                              bind:value={aiEngineCreateDraft.model_name}
                              on:input={() => clearAIEngineModelFieldError('model_name')}
                              placeholder="例如：gpt-4.1"
                            /></label
                          ><label
                            >温度<input
                              aria-label="温度"
                              type="number"
                              bind:value={aiEngineCreateDraft.temperature}
                              min="0"
                              max="2"
                              step="0.1"
                            /></label
                          ><label
                            >上下文窗口<input
                              aria-label="上下文窗口"
                              type="number"
                              bind:value={aiEngineCreateDraft.context_window}
                              min="1"
                            /></label
                          ><label
                            >支持的能力
                            <div class="ai-capability-card-picker">
                              {#each ['chat', 'vision', 'audio', 'tool_calling', 'structured_output', 'stream', 'long_context', 'deep_thinking'] as capability}<button
                                  class:active={aiEngineCreateDraft.capabilities.includes(
                                    capability
                                  )}
                                  type="button"
                                  on:click={() =>
                                    toggleAIEndpointCapability(capability)}
                                  >{aiCapabilityLabel(capability)}</button
                                >{/each}
                            </div></label
                          >
                          <div class="ai-model-form-actions">
                            <button
                              class="secondary"
                              type="button"
                              on:click={addAIEndpoint}
                              disabled={busy || aiEngineModelTesting}
                              ><Plus
                                size={14}
                                aria-hidden="true"
                              />{aiEngineEditingEndpointIndex >= 0
                                ? '保存模型'
                                : '添加模型'}</button
                            >
                          </div>
                        </div>
                      </div>
                      <div class="ai-model-list" aria-label="已添加模型">
                        {#if aiEngineCreateEndpoints.length === 0}<div
                            class="ai-model-empty"
                          >
                            尚未添加模型，填写上方表单后点击添加模型。
                          </div>{:else}<div
                            class="ai-model-table-head"
                            aria-hidden="true"
                          >
                            <span>模型厂商 / 地址</span><span>模型名称</span
                            ><span>能力</span><span>上下文</span><span
                              >状态</span
                            ><span>操作</span>
                          </div>
                          {#each aiEngineCreateEndpoints as endpoint, index}<div
                              class="ai-model-row"
                            >
                              <span class="ai-model-endpoint"
                                ><strong
                                  >{aiProviderLabel(
                                    endpoint.provider_type
                                  )}</strong
                                ><small>{endpoint.base_url}</small></span
                              ><span class="ai-model-name"
                                >{endpoint.model_name}</span
                              ><span class="ai-model-capabilities"
                                >{aiCapabilitiesForEndpoints([endpoint])
                                  .map(aiCapabilityLabel)
                                  .join(' · ')}</span
                              ><span class="ai-model-context"
                                >{endpoint.context_window.toLocaleString()}</span
                              ><span
                                class="ai-model-connection {aiEndpointHealthClass(
                                  endpoint
                                )}">{aiEndpointConnectionLabel(endpoint)}</span
                              ><span class="ai-model-actions"
                                ><button
                                  class="icon-button"
                                  type="button"
                                  aria-label={`编辑 ${endpoint.model_name}`}
                                  on:click={() => editAIEndpoint(index)}
                                  ><Pencil
                                    size={14}
                                    aria-hidden="true"
                                  /></button
                                ><button
                                  class="icon-button danger-action"
                                  type="button"
                                  aria-label={`删除 ${endpoint.model_name}`}
                                  on:click={() => removeAIEndpoint(index)}
                                  ><Trash2
                                    size={14}
                                    aria-hidden="true"
                                  /></button
                                ></span
                              >
                            </div>{/each}{/if}
                      </div>
                      <div class="ai-form-note">
                        当前引擎能力：{aiCapabilitiesForEndpoints(
                          aiEngineCreateEndpoints
                        )
                          .map(aiCapabilityLabel)
                          .join('、') || '添加并填写模型后自动计算'}
                      </div>{:else if aiEngineCreateStep === 3}
                      <p>
                        首版固定使用优先级路由，数字越大越先调用；仅在可恢复错误时切换。
                      </p>
                      <div class="ai-route-choice">
                        <button type="button" class="active"
                          ><strong>优先级路由</strong><small
                            >按成员优先级从高到低尝试，稳定且可解释。</small
                          ></button
                        >
                      </div>
                      <fieldset class="ai-fallback-picker">
                        <legend>故障切换条件</legend
                        >{#each [['timeout', '连接超时'], ['rate_limit', 'Provider 限流'], ['server_error', 'Provider 5xx']] as fallback}<label
                            ><input
                              type="checkbox"
                              checked={aiEngineCreateFallback.includes(
                                fallback[0]
                              )}
                              on:change={() =>
                                toggleAIEngineFallback(fallback[0])}
                            />{fallback[1]}</label
                          >{/each}
                      </fieldset>
                      <div class="ai-form-note">
                        参数错误、权限不足、能力不足、凭据错误和已开始流式输出时不会自动切换。
                      </div>{:else}
                      <p>发布前确认基础信息、模型连接和路由策略；发布后业务只感知统一的 AI 引擎入口。</p>
                      <div class="ai-review-grid">
                        <section class="ai-review-section" aria-labelledby="ai-review-basics-title">
                          <div class="ai-review-section-heading">
                            <h4 id="ai-review-basics-title">基础信息</h4>
                            <span>{scopeLevelLabel(scopeType(selectedScopeId))}</span>
                          </div>
                          <div class="ai-review-list">
                            <div><span>名称</span><strong>{aiEngineName || '未填写'}</strong></div>
                            <div><span>所属级别</span><strong>{scopeName(selectedScopeId)} · {scopeLevelLabel(scopeType(selectedScopeId))}</strong></div>
                            <div><span>用途说明</span><strong>{aiEngineCreateDescription || '未填写'}</strong></div>
                            <div><span>默认引擎</span><strong>{aiEngineCreateDefault ? '是 · 将接管该级别默认入口' : '否'}</strong></div>
                            <div><span>引擎图标</span><strong>{commonTeamIconLabels[aiEngineIcon] ?? formatIconName(aiEngineIcon)}</strong></div>
                          </div>
                        </section>

                        <section class="ai-review-section ai-review-models" aria-labelledby="ai-review-models-title">
                          <div class="ai-review-section-heading">
                            <h4 id="ai-review-models-title">模型连接</h4>
                            <span>{aiEngineCreateEndpoints.length} 个模型</span>
                          </div>
                          {#if aiEngineCreateEndpoints.length === 0}
                            <div class="ai-review-empty">尚未添加模型，返回“添加模型”步骤完成连接测试。</div>
                          {:else}
                            <div class="ai-review-endpoints">
                              {#each aiEngineCreateEndpoints as endpoint}
                                <article class="ai-review-endpoint">
                                  <div class="ai-review-endpoint-heading">
                                    <div><strong>{aiProviderLabel(endpoint.provider_type)} · {endpoint.model_name}</strong><small>{endpoint.base_url}</small></div>
                                    <span class="ai-review-status {aiEndpointHealthClass(endpoint)}">{aiEndpointConnectionLabel(endpoint)}</span>
                                  </div>
                                  <div class="ai-review-endpoint-meta">
                                    <span>优先级 {endpoint.priority}</span>
                                    <span>上下文 {endpoint.context_window.toLocaleString()}</span>
                                    <span>温度 {Number(endpoint.temperature ?? 0.7).toFixed(1)}</span>
                                    <span>{endpoint.credential.trim() ? '凭据已配置' : '未配置凭据'}</span>
                                  </div>
                                  <div class="ai-review-capabilities">
                                    {#each endpoint.capabilities.map(aiCapabilityLabel) as capability}<span>{capability}</span>{/each}
                                  </div>
                                </article>
                              {/each}
                            </div>
                          {/if}
                        </section>

                        <section class="ai-review-section" aria-labelledby="ai-review-routing-title">
                          <div class="ai-review-section-heading">
                            <h4 id="ai-review-routing-title">引擎能力与路由</h4>
                            <span>优先级路由</span>
                          </div>
                          <div class="ai-review-list">
                            <div><span>能力交集</span><strong>{aiCapabilitiesForEndpoints(aiEngineCreateEndpoints).map(aiCapabilityLabel).join('、') || '待添加模型'}</strong></div>
                            <div><span>调用顺序</span><strong>按模型优先级从高到低尝试</strong></div>
                            <div><span>故障切换</span><strong>{aiEngineCreateFallback.map((item) => item === 'timeout' ? '连接超时' : item === 'rate_limit' ? 'Provider 限流' : 'Provider 5xx').join('、') || '不自动切换'}</strong></div>
                            <div><span>不可切换</span><strong>参数、权限、凭据或能力错误；已开始流式输出</strong></div>
                          </div>
                        </section>

                        <section class="ai-review-section" aria-labelledby="ai-review-checks-title">
                          <div class="ai-review-section-heading">
                            <h4 id="ai-review-checks-title">发布检查</h4>
                            <span>发布前状态</span>
                          </div>
                          <div class="ai-review-checks">
                            <div class:passed={aiEngineCreateEndpoints.length > 0}>
                              <span>模型数量</span><strong>{aiEngineCreateEndpoints.length > 0 ? '已满足，至少 1 个模型' : '未满足，需要添加模型'}</strong>
                            </div>
                            <div class:passed={aiEngineCreateEndpoints.length > 0 && aiEngineCreateEndpoints.every((endpoint) => endpoint.testStatus === 'succeeded')}>
                              <span>连接测试</span><strong>{aiEngineCreateEndpoints.length > 0 && aiEngineCreateEndpoints.every((endpoint) => endpoint.testStatus === 'succeeded') ? '全部通过真实连接测试' : '存在未测试或失败的模型'}</strong>
                            </div>
                            <div class:passed={aiEngineCreateEndpoints.length > 0 && aiEngineCreateEndpoints.every((endpoint) => endpoint.credential.trim())}>
                              <span>模型凭据</span><strong>{aiEngineCreateEndpoints.length > 0 && aiEngineCreateEndpoints.every((endpoint) => endpoint.credential.trim()) ? '每个模型均已配置' : '存在未配置凭据的模型'}</strong>
                            </div>
                            <div class:passed={Boolean(aiEngineName.trim())}>
                              <span>名称校验</span><strong>{aiEngineName.trim() ? '已填写' : '未填写引擎名称'}</strong>
                            </div>
                          </div>
                        </section>
                      </div>
                      <div class="ai-form-note">
                        发布后可在引擎列表展开模型连接并单独刷新状态。模型凭据将保存为加密凭据，业务调用只选择引擎，不直接选择具体模型。
                      </div>{/if}
                  </form>
                </div>
              </section>
              {#if iconPickerTarget === 'ai-engine'}
                <div
                  class="dialog-backdrop"
                  role="presentation"
                  on:click={(event) => {
                    if (event.currentTarget === event.target)
                      iconPickerTarget = null;
                  }}
                >
                  <dialog
                    open
                    class="dialog icon-picker-dialog"
                    aria-labelledby="ai-engine-icon-picker-title"
                  >
                    <div class="dialog-heading">
                      <div>
                        <p class="eyebrow">ICON PICKER</p>
                        <h2 id="ai-engine-icon-picker-title">
                          选择 AI 引擎图标
                        </h2>
                      </div>
                      <button
                        class="icon-button"
                        type="button"
                        aria-label="关闭"
                        on:click={() => (iconPickerTarget = null)}>×</button
                      >
                    </div>
                    <div class="icon-picker-body">
                      <label class="icon-search"
                        ><Search size={16} aria-hidden="true" /><span
                          class="sr-only">搜索图标</span
                        ><input
                          bind:value={teamIconSearch}
                          placeholder="搜索图标，如 AI、模型、平台"
                          aria-label="搜索图标"
                        /></label
                      >
                      <div class="team-icon-grid" aria-label="AI 引擎图标列表">
                        {#each filteredTeamIconOptions as option}
                          {@const TeamIcon = teamIconComponent(option.value)}
                          <button
                            class:active={aiEngineIcon === option.value}
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
            {/if}
          </section>
        {:else}
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
                  <p>
                    仅已勾选的只读工具会暴露给模型；每个版本创建后不可修改。
                  </p>
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

            <section class="panel recent-panel">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">CONTROLLED RUNNER</p>
                  <h2>执行 Skill</h2>
                </div>
              </div>
              <form
                class="stack-form compact-form"
                on:submit|preventDefault={executeSkill}
              >
                <label
                  >目标资源<select bind:value={skillTargetResourceId}
                    ><option value="">无目标资源</option
                    >{#each executableTargets as item}<option value={item.id}
                        >{item.name} · {resourceSchemaName(item.kind)}</option
                      >{/each}</select
                  ></label
                >
                <label
                  >输入 JSON<textarea
                    bind:value={skillInput}
                    rows="5"
                    spellcheck="false"
                  ></textarea></label
                >
                <button
                  class="primary"
                  disabled={busy || !selectedSkillVersionId}>执行</button
                >
              </form>
              {#if skillOutput}<pre
                  class="config-preview">{skillOutput}</pre>{/if}
            </section>

            <section class="panel wide-panel">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">EXECUTIONS</p>
                  <h2>执行记录</h2>
                </div>
                <span class="count">{skillExecutions.length}</span>
              </div>
              <div class="table-list">
                {#each skillExecutions as execution}<div
                    class="list-row static"
                  >
                    <span
                      ><strong>{execution.model_name}</strong><small
                        >{formatDate(execution.started_at)} · {execution.total_tokens}
                        tokens · {execution.tool_call_count} tools</small
                      ></span
                    ><span class="status-label {execution.status}"
                      >{execution.status}</span
                    >
                  </div>{:else}<div class="empty-state">
                    当前 Scope 暂无执行记录。
                  </div>{/each}
              </div>
            </section>
          </section>
        {/if}
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
                          data-tooltip={permissionDescription(String(permission))}
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
                        : iconPickerTarget === 'edit'
                          ? editTeamIcon
                          : aiEngineIcon) === option.value}
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
