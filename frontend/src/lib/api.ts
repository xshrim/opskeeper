import { appURL } from './health';

export interface User {
  id: string;
  username: string;
  email: string;
  phone: string;
  display_name: string;
  status: string;
  must_change_password: boolean;
  created_at: string;
  updated_at: string;
  can_manage?: boolean;
}

export interface UserPreferences {
  theme: 'auto' | 'light' | 'dark';
  sidebar_mode: 'fixed' | 'hover';
  sidebar_collapsed: boolean;
  avatar_updated_at?: string;
}

export interface SessionContext {
  platform_admin: boolean;
  platform_role: boolean;
}

export interface Scope {
  id: string;
  type: string;
  parent_id?: string;
  status: string;
}

export interface Platform {
  id: string;
  scope: Scope;
  name: string;
  code: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface Team {
  id: string;
  platform_id: string;
  scope: Scope;
  name: string;
  code: string;
  icon: string;
  labels: Record<string, string>;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface Project {
  id: string;
  platform_id: string;
  team_id: string;
  scope: Scope;
  name: string;
  code: string;
  icon: string;
  labels: Record<string, string>;
  source: string;
  source_resource_id?: string;
  external_uid?: string;
  source_config: Record<string, unknown>;
  last_synced_at?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface Page<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
}

export interface Resource {
  id: string;
  scope_id: string;
  kind: string;
  subtype?: string;
  schema_version: number;
  name: string;
  external_uid?: string;
  source_resource_id?: string;
  labels: Record<string, string>;
  config: Record<string, unknown>;
  status: string;
  credential_id?: string;
  created_at: string;
  updated_at: string;
}

export interface ResourceSchema {
  id: string;
  kind: string;
  version: number;
  schema: {
    properties?: Record<
      string,
      {
        title?: string;
        type?: string;
        format?: string;
        description?: string;
        enum?: string[];
        sensitive?: boolean;
        items?: unknown;
      }
    >;
  };
  status: string;
  display_name: string;
  description: string;
  icon: string;
}

export type ConnectorCapability =
  | 'kubernetes_read'
  | 'query_metrics'
  | 'query_logs'
  | 'query_traces'
  | 'get_alerts';

export interface ConnectionCheck {
  id: string;
  resource_id: string;
  status: 'succeeded' | 'failed';
  error_category?: string;
  message: string;
  latency_ms: number;
  capabilities: ConnectorCapability[];
  checked_by?: string;
  checked_at: string;
}

export interface Credential {
  id: string;
  scope_id: string;
  name: string;
  purpose: string;
  key_version: string;
  created_at: string;
  updated_at: string;
}

export interface Relation {
  id: string;
  source_resource_id: string;
  target_resource_id: string;
  relation_type: string;
  attributes: Record<string, unknown>;
  discovery_source: string;
  confidence: number;
  confirmed: boolean;
  created_at: string;
}

export interface TopologyNode {
  resource: Resource;
  depth: number;
}

export interface LLMConnectionResult {
  provider_resource_id: string;
  model_name: string;
  status: string;
  latency_ms: number;
  message: string;
}

export interface SkillVersion {
  id: string;
  skill_resource_id: string;
  version: number;
  manifest: {
    name: string;
    description: string;
    instruction: string;
    target_kinds: string[];
  };
  input_schema: Record<string, unknown>;
  output_schema: Record<string, unknown>;
  tools: Array<{
    name: string;
    description: string;
    input_schema: Record<string, unknown>;
  }>;
  risk_level: string;
  status: string;
  created_at: string;
  published_at?: string;
}

export interface SkillExecution {
  id: string;
  scope_id: string;
  target_resource_id?: string;
  skill_resource_id: string;
  skill_version_id: string;
  provider_resource_id: string;
  model_name: string;
  status: string;
  output_preview?: string;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  tool_call_count: number;
  error_code?: string;
  error_message?: string;
  started_at: string;
  completed_at?: string;
}

export interface InspectionPolicy {
  id: string;
  scope_id: string;
  name: string;
  cron: string;
  timezone: string;
  status: string;
  target_resource_ids: string[];
  target_labels: Record<string, string>;
  skill_resource_ids: string[];
  timeout: number;
  timeout_seconds?: number;
  retries: number;
  max_concurrent: number;
  max_tool_calls: number;
  max_tokens: number;
}
export interface InspectionRun {
  id: string;
  policy_id: string;
  scope_id: string;
  trigger: string;
  status: string;
  window_start: string;
  window_end: string;
  score?: number;
  deterministic_completed: boolean;
  llm_status: string;
  error_message: string;
}
export interface InspectionFinding {
  id: string;
  policy_id: string;
  target_resource_id: string;
  rule: string;
  severity: string;
  message: string;
  status: string;
  first_observed_at: string;
  last_observed_at: string;
  resolved_at?: string;
}
export interface NotificationChannel {
  id: string;
  scope_id: string;
  name: string;
  kind: string;
  webhook_url: string;
  status: string;
  rate_limit_per_minute: number;
}
export interface OperationRequest {
  id: string;
  scope_id: string;
  target_resource_id: string;
  requested_by: string;
  source: string;
  operation_name: string;
  risk_level: 'read_only' | 'low' | 'medium' | 'high';
  parameters: Record<string, unknown>;
  parameters_hash: string;
  impact_summary: string;
  rollback_summary: string;
  dry_run: Record<string, unknown>;
  idempotency_key: string;
  status: string;
  expires_at?: string;
  created_at: string;
  updated_at: string;
}
export interface OperationExecution {
  id: string;
  operation_request_id: string;
  executor: string;
  idempotency_key: string;
  status: string;
  result: Record<string, unknown>;
  error_message?: string;
  created_at: string;
}
export interface MCPSnapshot {
  id: string;
  server_resource_id: string;
  scope_id: string;
  protocol_version: string;
  server_name: string;
  server_version: string;
  content_hash: string;
  tools: Array<{
    name: string;
    description: string;
    input_schema: Record<string, unknown>;
  }>;
  status: string;
  error_message?: string;
  created_at: string;
  untrusted: true;
}

export interface SkillRunResult {
  execution: SkillExecution;
  output: string;
  events: number;
}

export type DiagnosisStatus =
  | 'queued'
  | 'planning'
  | 'collecting'
  | 'analyzing'
  | 'succeeded'
  | 'failed'
  | 'cancelled';

export interface DiagnosisSession {
  id: string;
  scope_id: string;
  actor_user_id?: string;
  status: DiagnosisStatus;
  title: string;
  error_code: string;
  error_message: string;
  started_at: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface DiagnosisTarget {
  session_id: string;
  resource_id: string;
  created_at: string;
}

export interface DiagnosisMessage {
  id: string;
  session_id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  created_at: string;
}

export interface DiagnosisPlanStep {
  id: string;
  plan_id: string;
  sequence: number;
  phase: 'plan' | 'collect' | 'verify' | 'summarize';
  status: string;
  title: string;
  detail: string;
  created_at: string;
  updated_at: string;
}

export interface DiagnosisPlan {
  id: string;
  session_id: string;
  summary: string;
  steps: DiagnosisPlanStep[];
  created_at: string;
  updated_at: string;
}

export interface DiagnosisEvidence {
  id: string;
  session_id: string;
  target_resource_id?: string;
  source_resource_id?: string;
  capability: string;
  collected_at: string;
  window_start?: string;
  window_end?: string;
  content_hash: string;
  summary: Record<string, unknown>;
  content: unknown;
  partial: boolean;
  untrusted: boolean;
  created_at: string;
}

export interface DiagnosisHypothesis {
  id: string;
  session_id: string;
  statement: string;
  status: string;
  confidence: number;
  evidence_ids: string[];
  created_at: string;
  updated_at: string;
}

export interface DiagnosisReport {
  id: string;
  session_id: string;
  status: string;
  conclusion: string;
  recommendations: string[];
  evidence_ids: string[];
  created_at: string;
}

export interface DiagnosisSnapshot {
  session: DiagnosisSession;
  targets: DiagnosisTarget[];
  messages: DiagnosisMessage[];
  plan?: DiagnosisPlan;
  evidence: DiagnosisEvidence[];
  hypotheses: DiagnosisHypothesis[];
  report?: DiagnosisReport;
}

export interface DiagnosisEvent {
  id: number;
  session_id: string;
  type: string;
  payload: Record<string, unknown>;
  created_at: string;
}

export interface DiscoveryRun {
  id: string;
  cluster_resource_id: string;
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled';
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  item_count: number;
  imported_count: number;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface DiscoveryItem {
  id: string;
  run_id: string;
  kind: string;
  namespace?: string;
  name: string;
  external_uid: string;
  resource_version?: string;
  labels: Record<string, string>;
  payload: Record<string, unknown>;
  status: 'pending' | 'imported' | 'ignored' | 'missing';
  imported_resource_id?: string;
  imported_project_id?: string;
  created_at: string;
  updated_at: string;
}

export interface DiscoveryProjectMapping {
  project_id?: string;
  team_id?: string;
  name?: string;
  code?: string;
  ignore?: boolean;
}

export interface DiscoveryImportResult {
  run: DiscoveryRun;
  imported: DiscoveryItem[];
  ignored: DiscoveryItem[];
}

export interface Group {
  id: string;
  scope_id: string;
  name: string;
  description: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface RoleDefinition {
  id: string;
  name: string;
  scope_type: string;
  builtin: boolean;
  permissions: string[];
}

export interface RoleBinding {
  id: string;
  subject_type: string;
  subject_id: string;
  role_id: string;
  role_name: string;
  scope_id: string;
  scope_type: string;
  created_at: string;
}

export interface CreateUserResult {
  user: User;
  bindings: RoleBinding[];
  one_time_password: string;
}

export interface ResourceRoleDefinition {
  id: string;
  name: string;
  builtin: boolean;
  permissions: string[];
}

export interface ResourceRoleBinding {
  id: string;
  subject_type: string;
  subject_id: string;
  role_id: string;
  role_name: string;
  resource_id: string;
  resource_name: string;
  resource_kind: string;
  scope_id: string;
  created_at: string;
}

export interface ApiErrorShape {
  code: string;
  message: string;
  request_id?: string;
}

export class ApiError extends Error {
  status: number;
  code: string;
  requestId?: string;

  constructor(status: number, detail: ApiErrorShape) {
    super(detail.message);
    this.name = 'ApiError';
    this.status = status;
    this.code = detail.code;
    this.requestId = detail.request_id;
  }
}

let refreshPromise: Promise<boolean> | null = null;

async function refreshSession(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = fetch(appURL('api/v1/auth/refresh'), {
      method: 'POST',
      credentials: 'include'
    })
      .then((response) => response.ok)
      .catch(() => false)
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
}

export async function request<T>(
  path: string,
  init: RequestInit = {},
  retry = true
): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  headers.set('Accept', 'application/json');
  const response = await fetch(appURL(path), {
    ...init,
    headers,
    credentials: 'include'
  });

  if (response.status === 401 && retry && !path.includes('/auth/')) {
    if (await refreshSession()) {
      return request<T>(path, init, false);
    }
  }

  if (!response.ok) {
    let detail: ApiErrorShape = {
      code: 'request_failed',
      message: `Request failed (${response.status})`
    };
    try {
      const body = (await response.json()) as { error?: ApiErrorShape };
      if (body.error) detail = body.error;
    } catch {
      // Keep the status-based fallback when the server did not return JSON.
    }
    throw new ApiError(response.status, detail);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

const json = (body: unknown): RequestInit => ({
  method: 'POST',
  body: JSON.stringify(body)
});
const patch = (body: unknown): RequestInit => ({
  method: 'PATCH',
  body: JSON.stringify(body)
});

export const api = {
  me: () => request<User>('api/v1/auth/me'),
  sessionContext: () => request<SessionContext>('api/v1/auth/me/context'),
  updateProfile: (body: {
    display_name: string;
    email: string;
    phone: string;
  }) => request<User>('api/v1/auth/me', patch(body)),
  changePassword: (body: { current_password?: string; new_password: string }) =>
    request<User>('api/v1/auth/me/password', json(body)),
  preferences: () => request<UserPreferences>('api/v1/auth/me/preferences'),
  updatePreferences: (body: Omit<UserPreferences, 'avatar_updated_at'>) =>
    request<UserPreferences>('api/v1/auth/me/preferences', {
      method: 'PUT',
      body: JSON.stringify(body)
    }),
  updateAvatar: (file: File) =>
    request<UserPreferences>('api/v1/auth/me/avatar', {
      method: 'PUT',
      headers: { 'Content-Type': file.type },
      body: file
    }),
  avatarURL: (updatedAt: string) =>
    appURL(
      `api/v1/auth/me/avatar?updated_at=${encodeURIComponent(updatedAt)}`
    ).toString(),
  login: (identifier: string, password: string) =>
    request<User>('api/v1/auth/login', json({ identifier, password }), false),
  logout: () => request<void>('api/v1/auth/logout', { method: 'POST' }, false),
  platform: () => request<Platform>('api/v1/platform'),
  teams: () => request<Page<Team>>('api/v1/teams/?page=1&page_size=100'),
  team: (id: string) => request<Team>(`api/v1/teams/${id}/`),
  createTeam: (body: {
    name: string;
    code: string;
    icon: string;
    labels: Record<string, string>;
  }) => request<Team>('api/v1/teams/', json(body)),
  updateTeam: (id: string, body: Record<string, unknown>) =>
    request<Team>(`api/v1/teams/${id}/`, patch(body)),
  projects: (teamId: string) =>
    request<Page<Project>>(
      `api/v1/teams/${teamId}/projects?page=1&page_size=100`
    ),
  project: (id: string) => request<Project>(`api/v1/projects/${id}/`),
  createProject: (
    teamId: string,
    body: {
      name: string;
      code: string;
      icon: string;
      labels: Record<string, string>;
      source?: string;
    }
  ) => request<Project>(`api/v1/teams/${teamId}/projects`, json(body)),
  updateProject: (id: string, body: Record<string, unknown>) =>
    request<Project>(`api/v1/projects/${id}/`, patch(body)),
  resources: (kind = '') =>
    request<Page<Resource>>(
      `api/v1/resources?page=1&page_size=100${kind ? `&kind=${encodeURIComponent(kind)}` : ''}`
    ),
  resource: (id: string) => request<Resource>(`api/v1/resources/${id}/`),
  schemas: () => request<ResourceSchema[]>('api/v1/resources/schemas'),
  credentials: () => request<Credential[]>('api/v1/credentials'),
  createCredential: (body: {
    scope_id: string;
    name: string;
    purpose: string;
    secret: string;
  }) => request<Credential>('api/v1/credentials', json(body)),
  createResource: (body: Record<string, unknown>) =>
    request<Resource>('api/v1/resources', json(body)),
  updateResource: (id: string, body: Record<string, unknown>) =>
    request<Resource>(`api/v1/resources/${id}/`, patch(body)),
  deleteResource: (id: string) =>
    request<void>(`api/v1/resources/${id}/`, { method: 'DELETE' }),
  testResourceConnection: (id: string) =>
    request<ConnectionCheck>(`api/v1/resources/${id}/connection-tests`, {
      method: 'POST'
    }),
  latestResourceConnectionCheck: (id: string) =>
    request<ConnectionCheck>(`api/v1/resources/${id}/connection-tests/latest`),
  testLLMProvider: (
    id: string,
    body: { scope_id: string; model_name: string; stream: boolean }
  ) =>
    request<LLMConnectionResult>(`api/v1/llm-providers/${id}/test`, json(body)),
  setLLMDefault: (body: {
    scope_id: string;
    provider_resource_id: string;
    model_name: string;
  }) => request('api/v1/llm-defaults', { ...json(body), method: 'PUT' }),
  skillVersions: (skillId: string) =>
    request<SkillVersion[]>(`api/v1/skills/${skillId}/versions`),
  createSkillVersion: (skillId: string, body: Record<string, unknown>) =>
    request<SkillVersion>(`api/v1/skills/${skillId}/versions`, json(body)),
  publishSkillVersion: (skillId: string, versionId: string) =>
    request<SkillVersion>(
      `api/v1/skills/${skillId}/versions/${versionId}/publish`,
      { method: 'POST' }
    ),
  setSkillDefault: (body: {
    scope_id: string;
    skill_resource_id: string;
    skill_version_id: string;
  }) => request('api/v1/skill-defaults', { ...json(body), method: 'PUT' }),
  skillExecutions: (scopeId: string) =>
    request<SkillExecution[]>(
      `api/v1/skill-executions?scope_id=${encodeURIComponent(scopeId)}&limit=50`
    ),
  executeSkill: (body: Record<string, unknown>) =>
    request<SkillRunResult>('api/v1/skill-executions', json(body)),
  diagnosisSessions: (scopeId: string) =>
    request<DiagnosisSession[]>(
      `api/v1/diagnosis-sessions?scope_id=${encodeURIComponent(scopeId)}&limit=50`
    ),
  diagnosisSession: (id: string) =>
    request<DiagnosisSnapshot>(`api/v1/diagnosis-sessions/${id}/`),
  startDiagnosis: (body: {
    scope_id: string;
    title?: string;
    question: string;
    target_resource_ids: string[];
  }) => request<DiagnosisSession>('api/v1/diagnosis-sessions', json(body)),
  addDiagnosisTarget: (sessionId: string, resourceId: string) =>
    request<DiagnosisTarget>(
      `api/v1/diagnosis-sessions/${sessionId}/targets`,
      json({ resource_id: resourceId })
    ),
  askDiagnosis: (sessionId: string, content: string) =>
    request<DiagnosisMessage>(
      `api/v1/diagnosis-sessions/${sessionId}/messages`,
      json({ content })
    ),
  diagnosisEventsURL: (sessionId: string, after = 0) =>
    appURL(
      `api/v1/diagnosis-sessions/${sessionId}/events?after=${encodeURIComponent(String(after))}`
    ),
  inspectionPolicies: (scopeId: string) =>
    request<InspectionPolicy[]>(
      `api/v1/inspection-policies?scope_id=${encodeURIComponent(scopeId)}`
    ),
  inspectionRuns: (scopeId: string) =>
    request<InspectionRun[]>(
      `api/v1/inspection-runs?scope_id=${encodeURIComponent(scopeId)}`
    ),
  inspectionFindings: (scopeId: string) =>
    request<InspectionFinding[]>(
      `api/v1/inspection-findings?scope_id=${encodeURIComponent(scopeId)}`
    ),
  startInspectionRun: (policyId: string, scopeId: string) =>
    request<{ run_id: string }>(
      `api/v1/inspection-policies/${policyId}/runs?scope_id=${encodeURIComponent(scopeId)}`,
      { method: 'POST' }
    ),
  setInspectionPolicyStatus: (
    policyId: string,
    scopeId: string,
    status: string
  ) =>
    request<void>(
      `api/v1/inspection-policies/${policyId}/status`,
      patch({ scope_id: scopeId, status })
    ),
  notificationChannels: (scopeId: string) =>
    request<NotificationChannel[]>(
      `api/v1/notification-channels?scope_id=${encodeURIComponent(scopeId)}`
    ),
  createInspectionPolicy: (body: Record<string, unknown>) =>
    request<InspectionPolicy>('api/v1/inspection-policies', json(body)),
  createNotificationChannel: (body: Record<string, unknown>) =>
    request<NotificationChannel>('api/v1/notification-channels', json(body)),
  operationRequests: (scopeId: string) =>
    request<OperationRequest[]>(
      `api/v1/operation-requests?scope_id=${encodeURIComponent(scopeId)}&limit=50`
    ),
  createOperationRequest: (body: Record<string, unknown>) =>
    request<OperationRequest>('api/v1/operation-requests', json(body)),
  approveOperation: (
    id: string,
    body: { decision: string; parameters_hash: string; comment?: string }
  ) =>
    request<OperationRequest>(
      `api/v1/operation-requests/${id}/approvals`,
      json(body)
    ),
  startOperation: (id: string, idempotencyKey: string) =>
    request<OperationExecution>(
      `api/v1/operation-requests/${id}/execute`,
      json({ idempotency_key: idempotencyKey })
    ),
  discoverMCP: (resourceId: string) =>
    request<MCPSnapshot>(`api/v1/mcp-servers/${resourceId}/discover`, {
      method: 'POST'
    }),
  mcpSnapshots: (resourceId: string) =>
    request<MCPSnapshot[]>(
      `api/v1/mcp-servers/${resourceId}/snapshots?limit=20`
    ),
  relations: (id: string) =>
    request<Relation[]>(`api/v1/resources/${id}/relations`),
  createRelation: (id: string, body: Record<string, unknown>) =>
    request<Relation>(`api/v1/resources/${id}/relations`, json(body)),
  deleteRelation: (resourceId: string, relationId: string) =>
    request<void>(`api/v1/resources/${resourceId}/relations/${relationId}`, {
      method: 'DELETE'
    }),
  topology: (id: string) =>
    request<{ items: TopologyNode[] }>(
      `api/v1/resources/${id}/topology?depth=4&max_nodes=40`
    ),
  discoveryRuns: (clusterId: string) =>
    request<DiscoveryRun[]>(`api/v1/resources/${clusterId}/discoveries`),
  startDiscovery: (clusterId: string) =>
    request<DiscoveryRun>(`api/v1/resources/${clusterId}/discoveries`, {
      method: 'POST'
    }),
  discovery: (id: string) => request<DiscoveryRun>(`api/v1/discoveries/${id}/`),
  discoveryItems: (id: string) =>
    request<DiscoveryItem[]>(`api/v1/discoveries/${id}/items`),
  importDiscovery: (
    id: string,
    body: {
      item_ids: string[];
      project_mappings: Record<string, DiscoveryProjectMapping>;
    }
  ) =>
    request<DiscoveryImportResult>(
      `api/v1/discoveries/${id}/imports`,
      json(body)
    ),
  users: () => request<User[]>('api/v1/users/'),
  createUser: (body: {
    username: string;
    email: string;
    phone: string;
    display_name: string;
    password: string;
    password_mode: 'manual' | 'generated';
    grants: Array<{
      scope_id: string;
      role_id: string;
      resource_grants: Array<{
        resource_id: string;
        role_id: string;
      }>;
    }>;
  }) => request<CreateUserResult>('api/v1/users/', json(body)),
  resetUserPassword: (id: string) =>
    request<{ one_time_password: string }>(`api/v1/users/${id}/password-reset`, {
      method: 'POST'
    }),
  updateUser: (
    id: string,
    body: {
      status?: string;
      display_name?: string;
    }
  ) => request<User>(`api/v1/users/${id}`, patch(body)),
  groups: () => request<Group[]>('api/v1/groups/'),
  groupMembers: (id: string) =>
    request<Array<{ group_id: string; user_id: string; created_at: string }>>(
      `api/v1/groups/${id}/members`
    ),
  roles: () => request<RoleDefinition[]>('api/v1/roles/'),
  bindings: () => request<RoleBinding[]>('api/v1/role-bindings/'),
  createGroup: (body: Record<string, unknown>) =>
    request<Group>('api/v1/groups/', json(body)),
  createBinding: (body: Record<string, unknown>) =>
    request<RoleBinding>('api/v1/role-bindings/', json(body)),
  deleteBinding: (id: string) =>
    request<void>(`api/v1/role-bindings/${id}`, { method: 'DELETE' }),
  resourceRoles: () =>
    request<ResourceRoleDefinition[]>('api/v1/resource-roles/'),
  resourceBindings: () =>
    request<ResourceRoleBinding[]>('api/v1/resource-role-bindings/'),
  createResourceBinding: (body: Record<string, unknown>) =>
    request<ResourceRoleBinding>('api/v1/resource-role-bindings/', json(body)),
  deleteResourceBinding: (id: string) =>
    request<void>(`api/v1/resource-role-bindings/${id}`, {
      method: 'DELETE'
    })
};
