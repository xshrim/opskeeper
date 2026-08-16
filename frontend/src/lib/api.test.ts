import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError, request } from './api';

describe('request', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    const base = document.createElement('base');
    base.href = 'http://localhost:5173/opskeeper/';
    document.head.append(base);
  });

  it('retries an expired session after refreshing the cookie', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            error: { code: 'invalid_session', message: 'expired' }
          }),
          { status: 401 }
        )
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ok: true }), { status: 200 })
      );

    await expect(request<{ ok: boolean }>('api/v1/platform')).resolves.toEqual({
      ok: true
    });
    expect(String(fetchMock.mock.calls[1][0])).toBe(
      'http://localhost:5173/opskeeper/api/v1/auth/refresh'
    );
    expect(fetchMock.mock.calls[1][1]).toEqual(
      expect.objectContaining({ method: 'POST', credentials: 'include' })
    );
    expect(fetchMock).toHaveBeenCalledTimes(3);
    document.querySelector('base')?.remove();
  });

  it('maps structured API errors without exposing response internals', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify({
          error: {
            code: 'forbidden',
            message: 'No access',
            request_id: 'req-1'
          }
        }),
        { status: 403 }
      )
    );

    const error = await request('api/v1/resources').catch((value) => value);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 403,
      code: 'forbidden',
      requestId: 'req-1',
      message: 'No access'
    });
    document.querySelector('base')?.remove();
  });

  it('uses the resource connection test endpoints', async () => {
    const check = {
      id: 'check-1',
      resource_id: 'resource-1',
      status: 'succeeded',
      message: '连接测试通过',
      latency_ms: 12,
      capabilities: ['query_metrics'],
      checked_at: '2026-08-16T00:00:00Z'
    };
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockImplementation(
        async () => new Response(JSON.stringify(check), { status: 200 })
      );

    await api.testResourceConnection('resource-1');
    await api.latestResourceConnectionCheck('resource-1');

    expect(String(fetchMock.mock.calls[0][0])).toBe(
      'http://localhost:5173/opskeeper/api/v1/resources/resource-1/connection-tests'
    );
    expect(fetchMock.mock.calls[0][1]).toEqual(
      expect.objectContaining({ method: 'POST' })
    );
    expect(String(fetchMock.mock.calls[1][0])).toBe(
      'http://localhost:5173/opskeeper/api/v1/resources/resource-1/connection-tests/latest'
    );
    document.querySelector('base')?.remove();
  });

  it('uses scoped diagnosis endpoints and preserves the application base path', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      .mockImplementation(
        async () => new Response(JSON.stringify([]), { status: 200 })
      );

    await api.diagnosisSessions('scope one');
    expect(String(fetchMock.mock.calls[0][0])).toBe(
      'http://localhost:5173/opskeeper/api/v1/diagnosis-sessions?scope_id=scope%20one&limit=50'
    );
    expect(String(api.diagnosisEventsURL('session-1', 42))).toBe(
      'http://localhost:5173/opskeeper/api/v1/diagnosis-sessions/session-1/events?after=42'
    );
    document.querySelector('base')?.remove();
  });
});
