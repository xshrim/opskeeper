import type { ResourceSchema } from '../../lib/api';

export function resourceAddStepTitle(step: number, kind: string) {
  if (step === 1) return '基础配置';
  if (kind === 'MCPServer')
    return ['MCP 配置', '总结核验'][step - 2] ?? 'MCP 配置';
  if (kind === 'Docker')
    return ['Docker 配置', '总结核验'][step - 2] ?? 'Docker 配置';
  if (kind === 'Kubernetes')
    return ['Kubernetes 配置', '总结核验'][step - 2] ?? 'Kubernetes 配置';
  if (kind === 'Host')
    return ['Host 配置', '总结核验'][step - 2] ?? 'Host 配置';
  if (kind === 'PostgreSQL')
    return ['PostgreSQL 配置', '总结核验'][step - 2] ?? 'PostgreSQL 配置';
  if (kind === 'Oracle')
    return ['Oracle 配置', '总结核验'][step - 2] ?? 'Oracle 配置';
  if (kind === 'RabbitMQ')
    return ['RabbitMQ 配置', '总结核验'][step - 2] ?? 'RabbitMQ 配置';
  if (kind === 'MinIO')
    return ['MinIO 配置', '总结核验'][step - 2] ?? 'MinIO 配置';
  if (kind === 'Repository')
    return ['Repository 配置', '总结核验'][step - 2] ?? 'Repository 配置';
  return '配置资源';
}
export function resourceAddStepDescription(step: number, kind: string) {
  if (step === 1) return '配置资源类型、名称、归属和标签。';
  if (kind === 'MCPServer')
    return step === 2
      ? '配置 MCP Server 的连接参数和工具范围。'
      : '确认配置并核验 MCP Server 连接。';
  if (kind === 'Docker')
    return step === 2
      ? '配置 Docker Engine 的连接方式和 TLS 凭据。'
      : '确认配置并核验 Docker 连接。';
  if (kind === 'Kubernetes')
    return step === 2
      ? '配置 Kubernetes API 的连接方式和访问凭据。'
      : '确认配置并核验 Kubernetes 连接。';
  if (kind === 'Host')
    return step === 2
      ? '配置 Linux 主机的本机或 SSH 接入方式。'
      : '确认配置并核验 Host 连接。';
  if (kind === 'Repository')
    return step === 2 ? '配置 Git 或 Bundle Repository。' : '确认 Repository 配置。';
  if (kind === 'PostgreSQL')
    return step === 2 ? '配置 PostgreSQL 数据库连接方式和凭据。' : '确认配置并核验 PostgreSQL 连接。';
  if (kind === 'Oracle')
    return step === 2 ? '配置 Oracle 数据库连接方式和凭据。' : '确认配置并核验 Oracle 连接。';
  if (kind === 'RabbitMQ')
    return step === 2 ? '配置 RabbitMQ Management API 连接方式和凭据。' : '确认配置并核验 RabbitMQ 连接。';
  if (kind === 'MinIO')
    return step === 2 ? '配置 MinIO S3 API 连接方式和凭据。' : '确认配置并核验 MinIO 连接。';
  return '配置资源连接参数并确认设置。';
}

export type DockerAccessMode = 'direct' | 'agent';
export type HostAccessMode = 'direct' | 'agent';
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
  if (draft.connectionMode === 'kubeconfig')
    return Boolean(draft.kubeconfig.trim());
  return Boolean(draft.server.trim());
}
export function kubernetesConfigForSave(
  draft: KubernetesConnectionDraft
): Record<string, unknown> {
  if (draft.isAgent && draft.connectionOverride === false) return {};
  const config: Record<string, unknown> = {
    connection_mode: draft.connectionMode
  };
  if (draft.connectionMode === 'kubeconfig') {
    return config;
  }
  if (draft.server.trim()) config.server = draft.server.trim();
  if (draft.skipTLSVerify) config.skip_tls_verify = true;
  return config;
}
export function kubernetesCredentialForSave(
  draft: KubernetesConnectionDraft
): Record<string, string> {
  const out: Record<string, string> = {};
  if (draft.isAgent && draft.connectionOverride === false) return out;
  if (draft.connectionMode === 'kubeconfig' && draft.kubeconfig.trim()) {
    out.kubeconfig = dockerTLSValueForSave(draft.kubeconfig);
  }
  if (draft.connectionMode === 'endpoint' && draft.token.trim())
    out.token = draft.token.trim();
  if (draft.connectionMode === 'endpoint') {
    if (draft.caBase64.trim()) out.ca = dockerTLSValueForSave(draft.caBase64);
    if (draft.certBase64.trim())
      out.client_cert = dockerTLSValueForSave(draft.certBase64);
    if (draft.keyBase64.trim())
      out.client_key = dockerTLSValueForSave(draft.keyBase64);
  }
  return out;
}

