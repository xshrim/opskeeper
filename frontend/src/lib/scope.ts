import type { Platform, Project, Team } from './api';

export type ScopeChoice = {
  id: string;
  type: string;
  name: string;
  parentId?: string;
};

export function buildScopeChoices(
  platform: Platform | null,
  teams: Team[],
  projects: Project[]
): ScopeChoice[] {
  const choices: ScopeChoice[] = platform
    ? [{ id: platform.scope.id, type: 'platform', name: platform.name }]
    : [];
  for (const team of teams) {
    choices.push({
      id: team.scope.id,
      type: 'team',
      name: team.name,
      parentId: platform?.scope.id
    });
  }
  for (const project of projects) {
    choices.push({
      id: project.scope.id,
      type: 'project',
      name: project.name,
      parentId: teams.find((team) => team.id === project.team_id)?.scope.id
    });
  }
  return choices;
}

export function scopeName(scopes: ScopeChoice[], id: string) {
  return scopes.find((scope) => scope.id === id)?.name ?? id.slice(0, 8);
}

export function scopeType(scopes: ScopeChoice[], id: string) {
  return scopes.find((scope) => scope.id === id)?.type ?? 'scope';
}

export function scopeContains(scopes: ScopeChoice[], ancestorId: string, scopeId: string) {
  let current = scopes.find((scope) => scope.id === scopeId);
  while (current) {
    if (current.id === ancestorId) return true;
    current = current.parentId
      ? scopes.find((scope) => scope.id === current?.parentId)
      : undefined;
  }
  return false;
}
