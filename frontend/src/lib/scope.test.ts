import { describe, expect, it } from 'vitest';
import { scopesInWorkspace, type ScopeChoice } from './scope';

const scopes: ScopeChoice[] = [
  { id: 'platform', type: 'platform', name: '平台' },
  { id: 'team-a', type: 'team', name: '团队 A', parentId: 'platform' },
  { id: 'project-a1', type: 'project', name: '项目 A1', parentId: 'team-a' },
  { id: 'project-a2', type: 'project', name: '项目 A2', parentId: 'team-a' },
  { id: 'team-b', type: 'team', name: '团队 B', parentId: 'platform' },
  { id: 'project-b1', type: 'project', name: '项目 B1', parentId: 'team-b' }
];

describe('scopesInWorkspace', () => {
  it('allows only the selected project and its ancestors, not sibling projects', () => {
    expect(scopesInWorkspace(scopes, 'project-a1').map(({ id }) => id)).toEqual([
      'platform', 'team-a', 'project-a1'
    ]);
  });

  it('allows a selected team, its ancestors, and its projects', () => {
    expect(scopesInWorkspace(scopes, 'team-a').map(({ id }) => id)).toEqual([
      'platform', 'team-a', 'project-a1', 'project-a2'
    ]);
  });

  it('allows all descendants from the platform workspace', () => {
    expect(scopesInWorkspace(scopes, 'platform')).toEqual(scopes);
  });
});
