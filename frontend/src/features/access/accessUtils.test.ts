import { describe, expect, it } from 'vitest';
import { actorPermissionsAtScope, canManageResource, organizationScopeVisible, resourceVisibleToScope, userRoleBindings, userRoleBindingsAtSelection, usersAtScopes } from './accessUtils';

const scopes = [
  { id: 'platform', type: 'platform', name: '平台' },
  { id: 'team', type: 'team', name: '团队', parentId: 'platform' },
  { id: 'project', type: 'project', name: '项目', parentId: 'team' }
];

describe('access helpers', () => {
  it('includes direct and group role bindings without duplicates', () => {
    const groups = [{ id: 'group-1', status: 'active' }] as never[];
    const bindings = [
      { subject_type: 'user', subject_id: 'user-1', role_id: 'viewer' },
      { subject_type: 'group', subject_id: 'group-1', role_id: 'operator' }
    ] as never[];
    expect(userRoleBindings('user-1', groups, { 'group-1': ['user-1'] }, bindings)).toHaveLength(2);
  });

  it('shows sample user roles for selected team and project scopes', () => {
    const sampleScopes = [
      { id: 'platform-scope', type: 'platform', name: '平台' },
      { id: 'alpha-scope', type: 'team', name: '云平台研发部', parentId: 'platform-scope' },
      { id: 'alpha-01-scope', type: 'project', name: '统一身份中心', parentId: 'alpha-scope' },
      { id: 'alpha-02-scope', type: 'project', name: 'API 网关', parentId: 'alpha-scope' },
      { id: 'beta-scope', type: 'team', name: '基础设施运维部', parentId: 'platform-scope' },
      { id: 'beta-01-scope', type: 'project', name: '容器平台管理', parentId: 'beta-scope' },
      { id: 'beta-02-scope', type: 'project', name: '主机监控告警', parentId: 'beta-scope' },
      { id: 'gamma-scope', type: 'team', name: '数据智能部', parentId: 'platform-scope' },
      { id: 'gamma-01-scope', type: 'project', name: '数据集成平台', parentId: 'gamma-scope' },
      { id: 'gamma-02-scope', type: 'project', name: '实时指标分析', parentId: 'gamma-scope' }
    ];
    const sampleTeams = [
      { id: 'alpha', scope: { id: 'alpha-scope' } },
      { id: 'beta', scope: { id: 'beta-scope' } },
      { id: 'gamma', scope: { id: 'gamma-scope' } }
    ] as never[];
    const sampleProjects = [
      { id: 'alpha-01', team_id: 'alpha', scope: { id: 'alpha-01-scope' } },
      { id: 'alpha-02', team_id: 'alpha', scope: { id: 'alpha-02-scope' } },
      { id: 'beta-01', team_id: 'beta', scope: { id: 'beta-01-scope' } },
      { id: 'beta-02', team_id: 'beta', scope: { id: 'beta-02-scope' } },
      { id: 'gamma-01', team_id: 'gamma', scope: { id: 'gamma-01-scope' } },
      { id: 'gamma-02', team_id: 'gamma', scope: { id: 'gamma-02-scope' } }
    ] as never[];
    const sampleBindings = [
      { subject_type: 'user', subject_id: 'lina', role_id: 'team-admin', role_name: 'TeamAdmin', scope_id: 'alpha-scope' },
      { subject_type: 'user', subject_id: 'lina', role_id: 'project-viewer', role_name: 'ProjectViewer', scope_id: 'alpha-01-scope' },
      { subject_type: 'user', subject_id: 'wangtao', role_id: 'team-operator', role_name: 'TeamOperator', scope_id: 'beta-scope' },
      { subject_type: 'user', subject_id: 'wangtao', role_id: 'project-operator', role_name: 'ProjectOperator', scope_id: 'beta-02-scope' },
      { subject_type: 'user', subject_id: 'chenyu', role_id: 'team-viewer-gamma', role_name: 'TeamViewer', scope_id: 'gamma-scope' },
      { subject_type: 'user', subject_id: 'chenyu', role_id: 'project-admin-gamma', role_name: 'ProjectAdmin', scope_id: 'gamma-01-scope' },
      { subject_type: 'user', subject_id: 'zhaolei', role_id: 'team-operator-alpha', role_name: 'TeamOperator', scope_id: 'alpha-scope' },
      { subject_type: 'user', subject_id: 'zhaolei', role_id: 'team-viewer-gamma', role_name: 'TeamViewer', scope_id: 'gamma-scope' },
      { subject_type: 'user', subject_id: 'zhaolei', role_id: 'project-viewer-b1-zhaolei', role_name: 'ProjectViewer', scope_id: 'beta-01-scope' },
      { subject_type: 'user', subject_id: 'sunmin', role_id: 'project-admin', role_name: 'ProjectAdmin', scope_id: 'alpha-01-scope' },
      { subject_type: 'user', subject_id: 'sunmin', role_id: 'project-viewer-a2', role_name: 'ProjectViewer', scope_id: 'alpha-02-scope' },
      { subject_type: 'user', subject_id: 'sunmin', role_id: 'project-viewer-g2', role_name: 'ProjectViewer', scope_id: 'gamma-02-scope' },
      { subject_type: 'user', subject_id: 'zhangwei', role_id: 'platform-viewer', role_name: 'PlatformViewer', scope_id: 'platform-scope' },
      { subject_type: 'user', subject_id: 'zhangwei', role_id: 'team-viewer', role_name: 'TeamViewer', scope_id: 'alpha-scope' },
      { subject_type: 'user', subject_id: 'zhangwei', role_id: 'project-viewer-b1', role_name: 'ProjectViewer', scope_id: 'beta-01-scope' }
    ] as never[];
    const selectedRoles = (userId: string, selection: { kind: 'platform' | 'team' | 'project'; id: string }) =>
      userRoleBindingsAtSelection(userId, selection, sampleTeams, sampleProjects, sampleScopes, [], {}, sampleBindings)
        .map((binding) => binding.role_name);

    expect(selectedRoles('lina', { kind: 'team', id: 'alpha' })).toEqual(['TeamAdmin', 'ProjectViewer']);
    expect(selectedRoles('lina', { kind: 'project', id: 'alpha-01' })).toEqual(['ProjectViewer']);
    expect(selectedRoles('sunmin', { kind: 'team', id: 'alpha' })).toEqual(['ProjectAdmin', 'ProjectViewer']);
    expect(selectedRoles('sunmin', { kind: 'project', id: 'alpha-01' })).toEqual(['ProjectAdmin']);
    expect(selectedRoles('wangtao', { kind: 'team', id: 'beta' })).toEqual(['TeamOperator', 'ProjectOperator']);
    expect(selectedRoles('wangtao', { kind: 'project', id: 'beta-02' })).toEqual(['ProjectOperator']);
    expect(selectedRoles('chenyu', { kind: 'team', id: 'gamma' })).toEqual(['TeamViewer', 'ProjectAdmin']);
    expect(selectedRoles('chenyu', { kind: 'project', id: 'gamma-01' })).toEqual(['ProjectAdmin']);
    expect(selectedRoles('zhaolei', { kind: 'team', id: 'alpha' })).toEqual(['TeamOperator']);
    expect(selectedRoles('zhaolei', { kind: 'team', id: 'gamma' })).toEqual(['TeamViewer']);
    expect(selectedRoles('zhaolei', { kind: 'project', id: 'beta-01' })).toEqual(['ProjectViewer']);
    expect(selectedRoles('zhangwei', { kind: 'team', id: 'alpha' })).toEqual(['TeamViewer']);
    expect(selectedRoles('zhangwei', { kind: 'project', id: 'beta-01' })).toEqual(['ProjectViewer']);
    expect(selectedRoles('zhangwei', { kind: 'platform', id: 'platform' })).toEqual(['PlatformViewer', 'TeamViewer', 'ProjectViewer']);
  });

  it('lists direct and group members assigned within the selected scopes', () => {
    const users = [{ id: 'direct' }, { id: 'group' }, { id: 'other' }] as never[];
    const groups = [{ id: 'group-1', scope_id: 'team', status: 'active' }] as never[];
    const bindings = [
      { subject_type: 'user', subject_id: 'direct', scope_id: 'project' },
      { subject_type: 'group', subject_id: 'group-1', scope_id: 'team' },
      { subject_type: 'user', subject_id: 'other', scope_id: 'elsewhere' }
    ] as never[];

    expect(usersAtScopes(['team', 'project'], users, bindings, groups, { 'group-1': ['group'] }))
      .toEqual(users.slice(0, 2));
  });

  it('does not treat disabled group memberships as visible or effective roles', () => {
    const groups = [
      { id: 'active-group', scope_id: 'team', status: 'active' },
      { id: 'disabled-group', scope_id: 'team', status: 'disabled' }
    ] as never[];
    const groupMembers = {
      'active-group': ['active-member'],
      'disabled-group': ['disabled-member']
    };
    const bindings = [
      { subject_type: 'group', subject_id: 'active-group', role_id: 'viewer' },
      { subject_type: 'group', subject_id: 'disabled-group', role_id: 'operator' }
    ] as never[];
    const users = [{ id: 'active-member' }, { id: 'disabled-member' }] as never[];

    expect(userRoleBindings('active-member', groups, groupMembers, bindings)).toHaveLength(1);
    expect(userRoleBindings('disabled-member', groups, groupMembers, bindings)).toHaveLength(0);
    expect(usersAtScopes(['team'], users, bindings, groups, groupMembers)).toEqual([users[0]]);
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

  it('shows organization ancestors of readable scopes, but not unrelated projects', () => {
    expect(organizationScopeVisible(scopes, 'platform', ['project'])).toBe(true);
    expect(organizationScopeVisible(scopes, 'team', ['project'])).toBe(true);
    expect(organizationScopeVisible(scopes, 'project', ['project'])).toBe(true);
    expect(organizationScopeVisible(scopes, 'other', ['project'])).toBe(false);
  });

  it('limits resource management to the selected scope and permission', () => {
    const resource = { scope_id: 'team' } as never;
    expect(canManageResource(resource, 'resource:update', 'team', false, ['resource:update'])).toBe(true);
    expect(canManageResource(resource, 'resource:delete', 'team', false, ['resource:update'])).toBe(false);
    expect(canManageResource(resource, 'resource:delete', 'project', true, [])).toBe(true);
  });
});