export function kubernetesKubeconfigText(value: string) {
  if (!value.trim()) return '';
  try {
    const binary = atob(value.trim());
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    const decoded = new TextDecoder('utf-8', { fatal: true })
      .decode(bytes)
      .trim();
    if (/^(?:apiVersion|kind|clusters|contexts|users):/m.test(decoded))
      return decoded;
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
    if (!['http:', 'https:', 'tcp:', 'unix:'].includes(parsed.protocol))
      return false;
    if (parsed.username || parsed.password || parsed.search || parsed.hash)
      return false;
    if (parsed.protocol === 'unix:')
      return !parsed.host && parsed.pathname.startsWith('/');
    return (
      Boolean(parsed.host) && (!parsed.pathname || parsed.pathname === '/')
    );
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
  if (
    /-----BEGIN [A-Z0-9 ]+-----[\s\S]+-----END [A-Z0-9 ]+-----/.test(
      value.trim()
    )
  )
    return true;
  if (normalized.length % 4 === 1 || !/^[A-Za-z0-9+/]*={0,2}$/.test(normalized))
    return false;
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
    if (
      /-----BEGIN [A-Z0-9 ]+-----[\s\S]+-----END [A-Z0-9 ]+-----/.test(
        decoded.trim()
      )
    )
      return decoded.trim();
  } catch {
    // PEM text is accepted directly as well as Base64-encoded PEM.
  }
  return trimmed;
}

function utf8ToBase64(value: string) {
  const bytes = new TextEncoder().encode(value);
  let binary = '';
  const chunkSize = 0x8000;
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    binary += String.fromCharCode(
      ...bytes.subarray(offset, offset + chunkSize)
    );
  }
  return btoa(binary);
}

// Persist TLS material as Base64 even when the operator pastes PEM text.
export function dockerTLSValueForSave(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return '';
  const normalized = normalizeDockerTLSValue(trimmed);
  if (
    normalized.length % 4 !== 1 &&
    /^[A-Za-z0-9+/]*={0,2}$/.test(normalized)
  ) {
    try {
      if (atob(normalized).length > 0) return normalized;
    } catch {
      // Fall through so PEM input can be encoded below.
    }
  }
  return utf8ToBase64(trimmed);
}

export function dockerConnectionConfigurationValid(
  draft: DockerConnectionDraft
) {
  if (
    !Number.isFinite(draft.timeoutSeconds) ||
    draft.timeoutSeconds < 1 ||
    draft.timeoutSeconds > 300
  )
    return false;
  if (draft.accessMode === 'agent' && !draft.mcpServerResourceId.trim())
    return false;
  if (draft.accessMode === 'agent' && !draft.connectionOverride) return true;
  if (!draft.host.trim() || !dockerHostValid(draft.host)) return false;
  const tlsSupported = dockerHostSupportsTLS(draft.host);
  const tlsConfigured = Boolean(
    draft.caBase64.trim() ||
    draft.certBase64.trim() ||
    draft.keyBase64.trim() ||
    draft.skipTLSVerify
  );
  if (!tlsSupported && tlsConfigured) return false;
  if (Boolean(draft.certBase64.trim()) !== Boolean(draft.keyBase64.trim()))
    return false;
  if (
    [draft.caBase64, draft.certBase64, draft.keyBase64].some(
      (value) => value.trim() && !dockerTLSValueValid(value)
    )
  )
    return false;
  return true;
}

export type HostConnectionDraft = {
  accessMode: HostAccessMode;
  host: string;
  port: number;
  username: string;
  authMethod: 'password' | 'key';
  password: string;
  privateKey: string;
  passphrase: string;
  knownHosts: string;
  timeoutSeconds: number;
  mcpServerResourceId: string;
  connectionOverride: boolean;
};

export function hostAccessModeLabel(mode: string) {
  return mode === 'agent' ? 'Agent · MCP 代理' : 'Direct · 直接连接';
}

export function hostAccessSummary(
  subtype: string,
  config: Record<string, unknown> | undefined
) {
  const mode = String(subtype).toLowerCase();
  const authMethod = String(config?.auth_method ?? '').toLowerCase();
  if (mode === 'agent' && !authMethod) return subtype;
  if (!authMethod) return subtype;
  return `${subtype} · ${authMethod === 'key' ? 'SSH 私钥' : 'SSH 密码'}`;
}

