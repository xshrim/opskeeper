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
  if (kind !== 'AIProvider') return '配置资源';
  return ['Provider 配置', 'Model 配置', '总结核验'][step - 2] ?? '配置资源';
}

export type DockerAccessMode = 'direct' | 'agent';

export type DockerConnectionDraft = {
  accessMode: DockerAccessMode;
  host: string;
  caBase64: string;
  certBase64: string;
  keyBase64: string;
  serverName: string;
  skipTLSVerify: boolean;
  mcpServerResourceId: string;
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
  if (draft.accessMode === 'agent') return Boolean(draft.mcpServerResourceId.trim());
  if (!draft.host.trim() || !dockerHostValid(draft.host)) return false;
  const tlsSupported = dockerHostSupportsTLS(draft.host);
  const tlsConfigured = Boolean(
    draft.caBase64.trim() || draft.certBase64.trim() || draft.keyBase64.trim() ||
    draft.serverName.trim() || draft.skipTLSVerify
  );
  if (!tlsSupported && tlsConfigured) return false;
  if (Boolean(draft.certBase64.trim()) !== Boolean(draft.keyBase64.trim())) return false;
  if ([draft.caBase64, draft.certBase64, draft.keyBase64].some((value) => value.trim() && !dockerTLSValueValid(value))) return false;
  return true;
}

export function dockerConfigForSave(draft: DockerConnectionDraft): Record<string, unknown> {
  if (draft.accessMode === 'agent') return {};
  const config: Record<string, unknown> = {};
  if (draft.host.trim()) config.docker_host = draft.host.trim();
  if (dockerHostSupportsTLS(draft.host)) {
    if (draft.serverName.trim()) config.docker_server_name = draft.serverName.trim();
    if (draft.skipTLSVerify) config.docker_skip_tls_verify = true;
  }
  return config;
}

export function dockerCredentialForSave(draft: DockerConnectionDraft): Record<string, string> {
  if (!dockerHostSupportsTLS(draft.host)) return {};
  return Object.fromEntries(
    [
      ['docker_ca', draft.caBase64],
      ['docker_cert', draft.certBase64],
      ['docker_key', draft.keyBase64]
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
