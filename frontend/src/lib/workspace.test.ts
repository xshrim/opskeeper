import { describe, expect, it } from 'vitest';
import {
  initialWorkspaceSelection,
  projectSelection,
  providerModelSelection,
  resourceInWorkspace,
  teamSelection
} from './workspace';

describe('workspace helpers', () => {
  it('chooses the first platform role over team and project roles', () => {
    expect(
      initialWorkspaceSelection(
        [
          { scope_type: 'project', scope_id: 'project-scope' },
          { scope_type: 'platform', scope_id: 'platform-scope' },
          { scope_type: 'team', scope_id: 'team-scope' }
        ] as never[],
        [],
        [],
        'platform-scope',
        false
      )
    ).toEqual({
      teamId: '',
      projectId: '',
      scopeId: 'platform-scope'
    });
  });

  it('chooses the first team role over project roles and all projects in that team', () => {
    const teams = [
      { id: 'team-1', scope: { id: 'team-scope-1' } },
      { id: 'team-2', scope: { id: 'team-scope-2' } }
    ] as never[];
    const projects = [
      { id: 'project-1', team_id: 'team-1', scope: { id: 'project-scope-1' } }
    ] as never[];
    expect(
      initialWorkspaceSelection(
        [
          { scope_type: 'project', scope_id: 'project-scope-1' },
          { scope_type: 'team', scope_id: 'team-scope-2' },
          { scope_type: 'team', scope_id: 'team-scope-1' }
        ] as never[],
        teams,
        projects,
        'platform-scope',
        false
      )
    ).toEqual({
      teamId: 'team-2',
      projectId: '',
      scopeId: 'team-scope-2'
    });
  });

  it('selects the first project role and resolves its team', () => {
    const teams = [{ id: 'team-1', scope: { id: 'team-scope' } }] as never[];
    const projects = [
      { id: 'project-1', team_id: 'team-1', scope: { id: 'project-scope-1' } },
      { id: 'project-2', team_id: 'team-1', scope: { id: 'project-scope-2' } }
    ] as never[];
    expect(
      initialWorkspaceSelection(
        [
          { scope_type: 'project', scope_id: 'project-scope-2' },
          { scope_type: 'project', scope_id: 'project-scope-1' }
        ] as never[],
        teams,
        projects,
        'platform-scope',
        false
      )
    ).toEqual({
      teamId: 'team-1',
      projectId: 'project-2',
      scopeId: 'project-scope-2'
    });
  });

  it('falls back to platform or first team when no role binding resolves', () => {
    const teams = [{ id: 'team-1', scope: { id: 'team-scope' } }] as never[];
    expect(
      initialWorkspaceSelection(
        [] as never[],
        teams,
        [],
        'platform-scope',
        true
      )
    ).toEqual({
      teamId: '',
      projectId: '',
      scopeId: 'platform-scope'
    });
    expect(
      initialWorkspaceSelection(
        [] as never[],
        teams,
        [],
        'platform-scope',
        false
      )
    ).toEqual({
      teamId: 'team-1',
      projectId: '',
      scopeId: 'team-scope'
    });
  });

  it('keeps platform resources visible from team and project scopes', () => {
    expect(
      resourceInWorkspace(
        { scope_id: 'platform' } as never,
        'team',
        'platform',
        'team',
        ['project'],
        'project'
      )
    ).toBe(true);
  });

  it('limits a project workspace to its own team and project resources', () => {
    expect(
      resourceInWorkspace(
        { scope_id: 'other-project' } as never,
        'project',
        'platform',
        'team',
        ['project'],
        'project'
      )
    ).toBe(false);
    expect(
      resourceInWorkspace(
        { scope_id: 'project' } as never,
        'project',
        'platform',
        'team',
        ['project'],
        'project'
      )
    ).toBe(true);
  });

  it('keeps a selected model when switching to a provider that offers it', () => {
    expect(
      providerModelSelection(
        [{ provider_id: 'provider-1', models: [{ name: 'gpt-4o' }] }],
        'provider-1',
        'gpt-4o'
      )
    ).toEqual({ providerId: 'provider-1', modelName: 'gpt-4o' });
  });

  it('falls back to the first model for a provider without the current selection', () => {
    expect(
      providerModelSelection(
        [{ provider_id: 'provider-1', models: [{ name: 'claude-3' }] }],
        'provider-1',
        'gpt-4o'
      )
    ).toEqual({ providerId: 'provider-1', modelName: 'claude-3' });
  });

  it('resets project state when selecting a team', () => {
    expect(
      teamSelection('team-1', false, 'platform', [
        { id: 'team-1', scope: { id: 'team-scope' } }
      ] as never[])
    ).toEqual({
      teamId: 'team-1',
      projectId: '',
      scopeId: 'team-scope'
    });
  });

  it('selects the platform-wide team and project scope only for platform roles', () => {
    expect(teamSelection('', true, 'platform', [])).toEqual({
      teamId: '',
      projectId: '',
      scopeId: 'platform'
    });
    expect(teamSelection('', false, 'platform', [])).toBeNull();
  });

  it('selects all projects while keeping the selected team scope', () => {
    expect(
      projectSelection('', 'team-1', 'team-scope', 'platform', [])
    ).toEqual({
      teamId: 'team-1',
      projectId: '',
      scopeId: 'team-scope'
    });
  });

  it('moves to the project team when selecting a project from another team', () => {
    expect(
      projectSelection('project-1', 'team-1', 'team-scope', 'platform', [
        { id: 'project-1', team_id: 'team-2', scope: { id: 'project-scope' } }
      ] as never[])
    ).toEqual({
      teamId: 'team-2',
      projectId: 'project-1',
      scopeId: 'project-scope'
    });
  });
});
