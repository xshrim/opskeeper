import { describe, expect, it } from 'vitest';
import { projectSelection, providerModelSelection, resourceInWorkspace, teamSelection } from './workspace';

describe('workspace helpers', () => {
  it('keeps platform resources visible from team and project scopes', () => {
    expect(resourceInWorkspace(
      { scope_id: 'platform' } as never,
      'team',
      'platform',
      'team',
      ['project'],
      'project'
    )).toBe(true);
  });

  it('limits a project workspace to its own team and project resources', () => {
    expect(resourceInWorkspace(
      { scope_id: 'other-project' } as never,
      'project',
      'platform',
      'team',
      ['project'],
      'project'
    )).toBe(false);
    expect(resourceInWorkspace(
      { scope_id: 'project' } as never,
      'project',
      'platform',
      'team',
      ['project'],
      'project'
    )).toBe(true);
  });

  it('keeps a selected model when switching to a provider that offers it', () => {
    expect(providerModelSelection(
      [{ provider_resource_id: 'provider-1', models: [{ name: 'gpt-4o' }] }],
      'provider-1',
      'gpt-4o'
    )).toEqual({ providerId: 'provider-1', modelName: 'gpt-4o' });
  });

  it('falls back to the first model for a provider without the current selection', () => {
    expect(providerModelSelection(
      [{ provider_resource_id: 'provider-1', models: [{ name: 'claude-3' }] }],
      'provider-1',
      'gpt-4o'
    )).toEqual({ providerId: 'provider-1', modelName: 'claude-3' });
  });

  it('resets project state when selecting a team', () => {
    expect(teamSelection('team-1', false, 'platform', [{ id: 'team-1', scope: { id: 'team-scope' } }] as never[])).toEqual({
      teamId: 'team-1', projectId: '', scopeId: 'team-scope'
    });
  });

  it('moves to the project team when selecting a project from another team', () => {
    expect(projectSelection('project-1', 'team-1', 'team-scope', 'platform', [{ id: 'project-1', team_id: 'team-2', scope: { id: 'project-scope' } }] as never[])).toEqual({
      teamId: 'team-2', projectId: 'project-1', scopeId: 'project-scope'
    });
  });
});
