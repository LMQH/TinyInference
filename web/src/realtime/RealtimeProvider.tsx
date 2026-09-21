import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useQueryClient, type QueryClient } from '@tanstack/react-query';
import { decodeAdminEvent } from '../api/client/decoders';
import { adminKeys } from '../api/client/admin';
import { ContractError } from '../api/client/http';
import type { AdminEvent } from '../api/generated/admin';

export type ConnectionState = 'connecting' | 'connected' | 'reconnecting' | 'stale' | 'compatibility_error';

interface RealtimeValue {
  state: ConnectionState;
  lastSuccessfulUpdate: string | null;
  announce: (message: string) => void;
  announcement: string;
}

const RealtimeContext = createContext<RealtimeValue | null>(null);
const EVENT_TYPES = ['snapshot_changed', 'model_changed', 'queue_changed', 'metrics_updated', 'alert_changed', 'operation_changed', 'resync_required'];

function useRealtimeValue(): RealtimeValue {
  const value = useContext(RealtimeContext);
  if (!value) throw new Error('RealtimeProvider 缺失。');
  return value;
}

export const useRealtime = useRealtimeValue;

async function invalidateChanged(queryClient: QueryClient, event: AdminEvent): Promise<void> {
  const changed = event.type === 'resync_required' ? ['service', 'model', 'queue', 'resources', 'requests', 'metrics', 'alerts', 'logs', 'operations'] : (event.data.changed ?? []);
  const invalidations: Promise<unknown>[] = [queryClient.invalidateQueries({ queryKey: adminKeys.snapshot })];
  if (changed.includes('requests') || changed.includes('queue')) invalidations.push(queryClient.invalidateQueries({ queryKey: ['admin', 'requests'] }));
  if (changed.includes('metrics')) invalidations.push(queryClient.invalidateQueries({ queryKey: ['admin', 'metrics'] }));
  if (changed.includes('alerts')) invalidations.push(queryClient.invalidateQueries({ queryKey: ['admin', 'alerts'] }));
  if (changed.includes('logs')) invalidations.push(queryClient.invalidateQueries({ queryKey: ['admin', 'logs'] }));
  if (changed.includes('operations') || changed.includes('model')) invalidations.push(queryClient.invalidateQueries({ queryKey: adminKeys.operations }));
  await Promise.all(invalidations);
}

async function readEvents(
  signal: AbortSignal,
  lastEventId: number | null,
  onOpen: () => void,
  onEvent: (event: AdminEvent) => Promise<void>,
): Promise<void> {
  const headers = new Headers({ Accept: 'text/event-stream' });
  if (lastEventId !== null) headers.set('Last-Event-ID', String(lastEventId));
  const response = await fetch('/admin/v1/events', { headers, credentials: 'same-origin', cache: 'no-store', redirect: 'error', signal });
  if (!response.ok || !response.body || !(response.headers.get('content-type') ?? '').includes('text/event-stream')) {
    throw new Error('事件流不可用。');
  }
  onOpen();

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  while (!signal.aborted) {
    const chunk = await reader.read();
    if (chunk.done) return;
    buffer += decoder.decode(chunk.value, { stream: true }).replaceAll('\r\n', '\n');
    let boundary = buffer.indexOf('\n\n');
    while (boundary >= 0) {
      const block = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      const payload = block.split('\n').filter((line) => line.startsWith('data:')).map((line) => line.slice(5).trimStart()).join('\n');
      if (payload) await onEvent(decodeAdminEvent(JSON.parse(payload) as unknown));
      boundary = buffer.indexOf('\n\n');
    }
  }
}

export interface RealtimeProviderProps { children: ReactNode }

export function RealtimeProvider({ children }: RealtimeProviderProps) {
  const queryClient = useQueryClient();
  const [state, setState] = useState<ConnectionState>('connecting');
  const [lastSuccessfulUpdate, setLastSuccessfulUpdate] = useState<string | null>(null);
  const [announcement, setAnnouncement] = useState('');
  const lastEventId = useRef<number | null>(null);
  const lastSnapshotVersion = useRef(0);
  const wasStale = useRef(false);

  useEffect(() => {
    const controller = new AbortController();
    let stopped = false;
    let reconnectDelay = 1000;

    const connect = async () => {
      while (!stopped) {
        if (document.visibilityState === 'hidden') {
          const { promise, resolve } = Promise.withResolvers<void>();
          window.setTimeout(resolve, 1000);
          await promise;
          continue;
        }
        setState(lastEventId.current === null ? 'connecting' : 'reconnecting');
        try {
          await readEvents(controller.signal, lastEventId.current, () => {
            setState('connected');
            if (wasStale.current) {
              wasStale.current = false;
              setAnnouncement('数据更新已恢复。');
            }
          }, async (event) => {
            if (!EVENT_TYPES.includes(event.type)) {
              setState('compatibility_error');
              setAnnouncement('收到无法识别的更新，正在重新同步全部数据。');
              await invalidateChanged(queryClient, { ...event, type: 'resync_required' });
              return;
            }
            if (event.type === 'resync_required') {
              lastEventId.current = Math.max(lastEventId.current ?? 0, event.event_id);
              lastSnapshotVersion.current = Math.max(lastSnapshotVersion.current, event.snapshot_version);
              await invalidateChanged(queryClient, event);
              setLastSuccessfulUpdate(event.occurred_at);
              setState('connected');
              reconnectDelay = 1000;
              return;
            }
            if (event.event_id <= (lastEventId.current ?? 0)) return;
            lastEventId.current = event.event_id;
            if (event.snapshot_version <= lastSnapshotVersion.current) return;
            lastSnapshotVersion.current = Math.max(lastSnapshotVersion.current, event.snapshot_version);
            await invalidateChanged(queryClient, event);
            setLastSuccessfulUpdate(event.occurred_at);
            setState('connected');
            reconnectDelay = 1000;
          });
          if (!stopped) throw new Error('事件流已结束。');
        } catch (error) {
          if (stopped || controller.signal.aborted) return;
          if (error instanceof ContractError || error instanceof SyntaxError) {
            setState('compatibility_error');
            setAnnouncement('收到无法解析的更新，正在重新同步全部数据。');
            await queryClient.invalidateQueries({ queryKey: adminKeys.all });
          } else {
            setState('reconnecting');
          }
          const { promise, resolve } = Promise.withResolvers<void>();
          window.setTimeout(resolve, reconnectDelay);
          await promise;
          reconnectDelay = Math.min(reconnectDelay * 2, 15000);
        }
      }
    };

    void connect();
    const onVisibility = () => {
      if (document.visibilityState === 'visible') void queryClient.invalidateQueries({ queryKey: adminKeys.all });
    };
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      stopped = true;
      controller.abort();
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [queryClient]);

  useEffect(() => {
    if (state !== 'reconnecting') return;
    const timer = window.setTimeout(() => {
      const snapshotState = queryClient.getQueryState(adminKeys.snapshot);
      if ((snapshotState?.errorUpdatedAt ?? 0) <= (snapshotState?.dataUpdatedAt ?? 0)) return;
      wasStale.current = true;
      setState('stale');
      setAnnouncement('数据更新已中断，当前显示最近一次成功结果。');
    }, 5000);
    return () => window.clearTimeout(timer);
  }, [queryClient, state]);

  const value = useMemo<RealtimeValue>(() => ({ state, lastSuccessfulUpdate, announce: setAnnouncement, announcement }), [announcement, lastSuccessfulUpdate, state]);
  return <RealtimeContext.Provider value={value}>{children}</RealtimeContext.Provider>;
}
