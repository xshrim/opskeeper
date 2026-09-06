import { describe, expect, it } from 'vitest';
import {
  buildResourceSchemaConfig,
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
});
