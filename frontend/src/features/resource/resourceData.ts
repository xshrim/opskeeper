import { api, ApiError } from '../../lib/api';
import { resourceHasConnector } from '../../lib/resources';
import type { ConnectionCheck, MCPSnapshot, Relation, Resource, TopologyNode } from '../../lib/api';

export async function loadResourceRelations(resourceId: string): Promise<{
  relations: Relation[];
  topology: TopologyNode[];
}> {
  const [relations, topology] = await Promise.all([
    api.relations(resourceId),
    api.topology(resourceId)
  ]);
  return { relations, topology: topology.items };
}

export async function loadResourceConnectionCheck(
  resource: Resource
): Promise<ConnectionCheck | null> {
  if (!resourceHasConnector(resource)) return null;
  if (resource.kind === 'AIProvider' || resource.kind === 'MCPServer') return null;
  try {
    return await api.latestResourceConnectionCheck(resource.id);
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return null;
    throw error;
  }
}

export function prependMCPSnapshot(
  snapshots: Record<string, MCPSnapshot[]>,
  resourceId: string,
  snapshot: MCPSnapshot
) {
  return {
    ...snapshots,
    [resourceId]: [snapshot, ...(snapshots[resourceId] ?? [])]
  };
}

export async function loadResourceConnectionChecks(resources: Resource[]) {
  const connectorItems = resources.filter(
    (resource) => resource.kind !== 'AIProvider'
      && resource.kind !== 'MCPServer'
      && resourceHasConnector(resource)
  );
  if (!connectorItems.length) return {} as Record<string, ConnectionCheck | null>;
  const checks = await Promise.all(connectorItems.map(async (resource) => {
    try {
      return [resource.id, await loadResourceConnectionCheck(resource)] as const;
    } catch {
      return [resource.id, null] as const;
    }
  }));
  return Object.fromEntries(checks) as Record<string, ConnectionCheck | null>;
}
