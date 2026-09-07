import { describe, expect, it } from 'vitest';
import {
  buildResourceSchemaConfig,
  dockerConfigForSave,
  dockerConnectionConfigurationValid,
  dockerCredentialForSave,
  dockerHostValid,
  dockerAccessModeLabel,
  dockerTLSValueValid,
  emptyProviderModelDraft,
  mcpConfigForSave,
  mcpConfigurationValid,
  parseMCPHeaders,
  providerPurposeMissingCapabilities
} from './resourceWorkflow';

describe('resource workflow helpers', () => {
  it('keeps sensitive schema values out of the resource config', () => {
    expect(
      buildResourceSchemaConfig(
        {
          id: 'schema-example',
          kind: 'Example',
          version: 1,
          status: 'active',
          display_name: 'Example',
          description: 'Example schema',
          icon: 'example',
          schema: {
            properties: { host: { type: 'string' }, password: { sensitive: true } }
          }
        },
        { host: 'db.example.com', password: 'secret' },
        '{}'
      )
    ).toEqual({ host: 'db.example.com' });
  });

  it('parses MCP headers and rejects malformed input', () => {
    expect(parseMCPHeaders('X-Tenant: production\nAuthorization: Bearer token')).toEqual({
      'X-Tenant': 'production',
      Authorization: 'Bearer token'
    });
    expect(() => parseMCPHeaders('missing separator')).toThrow('请求 Header 格式');
  });

  it('requires a safe MCP endpoint and serializes its allowlist', () => {
    expect(mcpConfigurationValid('sse', 'https://mcp.example.com/sse', '')).toBe(true);
    expect(mcpConfigurationValid('sse', 'https://token@example.com/sse', '')).toBe(false);
    expect(
      mcpConfigForSave({
        transport: 'sse',
        url: ' https://mcp.example.com/sse ',
        toolAllowlist: 'docker:*\n logs:read, deploy:*',
        timeoutSeconds: 120,
        maxResponseBytes: 1024
      })
    ).toMatchObject({
      url: 'https://mcp.example.com/sse',
      tool_allowlist: ['docker:*', 'logs:read', 'deploy:*']
    });
  });

  it('reports the provider capabilities missing from the default model', () => {
    expect(
      providerPurposeMissingCapabilities(['text', 'tool_calling', 'stream'], {
        name: 'model',
        contextWindowTokens: 128000,
        maxOutputTokens: 4096,
        temperature: 0.7,
        temperatureMutable: true,
        capabilities: ['text', 'stream'],
        enabled: true,
        priority: 0
      })
    ).toEqual(['tool_calling']);
  });

  it('provides stable defaults for a new provider model', () => {
    expect(emptyProviderModelDraft()).toMatchObject({
      contextWindowTokens: 128000,
      capabilities: ['text', 'tool_calling', 'structured_output', 'stream'],
      enabled: true
    });
  });

  it('validates Docker connection modes and keeps TLS Base64 in credentials', () => {
    expect(dockerAccessModeLabel('direct')).toBe('Direct · 直接连接');
    expect(dockerHostValid('unix:///var/run/docker.sock')).toBe(true);
    expect(dockerHostValid('https://docker.example.com:2376')).toBe(true);
    expect(dockerHostValid('https://user:secret@docker.example.com')).toBe(false);
    const direct = {
      accessMode: 'direct' as const,
      host: 'tcp://docker.example.com:2376',
      timeoutSeconds: 10,
      caBase64: 'Y2E=',
      certBase64: 'Y2VydA==',
      keyBase64: 'a2V5',
      serverName: 'docker.example.com',
      skipTLSVerify: false,
      mcpServerResourceId: '',
      connectionOverride: false
    };
    expect(dockerConnectionConfigurationValid(direct)).toBe(true);
    expect(dockerConfigForSave(direct)).toEqual({
      host: 'tcp://docker.example.com:2376',
      timeout: 10,
      tls_server_name: 'docker.example.com'
    });
    expect(dockerCredentialForSave(direct)).toEqual({
      tls_ca: 'Y2E=',
      tls_cert: 'Y2VydA==',
      tls_key: 'a2V5'
    });
    expect(dockerTLSValueValid('Y2E=')).toBe(true);
    expect(dockerTLSValueValid('/etc/docker/ca.pem')).toBe(false);
    expect(dockerConnectionConfigurationValid({ ...direct, accessMode: 'agent', mcpServerResourceId: '' })).toBe(false);
    expect(dockerConnectionConfigurationValid({ ...direct, certBase64: '', keyBase64: '' })).toBe(true);
    expect(dockerConnectionConfigurationValid({ ...direct, certBase64: '/etc/docker/cert.pem', keyBase64: '' })).toBe(false);
    expect(dockerConnectionConfigurationValid({ ...direct, host: 'http://docker.example.com:2375' })).toBe(false);
  });
});