export function hostConnectionConfigurationValid(draft: HostConnectionDraft) {
  if (
    !Number.isFinite(draft.timeoutSeconds) ||
    draft.timeoutSeconds < 1 ||
    draft.timeoutSeconds > 300
  )
    return false;
  if (draft.accessMode === 'agent' && !draft.mcpServerResourceId.trim())
    return false;
  if (draft.accessMode === 'agent' && !draft.connectionOverride) return true;
  if (!draft.host.trim())
    return (
      !draft.username.trim() &&
      !draft.password.trim() &&
      !draft.privateKey.trim()
    );
  if (draft.port < 1 || draft.port > 65535 || !draft.username.trim())
    return false;
  if (draft.authMethod === 'password')
    return Boolean(draft.password.trim()) && !draft.privateKey.trim();
  return Boolean(draft.privateKey.trim()) && !draft.password.trim();
}

export function hostConfigForSave(
  draft: HostConnectionDraft
): Record<string, unknown> {
  if (draft.accessMode === 'agent' && !draft.connectionOverride) return {};
  const config: Record<string, unknown> = {
    timeout_seconds: draft.timeoutSeconds
  };
  if (draft.host.trim()) config.host = draft.host.trim();
  if (draft.port) config.port = draft.port;
  if (draft.username.trim()) config.username = draft.username.trim();
  if (draft.authMethod) config.auth_method = draft.authMethod;
  return config;
}

export function hostCredentialForSave(
  draft: HostConnectionDraft
): Record<string, string> {
  if (draft.accessMode === 'agent' && !draft.connectionOverride) return {};
  return Object.fromEntries(
    [
      ['password', draft.password],
      ['private_key', draft.privateKey],
      ['passphrase', draft.passphrase],
      ['known_hosts', draft.knownHosts]
    ].filter(([, value]) => value.trim())
  );
}

export function dockerConfigForSave(
  draft: DockerConnectionDraft
): Record<string, unknown> {
  if (draft.accessMode === 'agent' && !draft.connectionOverride) return {};
  const config: Record<string, unknown> = {};
  if (draft.host.trim()) config.host = draft.host.trim();
  config.timeout = draft.timeoutSeconds;
  if (dockerHostSupportsTLS(draft.host)) {
    if (draft.skipTLSVerify) config.skip_tls_verify = true;
  }
  return config;
}

export function dockerCredentialForSave(
  draft: DockerConnectionDraft
): Record<string, string> {
  if (!dockerHostSupportsTLS(draft.host)) return {};
  return Object.fromEntries(
    [
      ['tls_ca', draft.caBase64],
      ['tls_cert', draft.certBase64],
      ['tls_key', draft.keyBase64]
    ]
      .filter(([, value]) => value.trim())
      .map(([key, value]) => [key, dockerTLSValueForSave(value)])
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
  return subtype
    .trim()
    .toLowerCase()
    .replace(/[^a-z]/g, '') === 'sse'
    ? 'sse'
    : 'streamable_http';
}

export function normalizeResourceFieldValue(
  value: string,
  type?: string
): unknown {
  if (type === 'integer' || type === 'number') return Number(value);
  if (type === 'boolean') return value === 'true';
  if (type === 'array') {
    try {
      const parsed: unknown = JSON.parse(value);
      if (Array.isArray(parsed)) return parsed;
    } catch {
      // Simple string arrays can still be entered as comma-separated values.
    }
    return value
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean);
  }
  return value;
}

export function buildResourceSchemaConfig(
  schema: ResourceSchema | null | undefined,
  values: Record<string, string>,
  raw: string
) {
  if (!schema?.schema.properties)
    return JSON.parse(raw) as Record<string, unknown>;
  return Object.fromEntries(
    Object.entries(schema.schema.properties)
      .filter(
        ([key, field]) => !field.sensitive && (values[key] ?? '').trim() !== ''
      )
      .map(([key, field]) => [
        key,
        normalizeResourceFieldValue(values[key], field.type)
      ])
  );
}

export function parseMCPHeaders(raw: string): Record<string, string> {
  const headers: Record<string, string> = {};
  for (const line of raw
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)) {
    const separator = line.indexOf(':');
    if (separator <= 0)
      throw new Error('请求 Header 格式应为“名称: 值”，每行一个。');
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
  if (!['streamable_http', 'sse'].includes(transport) || !urlValue.trim())
    return false;
  try {
    const url = new URL(urlValue.trim());
    if (
      !['http:', 'https:'].includes(url.protocol) ||
      url.username ||
      url.password
    )
      return false;
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
