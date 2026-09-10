import type { ResourceSchema } from '../../lib/api';
import type { Resource } from '../../lib/api';

export type ProviderModel = {
  name: string;
  contextWindowTokens: number;
  maxOutputTokens: number;
  temperature: number;
  temperatureMutable: boolean;
  capabilities: string[];
  enabled: boolean;
  priority: number;
};

export type ProviderTypeOption = { value: string; label: string; baseURL: string };
export type ProviderPurposeOption = { value: string; label: string; requiredCapabilities?: string[] };

export const providerTypeOptions: ProviderTypeOption[] = [
  { value: 'openai_compatible', label: 'OpenAI 兼容', baseURL: '' },
  { value: 'openai', label: 'OpenAI', baseURL: 'https://api.openai.com/v1' },
  { value: 'anthropic', label: 'Anthropic', baseURL: 'https://api.anthropic.com/v1' },
  { value: 'gemini', label: 'Gemini', baseURL: 'https://generativelanguage.googleapis.com/v1beta/openai' },
  { value: 'grok', label: 'Grok', baseURL: 'https://api.x.ai/v1' },
  { value: 'deepseek', label: 'DeepSeek', baseURL: 'https://api.deepseek.com/v1' },
  { value: 'qwen', label: 'Qwen', baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { value: 'kimi', label: 'Kimi', baseURL: 'https://api.moonshot.cn/v1' },
  { value: 'glm', label: 'GLM', baseURL: 'https://open.bigmodel.cn/api/paas/v4' },
  { value: 'minimax', label: 'MiniMax', baseURL: 'https://api.minimaxi.com/v1' },
  { value: 'mimo', label: 'MiMo', baseURL: 'https://api.xiaomimimo.com/v1' },
  { value: 'longcat', label: 'LongCat', baseURL: '' },
  { value: 'doubao', label: 'Doubao', baseURL: 'https://ark.cn-beijing.volces.com/api/v3' },
  { value: 'openrouter', label: 'OpenRouter', baseURL: 'https://openrouter.ai/api/v1' },
  { value: 'siliconflow', label: 'SiliconFlow', baseURL: 'https://api.siliconflow.cn/v1' },
  { value: 'ollama', label: 'Ollama', baseURL: 'http://localhost:11434/v1' }
];

export const providerCapabilityOptions = [
  { value: 'text', label: '文本' },
  { value: 'vision', label: '视觉' },
  { value: 'audio', label: '音频' },
  { value: 'tool_calling', label: '工具调用' },
  { value: 'structured_output', label: '结构化输出' },
  { value: 'stream', label: '流式输出' },
  { value: 'deep_thinking', label: '深度思考' }
];

export const providerPurposeOptions: ProviderPurposeOption[] = [
  { value: 'general', label: '通用', requiredCapabilities: ['text'] },
  { value: 'diagnosis', label: '诊断', requiredCapabilities: ['text', 'tool_calling', 'stream'] },
  { value: 'inspection', label: '巡检', requiredCapabilities: ['text', 'tool_calling', 'structured_output'] },
  { value: 'workflow', label: '工作流', requiredCapabilities: ['text', 'tool_calling', 'structured_output'] }
];

export function emptyProviderModelDraft(): ProviderModel {
  return {
    name: '',
    contextWindowTokens: 128000,
    maxOutputTokens: 128000,
    temperature: 0.7,
    temperatureMutable: true,
    capabilities: ['text', 'tool_calling', 'structured_output', 'stream'],
    enabled: true,
    priority: 0
  };
}

export function resourceAddStepTitle(step: number, kind: string) {
  if (step === 1) return '基础配置';
  if (kind === 'MCPServer') return ['MCP 配置', '总结核验'][step - 2] ?? 'MCP 配置';
  if (kind === 'Docker') return ['Docker 配置', '总结核验'][step - 2] ?? 'Docker 配置';
  if (kind === 'Kubernetes') return ['Kubernetes 配置', '总结核验'][step - 2] ?? 'Kubernetes 配置';
  if (kind !== 'AIProvider') return '配置资源';
  return ['Provider 配置', 'Model 配置', '总结核验'][step - 2] ?? '配置资源';
}
export function resourceAddStepDescription(step: number, kind: string) {
  if (step === 1) return '配置资源类型、名称、归属和标签。';
  if (kind === 'MCPServer') return step === 2 ? '配置 MCP Server 的连接参数和工具范围。' : '确认配置并核验 MCP Server 连接。';
  if (kind === 'Docker') return step === 2 ? '配置 Docker Engine 的连接方式和 TLS 凭据。' : '确认配置并核验 Docker 连接。';
  if (kind === 'Kubernetes') return step === 2 ? '配置 Kubernetes API 的连接方式和访问凭据。' : '确认配置并核验 Kubernetes 连接。';
  if (kind === 'AIProvider') {
    if (step === 2) return '配置 Provider 的服务地址、协议和访问凭据。';
    if (step === 3) return '配置 Model 参数、能力和默认模型。';
    return '确认 Provider、Model 和角色配置。';
  }
  return '配置资源连接参数并确认设置。';
}

export type DockerAccessMode = 'direct' | 'agent';
export type KubernetesConnectionMode = 'kubeconfig' | 'endpoint';
export type KubernetesConnectionDraft = {
  isAgent?: boolean;
  connectionOverride?: boolean;
  connectionMode: KubernetesConnectionMode;
  server: string;
  caBase64: string;
  token: string;
  certBase64: string;
  keyBase64: string;
  kubeconfig: string;
  skipTLSVerify: boolean;
};
export function kubernetesConfigurationValid(draft: KubernetesConnectionDraft) {
  if (draft.isAgent && draft.connectionOverride === false) return true;
  if (draft.connectionMode === 'kubeconfig') return Boolean(draft.kubeconfig.trim());
  return Boolean(draft.server.trim());
}
export function kubernetesConfigForSave(draft: KubernetesConnectionDraft): Record<string, unknown> {
  if (draft.isAgent && draft.connectionOverride === false) return {};
  const config: Record<string, unknown> = { connection_mode: draft.connectionMode };
  if (draft.connectionMode === 'kubeconfig') {
    return config;
  }
  if (draft.server.trim()) config.server = draft.server.trim();
  if (draft.skipTLSVerify) config.skip_tls_verify = true;
  return config;
}
export function kubernetesCredentialForSave(draft: KubernetesConnectionDraft): Record<string, string> {
  const out: Record<string,string> = {};
  if (draft.isAgent && draft.connectionOverride === false) return out;
  if (draft.connectionMode === 'kubeconfig' && draft.kubeconfig.trim()) {
    out.kubeconfig = dockerTLSValueForSave(draft.kubeconfig);
  }
  if (draft.connectionMode === 'endpoint' && draft.token.trim()) out.token = draft.token.trim();
  if (draft.connectionMode === 'endpoint') {
    if (draft.caBase64.trim()) out.ca = dockerTLSValueForSave(draft.caBase64);
    if (draft.certBase64.trim()) out.client_cert = dockerTLSValueForSave(draft.certBase64);
    if (draft.keyBase64.trim()) out.client_key = dockerTLSValueForSave(draft.keyBase64);
  }
  return out;
}

export function kubernetesKubeconfigText(value: string) {
  if (!value.trim()) return '';
  try {
    const binary = atob(value.trim());
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    const decoded = new TextDecoder('utf-8', { fatal: true }).decode(bytes).trim();
    if (/^(?:apiVersion|kind|clusters|contexts|users):/m.test(decoded)) return decoded;
  } catch {
    // The value is ordinary kubeconfig text.
  }
  return value.trim();
}

export type DockerConnectionDraft = {
  accessMode: DockerAccessMode;
  host: string;
  timeoutSeconds: number;
  caBase64: string;
  certBase64: string;
  keyBase64: string;
  skipTLSVerify: boolean;
  mcpServerResourceId: string;
  connectionOverride: boolean;
};

export function dockerAccessModeLabel(mode: string) {
  return mode === 'agent' ? 'Agent · MCP 代理' : 'Direct · 直接连接';
}

export function dockerHostValid(value: string) {
  const raw = value.trim();
  if (!raw) return true;
  try {
    const parsed = new URL(raw);
    if (!['http:', 'https:', 'tcp:', 'unix:'].includes(parsed.protocol)) return false;
    if (parsed.username || parsed.password || parsed.search || parsed.hash) return false;
    if (parsed.protocol === 'unix:') return !parsed.host && parsed.pathname.startsWith('/');
    return Boolean(parsed.host) && (!parsed.pathname || parsed.pathname === '/');
  } catch {
    return false;
  }
}

export function dockerHostSupportsTLS(value: string) {
  try {
    const protocol = new URL(value.trim()).protocol.toLowerCase();
    return protocol === 'tcp:' || protocol === 'https:';
  } catch {
    return false;
  }
}

export function normalizeDockerTLSValue(value: string) {
  return value.trim().replace(/\s+/g, '');
}

export function dockerTLSValueValid(value: string) {
  const normalized = normalizeDockerTLSValue(value);
  if (!normalized) return false;
  if (/-----BEGIN [A-Z0-9 ]+-----[\s\S]+-----END [A-Z0-9 ]+-----/.test(value.trim())) return true;
  if (normalized.length % 4 === 1 || !/^[A-Za-z0-9+/]*={0,2}$/.test(normalized)) return false;
  try {
    return atob(normalized).length > 0;
  } catch {
    return false;
  }
}

export function dockerTLSValueForDisplay(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return '';
  try {
    const decoded = atob(normalizeDockerTLSValue(trimmed));
    if (/-----BEGIN [A-Z0-9 ]+-----[\s\S]+-----END [A-Z0-9 ]+-----/.test(decoded.trim())) return decoded.trim();
  } catch {
    // Legacy credentials may already contain PEM text.
  }
  return trimmed;
}

function utf8ToBase64(value: string) {
  const bytes = new TextEncoder().encode(value);
  let binary = '';
  const chunkSize = 0x8000;
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunkSize));
  }
  return btoa(binary);
}

