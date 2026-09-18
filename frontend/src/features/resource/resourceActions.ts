import { api } from '../../lib/api';
import type {
  ConnectionCheck,
  MCPSnapshot,
  Relation,
  Resource,
  ResourceSchema
} from '../../lib/api';

export type ResourceSecret = { purpose: string; secret: string };

function secretPayload(purpose: string, secret: string): ResourceSecret | null {
  const value = secret.trim();
  return value ? { purpose, secret: value } : null;
}

export function createResourceRelation(
  resourceId: string,
  targetResourceId: string,
  relationType: string
) {
  return api.createRelation(resourceId, {
    target_resource_id: targetResourceId,
    relation_type: relationType,
    attributes: {},
    confirmed: true
  });
}

export function removeResourceRelation(resourceId: string, relation: Relation) {
  return api.deleteRelation(resourceId, relation.id);
}

export function setResourceEnabled(resource: Resource, enabled: boolean) {
  return api.updateResource(resource.id, {
    status: enabled ? 'active' : 'disabled',
    ...(resource.kind === 'AIProvider'
      ? { config: { ...(resource.config ?? {}), enabled } }
      : {})
  });
}

export function removeResource(resourceId: string) {
  return api.deleteResource(resourceId);
}

export function schemaSecret(
  schema: ResourceSchema | null | undefined,
  values: Record<string, string>
) {
  const secret = Object.fromEntries(
    Object.entries(values).filter(([, value]) => value.trim() !== '')
  );
  if (!schema || Object.keys(secret).length === 0) return null;
  return secretPayload(`${schema.display_name} 敏感连接信息`, JSON.stringify(secret));
}

export function providerSecret(apiKey: string) {
  return secretPayload('AI Provider 访问凭据', apiKey);
}

export function mcpSecret(
  token: string,
  headers: Record<string, string>,
  tls: Record<string, string | boolean> = {}
) {
  const tlsSecret = Object.fromEntries(
    Object.entries(tls).filter(([, value]) =>
      typeof value === 'boolean' ? value : String(value).trim() !== ''
    )
  );
  const secret = { token: token.trim(), headers, ...tlsSecret };
  if (!token.trim() && Object.keys(headers).length === 0 && Object.keys(tlsSecret).length === 0)
    return null;
  return secretPayload('MCP Server 访问与 TLS 凭据', JSON.stringify(secret));
}

export function dockerSecret(values: Record<string, string>) {
  const secret = Object.fromEntries(
    Object.entries(values).filter(([, value]) => value.trim() !== '')
  );
  if (Object.keys(secret).length === 0) return null;
  return secretPayload('Docker TLS Base64 凭据', JSON.stringify(secret));
}

export function kubernetesSecret(values: Record<string, string>) {
  const secret = Object.fromEntries(
    Object.entries(values).filter(([, value]) => value.trim())
  );
  if (!Object.keys(secret).length) return null;
  return secretPayload('Kubernetes kubeconfig 与 Token', JSON.stringify(secret));
}

export function hostSecret(values: Record<string, string>) {
  const secret = Object.fromEntries(
    Object.entries(values).filter(([, value]) => value.trim() !== '')
  );
  if (Object.keys(secret).length === 0) return null;
  return secretPayload('Host SSH 密码、私钥与 known_hosts', JSON.stringify(secret));
}

export async function testResourceConnector(
  resource: Resource,
  scopeId: string
): Promise<{ check: ConnectionCheck; snapshot?: MCPSnapshot }> {
  if (resource.kind === 'AIProvider') {
    const config = resource.config ?? {};
    const models = Array.isArray(config.models)
      ? (config.models as Array<Record<string, unknown>>)
      : [];
    const configuredDefault = String(config.default_model ?? '').trim();
    const defaultModel =
      models.find(
        (model) => String(model.name ?? '').trim() === configuredDefault
      ) ?? models.find((model) => Boolean(model.enabled ?? true));
    const modelName = String(defaultModel?.name ?? '').trim();
    if (!modelName)
      throw new Error('该 AI Provider 尚未配置可用的默认 Model。');
    const result = await api.testAIProvider(resource.id, {
      scope_id: scopeId || resource.scope_id,
      model_name: modelName,
      stream:
        Array.isArray(defaultModel?.capabilities) &&
        (defaultModel?.capabilities as unknown[]).includes('stream')
    });
    return {
      check: {
        id: `ai-provider-${resource.id}`,
        resource_id: resource.id,
        status: result.status === 'succeeded' ? 'succeeded' : 'failed',
        message: result.message,
        latency_ms: result.latency_ms,
        capabilities: [],
        checked_at: new Date().toISOString()
      }
    };
  }

  if (resource.kind === 'MCPServer') {
    const snapshot = await api.discoverMCP(resource.id);
    return {
      snapshot,
      check: {
        id: `mcp-server-${resource.id}`,
        resource_id: resource.id,
        status: snapshot.status === 'succeeded' ? 'succeeded' : 'failed',
        message:
          snapshot.error_message ||
          (snapshot.status === 'succeeded'
            ? 'MCP Server 连接正常'
            : 'MCP Server 连接失败'),
        latency_ms: snapshot.latency_ms ?? 0,
        capabilities: [],
        checked_at: new Date().toISOString()
      }
    };
  }

  if (String(resource.subtype ?? '').toLowerCase() === 'agent') {
    if (!resource.agent_ref) throw new Error('Agent 资源未关联 MCPServer。');
    const snapshot = await api.discoverMCP(resource.agent_ref);
    return {
      snapshot,
      check: {
        id: `mcp-agent-${resource.id}`,
        resource_id: resource.id,
        status: snapshot.status === 'succeeded' ? 'succeeded' : 'failed',
        message:
          snapshot.error_message ||
          (snapshot.status === 'succeeded'
            ? 'MCPServer 连接正常'
            : 'MCPServer 连接失败'),
        latency_ms: snapshot.latency_ms ?? 0,
        capabilities: [],
        checked_at: snapshot.created_at || new Date().toISOString()
      }
    };
  }

  return { check: await api.testResourceConnection(resource.id) };
}

