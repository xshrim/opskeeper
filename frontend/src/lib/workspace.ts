import type { Resource } from './api';
import type { Project, RoleBinding, Team } from './api';

export function initialWorkspaceSelection(
  roleBindings: RoleBinding[],
  teams: Team[],
  projects: Project[],
  platformScopeId: string | undefined,
  hasPlatformRole: boolean
) {
  const scopePriority: Record<string, number> = {
    platform: 0,
    team: 1,
    project: 2
  };
  const strongestBinding = roleBindings.reduce<RoleBinding | null>(
    (strongest, binding) => {
      const priority = scopePriority[binding.scope_type];
      if (priority === undefined) return strongest;
      return strongest === null ||
        priority < scopePriority[strongest.scope_type]
        ? binding
        : strongest;
    },
    null
  );

  if (strongestBinding?.scope_type === 'platform') {
    return {
      teamId: '',
      projectId: '',
      scopeId: strongestBinding.scope_id
    };
  }
  if (strongestBinding?.scope_type === 'team') {
    const team = teams.find(
      (item) => item.scope.id === strongestBinding.scope_id
    );
    if (team) return { teamId: team.id, projectId: '', scopeId: team.scope.id };
  }
  if (strongestBinding?.scope_type === 'project') {
    const project = projects.find(
      (item) => item.scope.id === strongestBinding.scope_id
    );
    if (project) {
      return {
        teamId: project.team_id,
        projectId: project.id,
        scopeId: project.scope.id
      };
    }
  }

  const defaultTeam = hasPlatformRole ? undefined : teams[0];
  return {
    teamId: defaultTeam?.id ?? '',
    projectId: '',
    scopeId: defaultTeam?.scope.id ?? platformScopeId ?? ''
  };
}

export function resourceInWorkspace(
  resource: Resource,
  activeScopeType: string | undefined,
  platformScopeId: string | undefined,
  selectedTeamScopeId: string | undefined,
  teamProjectScopeIds: string[],
  selectedProjectScopeId: string | undefined
) {
  if (activeScopeType === 'platform') return true;
  if (activeScopeType === 'team') {
    return (
      resource.scope_id === platformScopeId ||
      resource.scope_id === selectedTeamScopeId ||
      teamProjectScopeIds.includes(resource.scope_id)
    );
  }
  return (
    resource.scope_id === platformScopeId ||
    resource.scope_id === selectedTeamScopeId ||
    resource.scope_id === selectedProjectScopeId
  );
}

export function providerModelSelection(
  providers: Array<{ provider_id: string; models: Array<{ name?: unknown }> }>,
  providerId: string,
  modelName: string
) {
  const provider =
    providers.find((item) => item.provider_id === providerId) ?? providers[0];
  if (!provider) return { providerId, modelName };
  const selectedModel = provider.models.some(
    (model) => String(model.name ?? '') === modelName
  )
    ? modelName
    : String(provider.models[0]?.name ?? '');
  return { providerId: provider.provider_id, modelName: selectedModel };
}

export function teamSelection(
  teamId: string,
  hasPlatformRole: boolean,
  platformScopeId: string | undefined,
  teams: Team[]
) {
  if (!teamId && hasPlatformRole) {
    return { teamId: '', projectId: '', scopeId: platformScopeId ?? '' };
  }
  const team = teams.find((item) => item.id === teamId);
  return team
    ? { teamId: team.id, projectId: '', scopeId: team.scope.id }
    : null;
}

export function projectSelection(
  projectId: string,
  selectedTeamId: string,
  selectedTeamScopeId: string | undefined,
  platformScopeId: string | undefined,
  projects: Project[]
) {
  if (!projectId) {
    return {
      teamId: selectedTeamId,
      projectId: '',
      scopeId: selectedTeamScopeId ?? platformScopeId ?? ''
    };
  }
  const project = projects.find((item) => item.id === projectId);
  return {
    teamId: project?.team_id ?? selectedTeamId,
    projectId: project?.id ?? '',
    scopeId: project?.scope.id ?? selectedTeamScopeId ?? ''
  };
}
