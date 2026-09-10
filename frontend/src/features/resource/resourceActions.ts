import { api } from '../../lib/api';
import type { ConnectionCheck, MCPSnapshot, Relation, Resource, ResourceSchema } from '../../lib/api';

export function createResourceRelation(resourceId: string, targetResourceId: string, relationType: string) {
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

export async function createSchemaCredential(
  schema: ResourceSchema | null | undefined,
  values: Record<string, string>,
  scopeId: string,
  displayName: string
) {
  const secret = Object.fromEntries(Object.entries(values).filter(([, value]) => value.trim() !== ''));
  if (!scopeId || !schema || Object.keys(secret).length === 0) return '';
  const credential = await api.createCredential({
    scope_id: scopeId,
    name: `${displayName || schema.display_name} connection`,
    purpose: `${schema.display_name} 敏感连接信息`,
    secret: JSON.stringify(secret)
  });
  return credential.id;
}

export async function createProviderCredential(scopeId: string, name: string, apiKey: string) {
  if (!apiKey.trim() || !scopeId) return '';
  const credential = await api.createCredential({
    scope_id: scopeId,
    name: `${name || 'AI Provider'} API Key`,
    purpose: 'AI Provider 访问凭据',
    secret: apiKey.trim()
  });
  return credential.id;
}

export async function saveProviderCredential(
  provider: Resource,
  scopeId: string,
  name: string,
  apiKey: string
) {
  if (!apiKey.trim()) return provider.credential_id ?? '';
  if (!provider.credential_id) return createProviderCredential(scopeId, name, apiKey);
  await api.updateCredential(provider.credential_id, {
    name: `${name.trim() || 'AI Provider'} API Key`,
    purpose: 'AI Provider 访问凭据',
    secret: apiKey.trim()
  });
  return provider.credential_id;
}

export async function createMCPCredential(scopeId: string, name: string, token: string, headers: Record<string, string>, tls: Record<string, string | boolean> = {}) {
  const tlsSecret = Object.fromEntries(Object.entries(tls).filter(([, value]) => typeof value === 'boolean' ? value : String(value).trim() !== ''));
  const secret = { token: token.trim(), headers, ...tlsSecret };
  if (!scopeId || (!token.trim() && Object.keys(headers).length === 0 && Object.keys(tlsSecret).length === 0)) return '';
  const credential = await api.createCredential({
    scope_id: scopeId,
    name: `${name || 'MCP Server'} 访问凭据`,
    purpose: 'MCP Server 访问与 TLS 凭据',
    secret: JSON.stringify(secret)
  });
  return credential.id;
}

export async function saveMCPCredential(
  existing: Resource,
  scopeId: string,
  name: string,
  token: string,
  headers: Record<string, string>,
  tls: Record<string, string | boolean> = {}
) {
  const tlsSecret = Object.fromEntries(Object.entries(tls).filter(([, value]) => typeof value === 'boolean' ? true : String(value).trim() !== ''));
  if (!existing.credential_id) return createMCPCredential(scopeId, name, token, headers, tlsSecret);
  let nextToken = token.trim();
  let existingTLS: Record<string, string | boolean> = {};
  {
    try {
      const current = await api.credentialSecret(existing.credential_id);
      try {
        const parsed = JSON.parse(current.secret) as { token?: string; tls_ca?: string; tls_cert?: string; tls_key?: string; tls_skip_verify?: boolean };
        if (!nextToken) nextToken = String(parsed.token ?? '').trim();
        existingTLS = Object.fromEntries(Object.entries({ tls_ca: parsed.tls_ca, tls_cert: parsed.tls_cert, tls_key: parsed.tls_key, tls_skip_verify: parsed.tls_skip_verify }).filter(([, value]) => typeof value === 'boolean' ? true : String(value ?? '').trim() !== '')) as Record<string, string | boolean>;
      } catch {
        nextToken = current.secret.trim();
      }
    } catch {
      // Preserve legacy credentials when no replacement token is supplied.
    }
  }
  const nextTLS = { ...existingTLS, ...tlsSecret };
  if (!nextToken && Object.keys(headers).length === 0 && Object.keys(nextTLS).length === 0) return existing.credential_id;
  await api.updateCredential(existing.credential_id, {
    name: `${name.trim() || 'MCP Server'} 访问凭据`,
    purpose: 'MCP Server 访问与 TLS 凭据',
    secret: JSON.stringify({ token: nextToken, headers, ...nextTLS })
  });
  return existing.credential_id;
}

export async function createDockerCredential(
  scopeId: string,
  name: string,
  values: Record<string, string>
) {
  const secret = Object.fromEntries(Object.entries(values).filter(([, value]) => value.trim() !== ''));
  if (!scopeId || Object.keys(secret).length === 0) return '';
  const credential = await api.createCredential({
    scope_id: scopeId,
    name: `${name || 'Docker'} TLS 凭据`,
    purpose: 'Docker TLS Base64 凭据',
    secret: JSON.stringify(secret)
  });
  return credential.id;
}

export async function createKubernetesCredential(scopeId: string, name: string, values: Record<string, string>) {
  const secret = Object.fromEntries(Object.entries(values).filter(([, value]) => value.trim()));
  if (!scopeId || !Object.keys(secret).length) return '';
  const credential = await api.createCredential({ scope_id: scopeId, name: `${name || 'Kubernetes'} 连接凭据`, purpose: 'Kubernetes kubeconfig 与 Token', secret: JSON.stringify(secret) });
  return credential.id;
}
export async function saveKubernetesCredential(existing: Resource, scopeId: string, name: string, values: Record<string, string>) {
  const secret = Object.fromEntries(Object.entries(values).filter(([, value]) => value.trim()));
  if (!Object.keys(secret).length) return '';
  if (!existing.credential_id) return createKubernetesCredential(scopeId, name, values);
  await api.updateCredential(existing.credential_id, { name: `${name.trim() || 'Kubernetes'} 连接凭据`, purpose: 'Kubernetes kubeconfig 与 Token', secret: JSON.stringify(secret) });
  return existing.credential_id;
}

export async function saveDockerCredential(
  existing: Resource,
  scopeId: string,
  name: string,
  values: Record<string, string>
) {
  const secret = Object.fromEntries(Object.entries(values).filter(([, value]) => value.trim() !== ''));
  // An empty Direct form means "keep the existing encrypted credential".
  // Agent updates clear it explicitly at the resource workflow boundary.
  if (!Object.keys(secret).length) return existing.credential_id ?? '';
  if (!existing.credential_id) return createDockerCredential(scopeId, name, values);
  await api.updateCredential(existing.credential_id, {
    name: `${name.trim() || 'Docker'} TLS 凭据`,
    purpose: 'Docker TLS Base64 凭据',
    secret: JSON.stringify(secret)
  });
  return existing.credential_id;
}

export async function testResourceConnector(
  resource: Resource,
  scopeId: string
): Promise<{ check: ConnectionCheck; snapshot?: MCPSnapshot }> {
  if (resource.kind === 'AIProvider') {
    const config = resource.config ?? {};
    const models = Array.isArray(config.models)
      ? config.models as Array<Record<string, unknown>>
      : [];
    const configuredDefault = String(config.default_model ?? '').trim();
    const defaultModel = models.find((model) => String(model.name ?? '').trim() === configuredDefault)
      ?? models.find((model) => Boolean(model.enabled ?? true));
    const modelName = String(defaultModel?.name ?? '').trim();
    if (!modelName) throw new Error('该 AI Provider 尚未配置可用的默认 Model。');
    const result = await api.testAIProvider(resource.id, {
      scope_id: scopeId || resource.scope_id,
      model_name: modelName,
      stream: Array.isArray(defaultModel?.capabilities)
        && (defaultModel?.capabilities as unknown[]).includes('stream')
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
        message: snapshot.error_message || (snapshot.status === 'succeeded' ? 'MCP Server 连接正常' : 'MCP Server 连接失败'),
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
        message: snapshot.error_message || (snapshot.status === 'succeeded' ? 'MCPServer 连接正常' : 'MCPServer 连接失败'),
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

export function loadResourceCredentialSecret(credentialId: string) {
  return api.credentialSecret(credentialId);
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
  return api.createResource(body);
}

export function updateResourceRecord(resourceId: string, body: Record<string, unknown>) {
  return api.updateResource(resourceId, body);
}