// Persist TLS material as Base64 even when the operator pastes PEM text.
export function dockerTLSValueForSave(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return '';
  const normalized = normalizeDockerTLSValue(trimmed);
  if (normalized.length % 4 !== 1 && /^[A-Za-z0-9+/]*={0,2}$/.test(normalized)) {
    try {
      if (atob(normalized).length > 0) return normalized;
    } catch {
      // Fall through so PEM input can be encoded below.
    }
  }
  return utf8ToBase64(trimmed);
}

export function dockerConnectionConfigurationValid(draft: DockerConnectionDraft) {
  if (!Number.isFinite(draft.timeoutSeconds) || draft.timeoutSeconds < 1 || draft.timeoutSeconds > 300) return false;
  if (draft.accessMode === 'agent' && !draft.mcpServerResourceId.trim()) return false;
  if (draft.accessMode === 'agent' && !draft.connectionOverride) return true;
  if (!draft.host.trim() || !dockerHostValid(draft.host)) return false;
  const tlsSupported = dockerHostSupportsTLS(draft.host);
  const tlsConfigured = Boolean(
    draft.caBase64.trim() || draft.certBase64.trim() || draft.keyBase64.trim() ||
    draft.skipTLSVerify
  );
  if (!tlsSupported && tlsConfigured) return false;
  if (Boolean(draft.certBase64.trim()) !== Boolean(draft.keyBase64.trim())) return false;
  if ([draft.caBase64, draft.certBase64, draft.keyBase64].some((value) => value.trim() && !dockerTLSValueValid(value))) return false;
  return true;
}

