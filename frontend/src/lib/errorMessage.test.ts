import { describe, expect, it } from 'vitest';
import { ApiError } from './api';
import { errorMessage } from './errorMessage';

describe('errorMessage', () => {
  it('uses configured messages for common API statuses', () => {
    const options = { forbidden: 'forbidden', unauthorized: 'login again' };
    expect(errorMessage(new ApiError(403, { code: 'forbidden', message: 'raw' }), 'fallback', options)).toBe('forbidden');
    expect(errorMessage(new ApiError(401, { code: 'unauthorized', message: 'raw' }), 'fallback', options)).toBe('login again');
  });

  it('preserves server detail and handles syntax or unknown errors', () => {
    expect(errorMessage(new ApiError(500, { code: 'internal', message: 'detail' }), 'fallback')).toBe('detail');
    expect(errorMessage(new SyntaxError(), 'fallback', { invalidJson: 'invalid JSON' })).toBe('invalid JSON');
    expect(errorMessage({}, 'fallback')).toBe('fallback');
  });
});
