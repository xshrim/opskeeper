import { appURL } from './health';

export interface User {
  id: string;
  email: string;
  display_name: string;
  status: string;
  created_at: string;
  updated_at: string;
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
        description?: string;
        enum?: string[];
        sensitive?: boolean;
      }
    >;
  };
  status: string;
  display_name: string;
  description: string;
  icon: string;
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
  login: (email: string, password: string) =>
    request<User>('api/v1/auth/login', json({ email, password }), false),
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
  users: () => request<User[]>('api/v1/users/'),
  groups: () => request<Group[]>('api/v1/groups/'),
  roles: () => request<RoleDefinition[]>('api/v1/roles/'),
  bindings: () => request<RoleBinding[]>('api/v1/role-bindings/'),
  createGroup: (body: Record<string, unknown>) =>
    request<Group>('api/v1/groups/', json(body)),
  createBinding: (body: Record<string, unknown>) =>
    request<RoleBinding>('api/v1/role-bindings/', json(body)),
  deleteBinding: (id: string) =>
    request<void>(`api/v1/role-bindings/${id}`, { method: 'DELETE' })
};
