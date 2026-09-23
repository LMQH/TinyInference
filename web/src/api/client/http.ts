import type { SafeError } from '../generated/admin';

export class ApiError extends Error {
  readonly status: number;
  readonly detail: SafeError | null;
  readonly retryAfterSeconds: number | null;

  constructor(status: number, detail: SafeError | null) {
    super(detail?.message ?? `管理接口返回 HTTP ${status}`);
    this.name = 'ApiError';
    this.status = status;
    this.detail = detail;
    this.retryAfterSeconds = detail?.retry_after_seconds ?? null;
  }
}

export class ContractError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ContractError';
  }
}

const ADMIN_PREFIX = '/admin/v1/';

function assertSameOriginAdminPath(path: string): void {
  if (!path.startsWith(ADMIN_PREFIX) || path.startsWith('//') || path.includes('://') || path.includes('\\')) {
    throw new ContractError('客户端拒绝了非同源管理接口路径。');
  }
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseError(value: unknown): SafeError | null {
  if (!isObject(value) || !isObject(value.error)) return null;
  const error = value.error;
  if (
    typeof error.message !== 'string' ||
    typeof error.type !== 'string' ||
    typeof error.code !== 'string' ||
    !(typeof error.param === 'string' || error.param === null) ||
    typeof error.request_id !== 'string' ||
    typeof error.retryable !== 'boolean' ||
    !(typeof error.retry_after_seconds === 'number' || error.retry_after_seconds === null)
  ) return null;
  return {
    message: error.message,
    type: error.type,
    code: error.code,
    param: error.param,
    request_id: error.request_id,
    retryable: error.retryable,
    retry_after_seconds: error.retry_after_seconds,
  };
}

const etags = new Map<string, string>();
const cachedBodies = new Map<string, unknown>();

export interface RequestOptions<T> {
  method?: 'GET' | 'POST';
  body?: unknown;
  signal?: AbortSignal;
  decode: (value: unknown) => T;
  conditional?: boolean;
}

export async function requestJson<T>(path: string, options: RequestOptions<T>): Promise<T> {
  assertSameOriginAdminPath(path);
  const method = options.method ?? 'GET';
  const headers = new Headers({ Accept: 'application/json' });
  if (options.body !== undefined) headers.set('Content-Type', 'application/json');
  if (method === 'GET' && options.conditional !== false) {
    const etag = etags.get(path);
    if (etag) headers.set('If-None-Match', etag);
  }

  const response = await fetch(path, {
    method,
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
    credentials: 'same-origin',
    cache: 'no-store',
    redirect: 'error',
    signal: options.signal,
  });

  if (response.status === 304) {
    const cached = cachedBodies.get(path);
    if (cached === undefined) throw new ContractError('接口返回 304，但当前页面没有可用快照。');
    return options.decode(cached);
  }

  const contentType = response.headers.get('content-type') ?? '';
  const value: unknown = contentType.includes('application/json') ? await response.json() : null;
  if (!response.ok) throw new ApiError(response.status, parseError(value));
  if (!contentType.includes('application/json')) throw new ContractError('管理接口未返回 JSON。');

  const decoded = options.decode(value);
  if (method === 'GET') {
    const etag = response.headers.get('etag');
    if (etag) etags.set(path, etag);
    cachedBodies.set(path, value);
  }
  return decoded;
}

export function clearConditionalCache(): void {
  etags.clear();
  cachedBodies.clear();
}

export function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const code = error.detail?.code;
    return code ? `${error.message}（${code}）` : error.message;
  }
  if (error instanceof ContractError) return `${fallback} 后端响应与当前控制台契约不兼容。`;
  return fallback;
}
