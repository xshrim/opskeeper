import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, request } from './api';

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
});
