import type { Resource } from '../../lib/api';

export type ApplicationRuntimeKind = 'virtual_machine' | 'containerized' | 'cloud_native';

export type RuntimeResourceKind = 'Host' | 'Docker' | 'Kubernetes';

export type ApplicationResourceCreationRequest = {
  id: string;
  kind: RuntimeResourceKind;
  scopeId: string;
  instanceId?: string;
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