export function dockerConfigForSave(draft: DockerConnectionDraft): Record<string, unknown> {
  if (draft.accessMode === 'agent' && !draft.connectionOverride) return {};
  const config: Record<string, unknown> = {};
  if (draft.host.trim()) config.host = draft.host.trim();
  config.timeout = draft.timeoutSeconds;
  if (dockerHostSupportsTLS(draft.host)) {
    if (draft.skipTLSVerify) config.skip_tls_verify = true;
  }
  return config;
}

export function dockerCredentialForSave(draft: DockerConnectionDraft): Record<string, string> {
  if (!dockerHostSupportsTLS(draft.host)) return {};
  return Object.fromEntries(
    [
      ['tls_ca', draft.caBase64],
      ['tls_cert', draft.certBase64],
      ['tls_key', draft.keyBase64]
    ].filter(([, value]) => value.trim()).map(([key, value]) => [key, dockerTLSValueForSave(value)])
  );
}

export function parseResourceLabels(value: string): Record<string, string> {
  return Object.fromEntries(
    value
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean)
      .map((item) => {
        const [key, ...rest] = item.split('=');
        return [key.trim(), rest.join('=').trim()];
      })
  );
}

export function mcpTransportForSubtype(subtype: string) {
  return subtype.trim().toLowerCase().replace(/[^a-z]/g, '') === 'sse'
    ? 'sse'
    : 'streamable_http';
}

export function normalizeResourceFieldValue(value: string, type?: string): unknown {
  if (type === 'integer' || type === 'number') return Number(value);
  if (type === 'boolean') return value === 'true';
  if (type === 'array') {
    try {
      const parsed: unknown = JSON.parse(value);
      if (Array.isArray(parsed)) return parsed;
    } catch {
      // Simple string arrays can still be entered as comma-separated values.
    }
    return value.split(',').map((item) => item.trim()).filter(Boolean);
  }
  return value;
}

