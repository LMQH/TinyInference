import {
  decodeAlerts,
  decodeCancel,
  decodeLogs,
  decodeMetrics,
  decodeModelAction,
  decodeOperations,
  decodeRequestPage,
  decodeSnapshot,
} from './decoders';
import { requestJson } from './http';
import type { LogLevel } from '../generated/admin';

export const adminKeys = {
  all: ['admin'] as const,
  snapshot: ['admin', 'snapshot'] as const,
  requests: (cursor: string | null) => ['admin', 'requests', cursor] as const,
  metrics: (window: MetricWindow, granularity: MetricGranularity) => ['admin', 'metrics', window, granularity] as const,
  alerts: (cursor: string | null) => ['admin', 'alerts', cursor] as const,
  logs: (level: LogLevel, cursor: string | null) => ['admin', 'logs', level, cursor] as const,
  operations: ['admin', 'operations'] as const,
};

export type MetricWindow = '1h' | '24h' | '7d' | '30d';
export type MetricGranularity = '1m' | '1h' | '1d';
export interface MetricSelection { window: MetricWindow; granularity: MetricGranularity }

export const METRIC_SELECTIONS: readonly MetricSelection[] = [
  { window: '1h', granularity: '1m' },
  { window: '24h', granularity: '1h' },
  { window: '7d', granularity: '1h' },
  { window: '30d', granularity: '1d' },
];

function params(values: Record<string, string | number | null>): string {
  const query = new URLSearchParams();
  Object.entries(values).forEach(([key, value]) => {
    if (value !== null) query.set(key, String(value));
  });
  return query.toString();
}

export const adminApi = {
  snapshot: (signal?: AbortSignal) => requestJson('/admin/v1/snapshot', { decode: decodeSnapshot, signal }),
  requests: (cursor: string | null, signal?: AbortSignal) => requestJson(`/admin/v1/requests?${params({ cursor, limit: 50 })}`, { decode: decodeRequestPage, signal }),
  metrics: (selection: MetricSelection, signal?: AbortSignal) => requestJson(`/admin/v1/metrics?${params({ window: selection.window, granularity: selection.granularity })}`, { decode: decodeMetrics, signal }),
  alerts: (cursor: string | null, signal?: AbortSignal) => requestJson(`/admin/v1/alerts?${params({ cursor, limit: 50 })}`, { decode: decodeAlerts, signal }),
  logs: (level: LogLevel, cursor: string | null, signal?: AbortSignal) => requestJson(`/admin/v1/logs?${params({ cursor, limit: 50, level })}`, { decode: decodeLogs, signal }),
  operations: (signal?: AbortSignal) => requestJson('/admin/v1/operations', { decode: decodeOperations, signal }),
  startModel: (signal?: AbortSignal) => requestJson('/admin/v1/model/start', { method: 'POST', decode: decodeModelAction, signal, conditional: false }),
  stopModel: (signal?: AbortSignal) => requestJson('/admin/v1/model/stop', { method: 'POST', decode: decodeModelAction, signal, conditional: false }),
  cancelWaiting: (requestId: string, signal?: AbortSignal) => requestJson(`/admin/v1/queue/${encodeURIComponent(requestId)}/cancel`, { method: 'POST', decode: decodeCancel, signal, conditional: false }),
};
