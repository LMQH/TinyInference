import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { adminApi, adminKeys, type MetricSelection } from './admin';
import type { LogLevel } from '../generated/admin';

const visibleInterval = (interval: number) => document.visibilityState === 'visible' ? interval : false;

export function useSnapshot() {
  return useQuery({
    queryKey: adminKeys.snapshot,
    queryFn: ({ signal }) => adminApi.snapshot(signal),
    refetchInterval: () => visibleInterval(2000),
    retry: false,
  });
}

export function useRequests(cursor: string | null) {
  return useQuery({
    queryKey: adminKeys.requests(cursor),
    queryFn: ({ signal }) => adminApi.requests(cursor, signal),
    placeholderData: keepPreviousData,
    refetchInterval: () => visibleInterval(30000),
    retry: false,
  });
}

export function useMetrics(selection: MetricSelection) {
  return useQuery({
    queryKey: adminKeys.metrics(selection.window, selection.granularity),
    queryFn: ({ signal }) => adminApi.metrics(selection, signal),
    placeholderData: keepPreviousData,
    refetchInterval: () => visibleInterval(30000),
    retry: false,
  });
}

export function useAlerts(cursor: string | null) {
  return useQuery({
    queryKey: adminKeys.alerts(cursor),
    queryFn: ({ signal }) => adminApi.alerts(cursor, signal),
    placeholderData: keepPreviousData,
    refetchInterval: () => visibleInterval(30000),
    retry: false,
  });
}

export function useLogs(level: LogLevel, cursor: string | null) {
  return useQuery({
    queryKey: adminKeys.logs(level, cursor),
    queryFn: ({ signal }) => adminApi.logs(level, cursor, signal),
    placeholderData: keepPreviousData,
    refetchInterval: () => visibleInterval(30000),
    retry: false,
  });
}

export function useOperations() {
  return useQuery({
    queryKey: adminKeys.operations,
    queryFn: ({ signal }) => adminApi.operations(signal),
    refetchInterval: () => visibleInterval(30000),
    retry: false,
  });
}
