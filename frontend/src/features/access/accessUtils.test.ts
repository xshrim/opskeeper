import { describe, expect, it } from 'vitest';
import { actorPermissionsAtScope, canManageResource, resourceVisibleToScope, userRoleBindings } from './accessUtils';

const scopes = [
  { id: 'platform', type: 'platform', name: '平台' },
  { id: 'team', type: 'team', name: '团队', parentId: 'platform' },
  { id: 'project', type: 'project', name: '项目', parentId: 'team' }
];

describe('access helpers', () => {
  it('includes direct and group role bindings without duplicates', () => {
    const groups = [{ id: 'group-1' }] as never[];
    const bindings = [
      { subject_type: 'user', subject_id: 'user-1', role_id: 'viewer' },
      { subject_type: 'group', subject_id: 'group-1', role_id: 'operator' }
    ] as never[];
    expect(userRoleBindings('user-1', groups, { 'group-1': ['user-1'] }, bindings)).toHaveLength(2);
  });

  it('inherits permissions from ancestor scopes', () => {
    const roles = [
      { id: 'team-admin', permissions: ['resource:read'] }
    ] as never[];
    const bindings = [{ subject_type: 'user', subject_id: 'user-1', role_id: 'team-admin', scope_id: 'team' }] as never[];
    expect(actorPermissionsAtScope('project', 'user-1', false, roles, [], {}, bindings, scopes)).toEqual(['resource:read']);
  });

  it('treats parent and child scopes as visible to each other', () => {
    expect(resourceVisibleToScope(scopes, 'team', 'project')).toBe(true);
    expect(resourceVisibleToScope(scopes, 'project', 'platform')).toBe(true);
    expect(resourceVisibleToScope(scopes, 'project', 'other')).toBe(false);
  });

  it('limits resource management to the selected scope and permission', () => {
    const resource = { scope_id: 'team' } as never;
    expect(canManageResource(resource, 'resource:update', 'team', false, ['resource:update'])).toBe(true);
    expect(canManageResource(resource, 'resource:delete', 'team', false, ['resource:update'])).toBe(false);
    expect(canManageResource(resource, 'resource:delete', 'project', true, [])).toBe(true);
  });
});
