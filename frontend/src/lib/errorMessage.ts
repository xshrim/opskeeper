import { ApiError } from './api';

export interface ErrorMessageOptions {
  forbidden?: string;
  unauthorized?: string;
  invalidJson?: string;
}

export function errorMessage(
  error: unknown,
  fallback: string,
  options: ErrorMessageOptions = {}
): string {
  if (error instanceof ApiError) {
    if (error.status === 403 && options.forbidden) return options.forbidden;
    if (error.status === 401 && options.unauthorized) return options.unauthorized;
    return error.message || fallback;
  }
  if (error instanceof SyntaxError && options.invalidJson) {
    return options.invalidJson;
  }
  return error instanceof Error ? error.message || fallback : fallback;
}
