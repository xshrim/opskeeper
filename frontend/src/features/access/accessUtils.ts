import type {
  Group,
  ResourceRoleDefinition,
  Resource,
  RoleBinding,
  RoleDefinition
} from '../../lib/api';
import { scopeContains, type ScopeChoice } from '../../lib/scope';

export function userRoleBindings(
  userId: string,
  groups: Group[],
  groupMembers: Record<string, string[]>,
  bindings: RoleBinding[]
) {
  const groupIds = groups
    .filter((group) => groupMembers[group.id]?.includes(userId))
    .map((group) => group.id);
  return bindings.filter(
    (binding) =>
      (binding.subject_type === 'user' && binding.subject_id === userId) ||
      (binding.subject_type === 'group' && groupIds.includes(binding.subject_id))
  );
}

export function actorPermissionsAtScope(
  scopeId: string,
  currentUserId: string | undefined,
  isPlatformAdmin: boolean,
  roles: RoleDefinition[],
  groups: Group[],
  groupMembers: Record<string, string[]>,
  bindings: RoleBinding[],
  scopeChoices: ScopeChoice[]
) {
  if (!currentUserId || !scopeId) return [];
  if (isPlatformAdmin) {
    return [...new Set(roles.flatMap((role) => role.permissions.map(String)))];
  }
  const roleIds = new Set(
    userRoleBindings(currentUserId, groups, groupMembers, bindings)
      .filter((binding) => scopeContains(scopeChoices, binding.scope_id, scopeId))
      .map((binding) => binding.role_id)
  );
  return [
    ...new Set(
      roles
        .filter((role) => roleIds.has(role.id))
        .flatMap((role) => role.permissions.map(String))
    )
  ];
}

export function resourceVisibleToScope(
  scopeChoices: ScopeChoice[],
  viewerScopeId: string,
  resourceScopeId: string
) {
  if (!viewerScopeId || !resourceScopeId) return false;
  if (viewerScopeId === resourceScopeId) return true;
  return (
    scopeContains(scopeChoices, viewerScopeId, resourceScopeId) ||
    scopeContains(scopeChoices, resourceScopeId, viewerScopeId)
  );
}

export function viewerResourceRoleAllowed(resourceRole: ResourceRoleDefinition) {
  return !resourceRole.permissions.some((permission) =>
    ['resource:create', 'resource:update', 'resource:delete'].includes(String(permission))
  );
}

export function canManageResource(
  resource: Resource,
  permission: string,
  selectedScopeId: string,
  isPlatformAdmin: boolean,
  permissions: string[]
) {
  return (
    isPlatformAdmin ||
    (resource.scope_id === selectedScopeId && permissions.includes(permission))
  );
}
