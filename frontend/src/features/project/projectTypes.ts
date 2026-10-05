import type { Resource } from '../../lib/api';

export type ApplicationRuntimeKind = 'virtual_machine' | 'containerized' | 'cloud_native';

export type ApplicationInstanceDraft = {
  id: string;
  name: string;
  targetResourceId: string;
  selector: Record<string, unknown>;
};

export type ApplicationDraft = {
  name: string;
  code: string;
  description: string;
  icon: string;
  runtimeKind: ApplicationRuntimeKind;
  externalUid?: string;
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
  resources.filter((resource) => resource.kind === runtimeResourceKind(kind) && resource.status !== 'disabled');
