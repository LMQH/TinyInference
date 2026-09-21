import { useRef, useState } from 'react';
import { Button, InlineNotification, Modal } from '@carbon/react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { WaitingRequest } from '../api/generated/admin';
import { adminApi, adminKeys } from '../api/client/admin';
import { errorMessage } from '../api/client/http';
import { absoluteTime, formatDuration, shortId, yesNo } from './format';
import { RequestStatusTag } from './StatusTag';
import { useRealtime } from '../realtime/RealtimeProvider';

export interface WaitingQueueProps { waiting: WaitingRequest[]; limit?: number; headingId?: string }

export function WaitingQueue({ waiting, limit, headingId = 'waiting-queue-heading' }: WaitingQueueProps) {
  const queryClient = useQueryClient();
  const realtime = useRealtime();
  const [selected, setSelected] = useState<WaitingRequest | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const errorRef = useRef<HTMLDivElement | null>(null);
  const visible = limit === undefined ? waiting : waiting.slice(0, limit);
  const cancelMutation = useMutation({
    mutationFn: (requestId: string) => adminApi.cancelWaiting(requestId),
    onSuccess: async (result) => {
      const cancelled = selected;
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: adminKeys.snapshot }),
        queryClient.invalidateQueries({ queryKey: ['admin', 'requests'] }),
      ]);
      setSelected(null);
      if (cancelled) realtime.announce(`请求 ${shortId(result.request_id)} 已取消。`);
      window.requestAnimationFrame(() => {
        if (triggerRef.current?.isConnected) triggerRef.current.focus();
        else document.getElementById(headingId)?.focus();
      });
    },
    onError: async () => {
      await queryClient.invalidateQueries({ queryKey: adminKeys.snapshot });
      window.requestAnimationFrame(() => errorRef.current?.focus());
    },
  });

  if (visible.length === 0) return <p className="empty-state">当前没有等待请求。</p>;

  return (
    <>
      <div className="table-scroll" role="region" aria-labelledby={headingId} tabIndex={0}>
        <p className="table-description">表格可横向滚动。队列按后端权威 FIFO 顺位显示，不能重排。</p>
        <table className="data-table">
          <caption>等待队列，按 FIFO 顺序，不能重排</caption>
          <thead><tr><th scope="col">顺位</th><th scope="col">请求标识</th><th scope="col">端点</th><th scope="col">流式</th><th scope="col">进入队列时间</th><th scope="col">已等待</th><th scope="col">超时截止时间</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
          <tbody>
            {visible.map((request) => (
              <tr key={request.id}>
                <td>{request.position}</td>
                <td className="code" title={request.id}>{shortId(request.id)}</td>
                <td>{request.endpoint}</td>
                <td>{yesNo(request.stream)}</td>
                <td title={request.enqueued_at}>{absoluteTime(request.enqueued_at)}</td>
                <td>{formatDuration(Math.max(0, Date.now() - new Date(request.enqueued_at).getTime()))}</td>
                <td title={request.deadline_at}>{absoluteTime(request.deadline_at)}</td>
                <td><RequestStatusTag state={request.status} /></td>
                <td>
                  {request.can_cancel ? (
                    <Button
                      kind="danger--tertiary"
                      size="sm"
                      aria-label={`取消请求 ${shortId(request.id)}`}
                      onClick={(event) => {
                        triggerRef.current = event.currentTarget;
                        cancelMutation.reset();
                        setSelected(request);
                      }}
                    >取消请求</Button>
                  ) : '当前不可取消'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Modal
        danger
        open={selected !== null}
        modalHeading="取消此等待请求？"
        primaryButtonText={cancelMutation.isPending ? '正在提交…' : '取消请求'}
        secondaryButtonText="返回"
        primaryButtonDisabled={cancelMutation.isPending}
        preventCloseOnClickOutside={cancelMutation.isPending}
        onRequestClose={() => { if (!cancelMutation.isPending) setSelected(null); }}
        onRequestSubmit={() => { if (selected && !cancelMutation.isPending) cancelMutation.mutate(selected.id); }}
      >
        {selected && <p>请求 <span className="code">{shortId(selected.id)}</span> 取消后不会执行，也不能恢复。其他等待请求的相对顺序不会改变。</p>}
        {cancelMutation.isError && (
          <div ref={errorRef} tabIndex={-1}>
            <InlineNotification
              hideCloseButton
              kind="error"
              role="alert"
              title="无法取消该请求"
              subtitle={`${errorMessage(cancelMutation.error, '请求状态可能已经变化。')} 列表已重新加载。`}
            />
          </div>
        )}
      </Modal>
    </>
  );
}
