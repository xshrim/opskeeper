<script lang="ts">
  import { onMount } from 'svelte';
  import { fetchHealth, toStatusRows, type HealthReport } from './lib/health';
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
    type SkillExecution,
    type SkillVersion,
    type TopologyNode,
    type User
  } from './lib/api';

  type View =
    | 'overview'
    | 'organization'
    | 'discovery'
    | 'resources'
    | 'ai'
    | 'diagnosis'
    | 'inspection'
    | 'access';
  type ProjectMappingDraft = DiscoveryProjectMapping & {
    mode: 'existing' | 'create' | 'ignore';
  };
  type ScopeChoice = {
    id: string;
    type: string;
    name: string;
    parentId?: string;
  };
  type SkillToolOption = {
    name: string;
    title: string;
    description: string;
    inputSchema: Record<string, unknown>;
  };

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
  let email = '';
  let password = '';
  let loginError = '';
  let notice = '';
  let errorMessage = '';
  let busy = false;
  let view: View = 'overview';
  let platform: Platform | null = null;
  let teams: Team[] = [];
  let projects: Project[] = [];
  let resources: Resource[] = [];
  let schemas: ResourceSchema[] = [];
  let health: HealthReport | null = null;
  let selectedScopeId = '';
  let selectedResourceId = '';
  let relations: Relation[] = [];
  let topology: TopologyNode[] = [];
  let connectionCheck: ConnectionCheck | null = null;
  let connectionBusy = false;
  let users: User[] = [];
  let groups: Group[] = [];
  let roles: RoleDefinition[] = [];
  let bindings: RoleBinding[] = [];
  let resourceRoles: ResourceRoleDefinition[] = [];
  let resourceBindings: ResourceRoleBinding[] = [];
  let aiLoaded = false;
  let selectedProviderId = '';
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

  $: scopeChoices = buildScopeChoices(platform, teams, projects);
  $: activeScope =
    scopeChoices.find((scope) => scope.id === selectedScopeId) ??
    scopeChoices[0];
  $: visibleProjects = selectedScopeId
    ? projects.filter(
        (project) =>
          project.scope.id === selectedScopeId ||
          project.team_id === selectedScopeId
      )
    : projects;
  $: visibleResources = selectedScopeId
    ? resources.filter((resource) => resource.scope_id === selectedScopeId)
    : resources;
  $: selectedResource =
    resources.find((resource) => resource.id === selectedResourceId) ?? null;
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
  $: llmProviders = resources.filter((item) => item.kind === 'LLMProvider');
  $: skillResources = resources.filter((item) => item.kind === 'Skill');
  $: executableTargets = visibleResources.filter(
    (item) => item.kind !== 'LLMProvider' && item.kind !== 'Skill'
  );
  $: diagnosisTargets = visibleResources.filter(
    (item) => item.status === 'active'
  );

  onMount(() => {
    void bootstrap();
    const controller = new AbortController();
    const checkHealth = async () => {
      try {
        health = await fetchHealth(controller.signal);
      } catch {
        health = null;
      }
    };
    void checkHealth();
    const interval = window.setInterval(checkHealth, 15_000);
    return () => {
      controller.abort();
      window.clearInterval(interval);
      closeDiagnosisEvents();
    };
  });

  async function bootstrap() {
    try {
      currentUser = await api.me();
      authState = 'ready';
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
      const projectPages = await Promise.all(
        teams.map((team) => api.projects(team.id))
      );
      projects = projectPages.flatMap((page) => page.items);
      selectedScopeId = selectedScopeId || platform.scope.id;
    } catch (error) {
      errorMessage = describeError(error, '工作区数据加载失败');
    }
  }

  async function login() {
    busy = true;
    loginError = '';
    try {
      currentUser = await api.login(email.trim(), password);
      password = '';
      authState = 'ready';
      await loadWorkspace();
    } catch (error) {
      loginError = describeError(error, '登录失败，请检查邮箱和密码');
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
      authState = 'login';
      view = 'overview';
      busy = false;
    }
  }

  function chooseView(nextView: View) {
    view = nextView;
    notice = '';
    errorMessage = '';
    if (nextView === 'access' && !accessLoaded) void loadAccess();
    if (nextView === 'discovery' && !discoveryLoaded) void loadDiscovery();
    if (nextView === 'ai' && !aiLoaded) void loadAI();
    if (nextView === 'diagnosis' && !diagnosisLoaded) void loadDiagnosis();
	if (nextView === 'inspection' && !inspectionLoaded) void loadInspection();
    if (nextView !== 'diagnosis') closeDiagnosisEvents();
  }

	async function loadInspection() {
		if (!selectedScopeId) return;
		inspectionLoaded = true;
		try { [inspectionPolicies, inspectionRuns, inspectionFindings, notificationChannels] = await Promise.all([api.inspectionPolicies(selectedScopeId), api.inspectionRuns(selectedScopeId), api.inspectionFindings(selectedScopeId), api.notificationChannels(selectedScopeId)]); }
		catch (error) { errorMessage = describeError(error, '巡检数据加载失败'); }
	}
	async function rerunInspection(policyID: string) { busy=true; try { await api.startInspectionRun(policyID, selectedScopeId); notice='已创建手动巡检任务。'; await loadInspection(); } catch (error) { errorMessage=describeError(error,'创建巡检任务失败'); } finally { busy=false; } }
	async function setInspectionPolicyStatus(policyID: string, status: string) { busy=true; try { await api.setInspectionPolicyStatus(policyID,selectedScopeId,status); notice=status==='disabled'?'已停止周期巡检。':'已恢复周期巡检。'; await loadInspection(); } catch (error) { errorMessage=describeError(error,'更新巡检策略失败'); } finally { busy=false; } }

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
    selectedProviderId = selectedProviderId || llmProviders[0]?.id || '';
    selectedSkillId = selectedSkillId || skillResources[0]?.id || '';
    llmModelName =
      llmModelName ||
      String(
        (
          llmProviders.find((item) => item.id === selectedProviderId)?.config
            .models as Array<{ name?: string }> | undefined
        )?.[0]?.name || ''
      );
    if (selectedSkillId) await loadSkillVersions();
    if (selectedScopeId) {
      try {
        skillExecutions = await api.skillExecutions(selectedScopeId);
      } catch {
        skillExecutions = [];
      }
    }
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
    if (!selectedProviderId || !selectedScopeId || !llmModelName) return;
    await action(async () => {
      llmConnection = await api.testLLMProvider(selectedProviderId, {
        scope_id: selectedScopeId,
        model_name: llmModelName,
        stream: true
      });
      notice = `${llmConnection.message}，耗时 ${llmConnection.latency_ms} ms`;
    });
  }

  async function setLLMDefault() {
    if (!selectedProviderId || !selectedScopeId || !llmModelName) return;
    await action(async () => {
      await api.setLLMDefault({
        scope_id: selectedScopeId,
        provider_resource_id: selectedProviderId,
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
      if (result.every((item) => item.status === 'rejected')) {
        errorMessage = '当前账号没有成员与角色管理权限。';
      }
    } catch {
      errorMessage = '成员和角色数据加载失败。';
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
    if (
      !current ||
      !['Kubernetes', 'Prometheus', 'Loki'].includes(current.kind)
    )
      return;
    try {
      const check = await api.latestResourceConnectionCheck(id);
      if (selectedResourceId === id) connectionCheck = check;
    } catch (error) {
      if (error instanceof ApiError && error.status === 404) return;
      if (selectedResourceId === id)
        errorMessage = describeError(error, '连接状态加载失败');
    }
  }

  async function testSelectedResourceConnection() {
    if (!selectedResource || !selectedResourceHasConnector) return;
    connectionBusy = true;
    errorMessage = '';
    try {
      connectionCheck = await api.testResourceConnection(selectedResource.id);
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
      return error.message || fallback;
    }
    if (error instanceof SyntaxError) return '配置必须是有效的 JSON 对象。';
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
        parentId: team.platform_id
      });
    for (const project of currentProjects)
      choices.push({
        id: project.scope.id,
        type: 'project',
        name: project.name,
        parentId: project.team_id
      });
    return choices;
  }

  function scopeName(id: string) {
    return (
      scopeChoices.find((scope) => scope.id === id)?.name ?? id.slice(0, 8)
    );
  }

  function scopeType(id: string) {
    return scopeChoices.find((scope) => scope.id === id)?.type ?? 'scope';
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

  function iconGlyph(icon: string | undefined) {
    const glyphs: Record<string, string> = {
      platform: '▣',
      team: '♟',
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
</script>

<svelte:head>
  <meta name="description" content="OpsKeeper platform control plane" />
</svelte:head>

{#if authState === 'loading'}
  <div class="loading-screen">
    <span class="spinner"></span>
    <p>正在恢复工作区会话…</p>
  </div>
{:else if authState === 'login'}
  <main class="login-shell">
    <section class="login-panel" aria-labelledby="login-heading">
      <div class="brand large">
        <span class="brand-mark" aria-hidden="true">O</span><span
          >OpsKeeper</span
        >
      </div>
      <p class="eyebrow">CONTROL PLANE</p>
      <h1 id="login-heading">登录管理控制台</h1>
      <p class="muted">使用本地账号访问组织、权限和资源工作区。</p>
      {#if loginError}<div class="alert error" role="alert">
          {loginError}
        </div>{/if}
      <form class="stack-form" on:submit|preventDefault={login}>
        <label
          >邮箱<input
            type="email"
            bind:value={email}
            autocomplete="username"
            required
            placeholder="admin@example.com"
          /></label
        >
        <label
          >密码<input
            type="password"
            bind:value={password}
            autocomplete="current-password"
            required
          /></label
        >
        <button class="primary full" type="submit" disabled={busy}
          >{busy ? '登录中…' : '登录'}</button
        >
      </form>
    </section>
  </main>
{:else}
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark" aria-hidden="true">O</span><span
          >OpsKeeper</span
        >
      </div>
      <div class="workspace-label">WORKSPACE</div>
      <nav aria-label="主导航">
        <button
          class:active={view === 'overview'}
          class="nav-item"
          on:click={() => chooseView('overview')}
          ><span aria-hidden="true">⌂</span>总览</button
        >
        <button
          class:active={view === 'organization'}
          class="nav-item"
          on:click={() => chooseView('organization')}
          ><span aria-hidden="true">▦</span>组织</button
        >
        <button
          class:active={view === 'resources'}
          class="nav-item"
          on:click={() => chooseView('resources')}
          ><span aria-hidden="true">◇</span>资源</button
        >
        <button
          class:active={view === 'discovery'}
          class="nav-item"
          on:click={() => chooseView('discovery')}
          ><span aria-hidden="true">☸</span>集群导入</button
        >
        <button
          class:active={view === 'ai'}
          class="nav-item"
          on:click={() => chooseView('ai')}
          ><span aria-hidden="true">✦</span>模型与 Skill</button
        >
        <button
          class:active={view === 'diagnosis'}
          class="nav-item"
          on:click={() => chooseView('diagnosis')}
          ><span aria-hidden="true">⌁</span>AI 诊断</button
        >
		<button class:active={view === 'inspection'} class="nav-item" on:click={() => chooseView('inspection')}><span aria-hidden="true">◴</span>自动巡检</button>
        <button
          class:active={view === 'access'}
          class="nav-item"
          on:click={() => chooseView('access')}
          ><span aria-hidden="true">♙</span>成员与角色</button
        >
      </nav>
      <div class="sidebar-footer">
        <span class="status-dot"></span><span
          >{health?.status === 'ready' ? '服务正常' : '检查服务状态'}</span
        >
      </div>
    </aside>

    <main class="main-content">
      <header class="topbar">
        <div>
          <p class="breadcrumb">
            {activeScope?.name ?? 'Platform'} / {view === 'overview'
              ? 'Overview'
              : view === 'organization'
                ? 'Organization'
                : view === 'resources'
                  ? 'Resources'
                  : view === 'discovery'
                    ? 'Kubernetes Import'
                    : view === 'ai'
                      ? 'AI Runtime'
                      : view === 'diagnosis'
                        ? 'AI Diagnosis'
						: view === 'inspection'
							? 'Inspection'
                        : 'Access'}
          </p>
          <h1>
            {view === 'overview'
              ? '平台总览'
              : view === 'organization'
                ? '组织管理'
                : view === 'resources'
                  ? '资源目录'
                  : view === 'discovery'
                    ? '集群项目与应用导入'
                    : view === 'ai'
                      ? '模型与 Skill'
                      : view === 'diagnosis'
                        ? 'AI 诊断工作台'
						: view === 'inspection'
							? '自动巡检与健康'
                        : '成员与角色'}
          </h1>
        </div>
        <div class="topbar-actions">
          <span class="user-chip"
            ><span class="avatar"
              >{(currentUser?.display_name || currentUser?.email || 'U')
                .slice(0, 1)
                .toUpperCase()}</span
            ><span>{currentUser?.display_name || currentUser?.email}</span
            ></span
          ><button class="quiet-button" on:click={logout} disabled={busy}
            >退出</button
          >
        </div>
      </header>

      <div class="scope-bar">
        <label class="scope-picker"
          ><span>当前作用域</span><select
            bind:value={selectedScopeId}
            on:change={() => {
              selectedResourceId = '';
              if (view === 'diagnosis') {
                selectedDiagnosisId = '';
                diagnosisSnapshot = null;
                void loadDiagnosisSessions();
              }
            }}
            ><option value="" disabled>选择作用域</option
            >{#each scopeChoices as scope}<option value={scope.id}
                >{scope.type === 'platform'
                  ? '平台'
                  : scope.type === 'team'
                    ? '团队'
                    : '项目'} · {scope.name}</option
              >{/each}</select
          ></label
        >
        <div class="scope-trail">
          <span class="scope-type">{activeScope?.type ?? 'scope'}</span><span
            >{activeScope?.name ?? '未选择'}</span
          >
        </div>
      </div>

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
              {#each teams as team}<button
                  class:selected={selectedScopeId === team.scope.id}
                  class="list-row"
                  on:click={() => {
                    selectedScopeId = team.scope.id;
                    projectTeamId = team.id;
                  }}
                  ><span class="entity-summary"
                    ><span class="entity-icon team-icon"
                      >{iconGlyph(team.icon)}</span
                    ><span
                      ><strong>{team.name}</strong><small
                        >{team.code} · {team.status}</small
                      ></span
                    ></span
                  ><span class="row-arrow">→</span></button
                >{:else}<div class="empty-state">暂无团队</div>{/each}
            </div>
            <form class="inline-form" on:submit|preventDefault={createTeam}>
              <input
                bind:value={teamIcon}
                placeholder="图标，如 team 或 ♟"
                aria-label="团队图标"
              />
              <input
                bind:value={teamName}
                required
                placeholder="团队名称"
                aria-label="团队名称"
              /><input
                bind:value={teamCode}
                required
                placeholder="编码"
                aria-label="团队编码"
              /><button class="primary" disabled={busy}>新增团队</button>
            </form>
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
            <div class="panel-heading">
              <div>
                <p class="eyebrow">CATALOG</p>
                <h2>资源目录</h2>
              </div>
              <span class="count">{visibleResources.length}</span>
            </div>
            <div class="filter-row">
              <select bind:value={resourceKind}
                ><option value="">全部类型</option
                >{#each schemas as schema}<option value={schema.kind}
                    >{schemaName(schema)}</option
                  >{/each}</select
              >
            </div>
            <div class="table-list resource-list">
              {#each visibleResources.filter((item) => !resourceKind || item.kind === resourceKind) as resource}<button
                  class:selected={selectedResourceId === resource.id}
                  class="list-row"
                  on:click={() => void loadResourceDetails(resource.id)}
                  ><span class="entity-summary"
                    ><span class="entity-icon resource-icon"
                      >{iconGlyph(
                        schemas.find((schema) => schema.kind === resource.kind)
                          ?.icon
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
                >{:else}<div class="empty-state">没有匹配的资源。</div>{/each}
            </div>
          </section>
          <section class="resource-workspace">
            <section class="panel">
              <div class="panel-heading">
                <div>
                  <p class="eyebrow">NEW RESOURCE</p>
                  <h2>登记资源</h2>
                </div>
                <span class="scope-type">{activeScope?.type ?? 'scope'}</span>
              </div>
              <form
                class="stack-form"
                on:submit|preventDefault={createResource}
              >
                <div class="resource-type-picker" aria-label="选择资源类型">
                  <div class="resource-type-grid">
                    {#each schemas as schema}
                      <button
                        type="button"
                        class="resource-type-card"
                        class:selected={resourceKind === schema.kind}
                        on:click={() => {
                          resourceKind = schema.kind;
                          resetResourceConfig();
                        }}
                      >
                        <span class="type-icon">{iconGlyph(schema.icon)}</span>
                        <span>
                          <strong>{schemaName(schema)}</strong>
                          <small
                            >{schema.description || '资源连接与运行信息'}</small
                          >
                        </span>
                      </button>
                    {/each}
                  </div>
                </div>
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
                {/if}<button class="primary" disabled={busy || !selectedScopeId}
                  >创建资源</button
                >
              </form>
            </section>
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
                    disabled={busy}>停用</button
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
                  <button class="secondary" disabled={busy}>保存修改</button>
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
                            title="删除关系"
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
			<section class="panel"><div class="panel-heading"><div><p class="eyebrow">POLICIES</p><h2>巡检策略</h2></div><span class="count">{inspectionPolicies.length}</span></div>
				<div class="table-list">{#each inspectionPolicies as policy}<article class="list-row"><div><strong>{policy.name}</strong><p>{policy.cron} · {policy.timezone} · {policy.target_resource_ids.length} 个目标 · {policy.status}</p></div><div class="inline-actions"><button class="quiet-button" disabled={busy || policy.status !== 'active'} on:click={() => rerunInspection(policy.id)}>立即运行</button><button class="quiet-button" disabled={busy} on:click={() => setInspectionPolicyStatus(policy.id, policy.status === 'active' ? 'disabled' : 'active')}>{policy.status === 'active' ? '停止' : '恢复'}</button></div></article>{:else}<p class="empty-state">当前作用域还没有巡检策略。</p>{/each}</div></section>
			<section class="panel"><div class="panel-heading"><div><p class="eyebrow">HEALTH</p><h2>最近运行</h2></div><span class="count">{inspectionRuns.length}</span></div><div class="table-list">{#each inspectionRuns as run}<article class="list-row"><div><strong>{run.score ?? '—'} 分 · {run.status}</strong><p>{new Date(run.window_start).toLocaleString()} · LLM {run.llm_status}</p></div></article>{:else}<p class="empty-state">尚无运行记录。</p>{/each}</div></section>
			<section class="panel wide-panel"><div class="panel-heading"><div><p class="eyebrow">FINDINGS</p><h2>异常与恢复</h2></div><span class="count">{inspectionFindings.length}</span></div><div class="table-list">{#each inspectionFindings as finding}<article class="list-row"><div><strong>{finding.severity} · {finding.rule}</strong><p>{finding.message || '无补充说明'} · {finding.status}</p></div></article>{:else}<p class="empty-state">没有已记录的异常。</p>{/each}</div></section>
			<section class="panel"><div class="panel-heading"><div><p class="eyebrow">WEBHOOKS</p><h2>通知渠道</h2></div><span class="count">{notificationChannels.length}</span></div><div class="table-list">{#each notificationChannels as channel}<article class="list-row"><div><strong>{channel.name}</strong><p>{channel.kind} · {channel.status} · 每分钟 {channel.rate_limit_per_minute} 次</p></div></article>{:else}<p class="empty-state">当前作用域没有启用的通知渠道。</p>{/each}</div></section>
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
      {:else if view === 'ai'}
        <section class="content-grid two-column ai-runtime">
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">MODEL PROVIDER</p>
                <h2>模型连接</h2>
              </div>
              <span class="count">{llmProviders.length}</span>
            </div>
            <form
              class="stack-form compact-form"
              on:submit|preventDefault={testLLMProvider}
            >
              <label
                >Provider<select
                  bind:value={selectedProviderId}
                  required
                  on:change={() => {
                    const models = llmProviders.find(
                      (item) => item.id === selectedProviderId
                    )?.config.models as Array<{ name?: string }> | undefined;
                    llmModelName = models?.[0]?.name || '';
                    llmConnection = null;
                  }}
                  ><option value="" disabled>选择 LLM Provider</option
                  >{#each llmProviders as provider}<option value={provider.id}
                      >{provider.name} · {scopeName(provider.scope_id)}</option
                    >{/each}</select
                ></label
              >
              <label
                >Model<input
                  bind:value={llmModelName}
                  required
                  placeholder="模型配置中的名称"
                /></label
              >
              <div class="form-actions">
                <button
                  class="secondary"
                  type="button"
                  on:click={setLLMDefault}
                  disabled={busy || !selectedScopeId}
                  >设为当前 Scope 默认</button
                >
                <button class="primary" disabled={busy || !selectedProviderId}
                  >测试连接</button
                >
              </div>
            </form>
            {#if llmConnection}<div class="detail-meta">
                <span>状态 <strong>{llmConnection.status}</strong></span><span
                  >{llmConnection.latency_ms} ms</span
                ><span>{llmConnection.model_name}</span>
              </div>{/if}
          </section>

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
              <button class="primary" disabled={busy || !selectedSkillVersionId}
                >执行</button
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
              {#each skillExecutions as execution}<div class="list-row static">
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
      {:else}
        <section class="content-grid two-column">
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">PEOPLE</p>
                <h2>用户</h2>
              </div>
              <span class="count">{users.length}</span>
            </div>
            <div class="table-list">
              {#each users as user}<div class="list-row static">
                  <span
                    ><strong>{user.display_name || user.email}</strong><small
                      >{user.email}</small
                    ></span
                  ><span class="status-label {user.status}">{user.status}</span>
                </div>{:else}<div class="empty-state">
                  没有可见用户，或当前账号没有管理权限。
                </div>{/each}
            </div>
          </section>
          <section class="panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">GROUPS</p>
                <h2>成员组</h2>
              </div>
              <span class="count">{groups.length}</span>
            </div>
            <div class="table-list">
              {#each groups as group}<div class="list-row static">
                  <span
                    ><strong>{group.name}</strong><small
                      >{group.description || '无描述'} · {scopeName(
                        group.scope_id
                      )}</small
                    ></span
                  ><span class="status-label {group.status}"
                    >{group.status}</span
                  >
                </div>{:else}<div class="empty-state">暂无成员组</div>{/each}
            </div>
            <form
              class="stack-form compact-form"
              on:submit|preventDefault={createGroup}
            >
              <label
                >作用域<select bind:value={groupScopeId} required
                  ><option value="" disabled>选择作用域</option
                  >{#each scopeChoices as scope}<option value={scope.id}
                      >{scope.name}</option
                    >{/each}</select
                ></label
              ><input
                bind:value={groupName}
                required
                placeholder="成员组名称"
              /><input
                bind:value={groupDescription}
                placeholder="描述"
              /><button class="primary" disabled={busy}>新增成员组</button>
            </form>
          </section>
          <section class="panel wide-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">AUTHORIZATION</p>
                <h2>角色绑定</h2>
              </div>
              <span class="count">{bindings.length}</span>
            </div>
            <div class="table-list">
              {#each bindings as binding}<div class="list-row static">
                  <span
                    ><strong>{binding.role_name}</strong><small
                      >{binding.subject_type} · {binding.subject_id.slice(0, 8)} ·
                      {scopeName(binding.scope_id)}</small
                    ></span
                  ><button
                    class="icon-button"
                    title="删除绑定"
                    aria-label="删除绑定"
                    on:click={() => deleteBinding(binding)}>×</button
                  >
                </div>{:else}<div class="empty-state">
                  暂无可见角色绑定
                </div>{/each}
            </div>
            <form class="binding-form" on:submit|preventDefault={createBinding}>
              <select bind:value={bindingSubjectType}
                ><option value="user">用户</option><option value="group"
                  >成员组</option
                ></select
              ><input
                bind:value={bindingSubjectId}
                required
                placeholder="主体 ID"
                aria-label="主体 ID"
              /><select bind:value={bindingRoleId} required
                ><option value="" disabled>角色</option
                >{#each roles as role}<option value={role.id}
                    >{role.name} · {role.scope_type}</option
                  >{/each}</select
              ><select bind:value={bindingScopeId} required
                ><option value="" disabled>作用域</option
                >{#each scopeChoices as scope}<option value={scope.id}
                    >{scope.name}</option
                  >{/each}</select
              ><button class="secondary" disabled={busy}>绑定角色</button>
            </form>
          </section>
          <section class="panel wide-panel">
            <div class="panel-heading">
              <div>
                <p class="eyebrow">RESOURCE ACCESS</p>
                <h2>具体资源授权</h2>
                <p class="muted">
                  在可进入项目的成员中，进一步限定可操作的具体资源。
                </p>
              </div>
              <span class="count">{resourceBindings.length}</span>
            </div>
            <div class="table-list">
              {#each resourceBindings as binding}<div class="list-row static">
                  <span
                    ><strong
                      >{binding.role_name} · {binding.resource_name}</strong
                    ><small
                      >{binding.subject_type} · {binding.subject_id.slice(0, 8)} ·
                      {resourceSchemaName(binding.resource_kind)} · {scopeName(
                        binding.scope_id
                      )}</small
                    ></span
                  ><button
                    class="icon-button"
                    title="删除资源授权"
                    aria-label="删除资源授权"
                    on:click={() => deleteResourceBinding(binding)}>×</button
                  >
                </div>{:else}<div class="empty-state">
                  暂无具体资源授权；Scope 角色仍按原有规则覆盖整个范围。
                </div>{/each}
            </div>
            <form
              class="binding-form resource-binding-form"
              on:submit|preventDefault={createResourceBinding}
            >
              <select bind:value={resourceBindingSubjectType}
                ><option value="user">用户</option><option value="group"
                  >成员组</option
                ></select
              ><input
                bind:value={resourceBindingSubjectId}
                required
                placeholder="主体 ID"
                aria-label="主体 ID"
              /><select bind:value={resourceBindingRoleId} required
                ><option value="" disabled>资源角色</option
                >{#each resourceRoles as role}<option value={role.id}
                    >{role.name}</option
                  >{/each}</select
              ><select bind:value={resourceBindingResourceId} required
                ><option value="" disabled>具体资源</option
                >{#each resources as resource}<option value={resource.id}
                    >{resource.name} · {resourceSchemaName(resource.kind)} · {scopeName(
                      resource.scope_id
                    )}</option
                  >{/each}</select
              ><button class="secondary" disabled={busy}>绑定资源角色</button>
            </form>
          </section>
        </section>
      {/if}
    </main>
  </div>
{/if}