export function buildResourceSchemaConfig(
  schema: ResourceSchema | null | undefined,
  values: Record<string, string>,
  raw: string
) {
  if (!schema?.schema.properties) return JSON.parse(raw) as Record<string, unknown>;
  return Object.fromEntries(
    Object.entries(schema.schema.properties)
      .filter(([key, field]) => !field.sensitive && (values[key] ?? '').trim() !== '')
      .map(([key, field]) => [key, normalizeResourceFieldValue(values[key], field.type)])
  );
}

export function parseMCPHeaders(raw: string): Record<string, string> {
  const headers: Record<string, string> = {};
  for (const line of raw.split(/\r?\n/).map((item) => item.trim()).filter(Boolean)) {
    const separator = line.indexOf(':');
    if (separator <= 0) throw new Error('请求 Header 格式应为“名称: 值”，每行一个。');
    const key = line.slice(0, separator).trim();
    const value = line.slice(separator + 1).trim();
    if (!key || /[\r\n:]/.test(key) || /[\r\n]/.test(value)) {
      throw new Error('请求 Header 名称或值无效。');
    }
    headers[key] = value;
  }
  return headers;
}

export function mcpConfigurationValid(
  transport: string,
  urlValue: string,
  headers: string
) {
  if (!['streamable_http', 'sse'].includes(transport) || !urlValue.trim()) return false;
  try {
    const url = new URL(urlValue.trim());
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return false;
    parseMCPHeaders(headers);
    return true;
  } catch {
    return false;
  }
}

export function mcpConfigForSave(input: {
  transport: string;
  url: string;
  toolAllowlist: string;
  timeoutSeconds: number;
  maxResponseBytes: number;
}): Record<string, unknown> {
  return {
    transport: input.transport,
    url: input.url.trim(),
    tool_allowlist: input.toolAllowlist
      .split(/[\n,]/)
      .map((item) => item.trim())
      .filter(Boolean),
    timeout_seconds: input.timeoutSeconds,
    max_response_bytes: input.maxResponseBytes
  };
}

export function providerPurposeMissingCapabilities(
  requiredCapabilities: string[],
  defaultModel: ProviderModel | undefined
) {
  if (!defaultModel) return requiredCapabilities;
  return requiredCapabilities.filter(
    (capability) => !defaultModel.capabilities.includes(capability)
  );
}

export function providerTypeLabel(type: unknown, options: ProviderTypeOption[]) {
  return options.find((option) => option.value === String(type))?.label ?? String(type || 'Provider');
}

export function providerModelsForResource(resource: Resource): Array<Record<string, unknown>> {
  return (Array.isArray(resource.config?.models) ? resource.config.models : []) as Array<Record<string, unknown>>;
}

export function providerDefaultModelForResource(resource: Resource) {
  const models = providerModelsForResource(resource);
  const configured = String(resource.config?.default_model ?? '').trim();
  return models.find((model) => String(model.name ?? '').trim() === configured) ?? models.find((model) => model.enabled !== false) ?? models[0];
}

export function providerModelCapabilities(model: Record<string, unknown> | undefined, options: Array<{ value: string; label: string }>) {
  if (!model || !Array.isArray(model.capabilities)) return [];
  return (model.capabilities as unknown[]).map((capability) => options.find((item) => item.value === String(capability))?.label ?? String(capability));
}

export function providerPurposeLabel(tag: string) {
  return ({ general: '通用', diagnosis: '诊断', inspection: '巡检', workflow: '工作流' } as Record<string, string>)[tag] ?? tag;
}

export function providerBaseURLValid(value: string) {
  try {
    const url = new URL(value.trim());
    return url.protocol === 'http:' || url.protocol === 'https:';
  } catch {
    return false;
  }
}

export function providerConfigForCreate(input: {
  type: string;
  protocol: string;
  baseURL: string;
  timeoutSeconds: number;
  maxConcurrency: number;
  rateLimitPerMinute: number;
  enabled: boolean;
  defaultModel: string;
  models: ProviderModel[];
}): Record<string, unknown> {
  return {
    provider_type: input.type,
    protocol: input.protocol,
    base_url: input.baseURL.trim(),
    timeout_seconds: input.timeoutSeconds,
    max_concurrency: input.maxConcurrency,
    rate_limit_per_minute: input.rateLimitPerMinute,
    enabled: input.enabled,
    default_model: input.defaultModel,
    models: input.models.map((model) => ({
      name: model.name.trim(),
      context_window_tokens: model.contextWindowTokens,
      max_output_tokens: model.maxOutputTokens,
      temperature: model.temperature,
      temperature_mutable: model.temperatureMutable,
      capabilities: model.capabilities,
      enabled: model.enabled,
      priority: model.priority
    }))
  };
}
