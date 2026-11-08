import type { Resource } from '../../lib/api';

export type ApplicationRuntimeKind = 'virtual_machine' | 'containerized' | 'cloud_native';

export type RuntimeResourceKind = 'Host' | 'Docker' | 'Kubernetes';

export type ApplicationResourceCreationRequest = {
  id: string;
  kind: string;
  scopeId: string;
  instanceId?: string;
  coverageTarget?: string;
};

export type ApplicationResourceCreationResult = {
  requestId: string;
  instanceId?: string;
  resource: Resource;
};

export type ApplicationInstanceDraft = {
  id: string;
  persistedId?: string;
  name: string;
  targetResourceId: string;
  selector: Record<string, unknown>;
  logBinding?: Record<string, unknown>;
  status?: string;
};

export type ApplicationDraft = {
  name: string;
  code: string;
  description: string;
  icon: string;
  labels: Record<string, string>;
  runtimeKind: ApplicationRuntimeKind;
  externalUid?: string;
  critical: boolean;
  technicalOwnerIds: string[];
  operationsOwnerIds: string[];
  dependencies?: Array<{ dependency_kind: string; target_resource_id: string; binding?: Record<string, string>; required?: boolean }>;
  relations?: Array<{ target_application_id: string; relation_kind: 'depends_on'; status?: string }>;
  instances: ApplicationInstanceDraft[];
};

export const runtimeLabel = (kind: ApplicationRuntimeKind) =>
  kind === 'virtual_machine' ? 'Host' : kind === 'containerized' ? 'Docker' : 'Kubernetes';

export const runtimeResourceKind = (kind: ApplicationRuntimeKind) =>
  runtimeLabel(kind);

export const slugify = (value: string) => {
  const slug = value.trim().toLocaleLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
  return slug || `application-${Date.now()}`;
};

export const resourcesForRuntime = (resources: Resource[], kind: ApplicationRuntimeKind) =>
  resources.filter((resource) => resource.kind === runtimeResourceKind(kind));