export function testDraftMCPConnection(body: {
  transport: string;
  url: string;
  token: string;
  request_headers: Record<string, string>;
  tool_allowlist: string[];
  timeout_seconds: number;
  max_response_bytes: number;
  tls_ca?: string;
  tls_cert?: string;
  tls_key?: string;
  tls_skip_verify?: boolean;
}) {
  return api.testDraftMCP(body);
}
export function testDraftPostgreSQL(body: { host: string; port: number; database: string; username: string; password: string; timeout_seconds?: number }) { return api.testDraftPostgreSQL(body); }
export function testDraftMySQL(body: { host: string; port: number; database: string; username: string; password: string; timeout_seconds?: number }) { return api.testDraftMySQL(body); }
export function testDraftOracle(body: { host: string; port: number; service_name?: string; sid?: string; username: string; password: string; timeout_seconds?: number; tls?: boolean }) { return api.testDraftOracle(body); }
export function testDraftRabbitMQ(body: { url: string; username?: string; password?: string; timeout_seconds?: number; tls_insecure?: boolean }) { return api.testDraftRabbitMQ(body); }
export function testDraftMinIO(body: { endpoint: string; access_key?: string; secret_key?: string; session_token?: string; region?: string; secure?: boolean; timeout_seconds?: number }) { return api.testDraftMinIO(body); }
export function testDraftKafka(body: { brokers: string[]; username?: string; password?: string; tls?: boolean; tls_server_name?: string; timeout_seconds?: number }) { return api.testDraftKafka(body); }
export function testDraftElasticsearch(body: { url: string; username?: string; password?: string; tls_insecure?: boolean; timeout_seconds?: number }) { return api.testDraftElasticsearch(body); }
export function testDraftRedis(body: { host: string; port: number; database: number; username: string; password: string; timeout_seconds?: number }) { return api.testDraftRedis(body); }
export function testDraftNacos(body: { host: string; port: number; scheme?: string; context_path?: string; username?: string; password?: string; access_token?: string; timeout_seconds?: number }) { return api.testDraftNacos(body); }

export function testDraftAIProviderConnection(body: {
  scope_id: string;
  provider_type: string;
  base_url: string;
  model_name: string;
  api_key: string;
  timeout_seconds: number;
  context_window: number;
  temperature: number;
  capabilities: string[];
  stream: boolean;
}) {
  return api.testDraftAIProvider(body);
}

export function loadMCPSnapshots(resourceId: string) {
  return api.mcpSnapshots(resourceId);
}

export async function syncAIProviderBindings(
  scopeId: string,
  providerId: string,
  existingTags: string[],
  nextTags: string[]
) {
  await Promise.all([
    ...existingTags
      .filter((tag) => !nextTags.includes(tag))
      .map((tag) => api.removeAIProviderBinding(scopeId, tag)),
    ...nextTags.map((tag) => api.setAIProviderBinding(scopeId, tag, providerId))
  ]);
  return api.aiProviderBindings(scopeId);
}

export function createResourceRecord(body: Record<string, unknown>) {
  const { credential, ...resource } = body;
  if (credential === undefined || credential === '') return api.createResource(resource);
  return api.createResource(body);
}

export function updateResourceRecord(
  resourceId: string,
  body: Record<string, unknown>
) {
  const { credential, ...resource } = body;
  if (credential === undefined || credential === '')
    return api.updateResource(resourceId, resource);
  return api.updateResource(resourceId, body);
}
