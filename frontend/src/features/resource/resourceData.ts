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
  if (resource.kind === 'MCPServer') {
    const [snapshot] = await api.mcpSnapshots(resource.id);
    if (!snapshot) return null;
    return {
      id: snapshot.id,
      resource_id: resource.id,
      status: snapshot.status === 'succeeded' ? 'succeeded' : 'failed',
      message: snapshot.error_message || (snapshot.status === 'succeeded' ? 'MCP Server 连接正常' : 'MCP Server 连接失败'),
      latency_ms: snapshot.latency_ms ?? 0,
      capabilities: [],
      checked_at: snapshot.created_at
    };
  }
  if (String(resource.subtype ?? '').toLowerCase() === 'agent' && resource.agent_ref) {
    const [snapshot] = await api.mcpSnapshots(resource.agent_ref);
    if (!snapshot) return null;
    return {
      id: `mcp-agent-${resource.id}`,
      resource_id: resource.id,
      status: snapshot.status === 'succeeded' ? 'succeeded' : 'failed',
      message: snapshot.error_message || (snapshot.status === 'succeeded' ? 'MCPServer 连接正常' : 'MCPServer 连接失败'),
      latency_ms: snapshot.latency_ms ?? 0,
      capabilities: [],
      checked_at: snapshot.created_at
    };
  }
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
  const connectorItems = resources.filter(resourceHasConnector);
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
