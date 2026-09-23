import { useEffect, useRef, useState } from 'react';
import { Button, InlineNotification, Modal, SkeletonPlaceholder, SkeletonText, Tag, TextInput } from '@carbon/react';
import { ArrowRight, Play, StopFilledAlt } from '@carbon/icons-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useOperations, useSnapshot } from '../../api/client/hooks';
import { adminApi, adminKeys } from '../../api/client/admin';
import { ApiError, errorMessage } from '../../api/client/http';
import { formatTime, shortId, yesNo } from '../../components/format';
import { ModelStatusTag, RequestStatusTag, ServiceStatusTag } from '../../components/StatusTag';
import { ResourceGrid } from '../../components/ResourceGrid';
import { WaitingQueue } from '../../components/WaitingQueue';
import { EmptyState, PageHeader } from '../../components/PageLayout';
import { useRealtime } from '../../realtime/RealtimeProvider';

function OverviewSkeleton() {
  return <div className="inline-stack" aria-busy="true" aria-label="正在加载运行状态"><SkeletonText heading width="35%" /><SkeletonPlaceholder style={{ width: '100%', height: '8rem' }} /><SkeletonPlaceholder style={{ width: '100%', height: '18rem' }} /></div>;
}

export default function OverviewRoute() {
  const operations = useOperations();
  const snapshot = useSnapshot();
  const queryClient = useQueryClient();
  const realtime = useRealtime();
  const [stopOpen, setStopOpen] = useState(false);
  const [nameDraft, setNameDraft] = useState('');
  const [nameTouched, setNameTouched] = useState(false);
  const [nameMessage, setNameMessage] = useState<string | null>(null);
  const [acceptedMessage, setAcceptedMessage] = useState<string | null>(null);
  const [lastAction, setLastAction] = useState<'start' | 'stop' | null>(null);
  const [retryReady, setRetryReady] = useState(true);
  const stopTrigger = useRef<HTMLButtonElement | null>(null);
  const actionError = useRef<HTMLDivElement | null>(null);
  const modalError = useRef<HTMLDivElement | null>(null);
  const previousModelState = useRef<string | null>(null);

  const refreshAfterAction = async () => {
    await Promise.allSettled([
      queryClient.invalidateQueries({ queryKey: adminKeys.snapshot }),
      queryClient.invalidateQueries({ queryKey: adminKeys.operations }),
    ]);
  };
  const lifecycle = useMutation({
    mutationFn: (action: 'start' | 'stop') => action === 'start' ? adminApi.startModel() : adminApi.stopModel(),
    onSuccess: async (accepted, action) => {
      setLastAction(action);
      setAcceptedMessage(`${action === 'start' ? '启动' : '停止'}操作已受理（操作标识 ${shortId(accepted.operation_id)}），完成结果以后端状态为准。`);
      if (action === 'stop') setStopOpen(false);
      await refreshAfterAction();
      realtime.announce(`${action === 'start' ? '启动' : '停止'}操作已受理，正在等待权威结果。`);
      window.requestAnimationFrame(() => {
        if (stopTrigger.current?.isConnected) stopTrigger.current.focus();
        else document.getElementById('model-control-heading')?.focus();
      });
    },
    onError: async () => {
      await refreshAfterAction();
      window.requestAnimationFrame(() => (stopOpen ? modalError.current : actionError.current)?.focus());
    },
  });
  const actionErrorDetail = lifecycle.error instanceof ApiError ? lifecycle.error.detail : null;

  useEffect(() => { if (snapshot.data) setNameDraft(snapshot.data.model.public_model_id); }, [snapshot.data?.model.public_model_id]);
  const nameChange = useMutation({
    mutationFn: ({ next, expected }: { next: string; expected: string }) => adminApi.setModelName(next, expected),
    onSuccess: async (saved) => {
      setNameMessage(`已保存模型名 ${saved.public_model_id}。新请求必须使用这个名称。`);
      await queryClient.invalidateQueries({ queryKey: adminKeys.snapshot });
    },
    onError: async () => { setNameMessage(null); await queryClient.invalidateQueries({ queryKey: adminKeys.snapshot }); },
  });

  useEffect(() => {
    const delaySeconds = actionErrorDetail?.retry_after_seconds ?? 0;
    if (delaySeconds <= 0) {
      setRetryReady(true);
      return;
    }
    setRetryReady(false);
    const timer = window.setTimeout(() => setRetryReady(true), delaySeconds * 1000);
    return () => window.clearTimeout(timer);
  }, [actionErrorDetail?.retry_after_seconds]);

  useEffect(() => {
    const nextState = snapshot.data?.model.state;
    if (!nextState) return;
    const previous = previousModelState.current;
    previousModelState.current = nextState;
    if (previous === null || previous === nextState) return;
    if (nextState === 'ready') realtime.announce('模型已就绪。');
    else if (nextState === 'unloaded') realtime.announce('模型已停止并卸载。');
    else if (nextState === 'unavailable') realtime.announce('模型当前不可用。');
    else if (nextState === 'starting') realtime.announce('模型正在加载并预热。');
    else if (nextState === 'stopping') realtime.announce('正在取消请求、清空队列并卸载模型。');
    else realtime.announce('控制台收到无法识别的模型状态。');
  }, [realtime, snapshot.data?.model.state]);

  if (snapshot.isPending) return <section className="page"><PageHeader eyebrow="SYSTEM OVERVIEW" title="运行总览" /><OverviewSkeleton /></section>;
  if (snapshot.isError && !snapshot.data) {
    return <section className="page"><PageHeader eyebrow="SYSTEM OVERVIEW" title="运行总览" /><div className="inline-stack"><InlineNotification hideCloseButton kind="error" role="alert" title="无法获取运行状态" subtitle={errorMessage(snapshot.error, '当前没有可安全展示的运行数据。')} /><div><Button kind="tertiary" onClick={() => void snapshot.refetch()}>重新读取</Button></div></div></section>;
  }
  if (!snapshot.data) return null;

  const data = snapshot.data;
  const modelState = data.model.state as string;
  const canStart = modelState === 'unloaded' || modelState === 'unavailable';
  const canStop = modelState === 'ready' || modelState === 'unavailable';
  const transition = modelState === 'starting' || modelState === 'stopping';
  const unknownState = !['unloaded', 'starting', 'ready', 'stopping', 'unavailable'].includes(modelState);
  const validName = /^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$/.test(nameDraft);

  return (
    <section className="page">
      <PageHeader eyebrow="SYSTEM OVERVIEW" title="运行总览" description="查看服务健康、模型状态与请求负载。" meta={<div className="overview-live"><span aria-hidden="true" /><strong>{realtime.state === 'connected' ? '实时同步' : '连接恢复中'}</strong><small>{formatTime(data.generated_at)}</small></div>} />

      <div className="section status-strip" aria-label="当前状态">
        <div className="status-cell"><span className="status-label">服务状态</span><div className="status-value"><ServiceStatusTag state={data.service.state} /></div></div>
        <div className="status-cell"><span className="status-label">模型状态</span><div className="status-value"><ModelStatusTag state={modelState} /></div></div>
        <div className="status-cell"><span className="status-label">快照时间</span><div className="status-value">{formatTime(data.generated_at)}</div></div>
        <div className="status-cell"><span className="status-label">更新连接</span><div className="status-value">{realtime.state === 'connected' ? '实时更新已连接' : realtime.state === 'stale' ? '更新已断开' : realtime.state === 'compatibility_error' ? '更新契约不兼容' : '正在恢复连接'}</div></div>
      </div>

      {(acceptedMessage || lifecycle.isError) && (
        <div className="section" ref={actionError} tabIndex={lifecycle.isError ? -1 : undefined}>
          <InlineNotification
            hideCloseButton
            kind={lifecycle.isError ? 'error' : 'info'}
            role={lifecycle.isError ? 'alert' : 'status'}
            title={lifecycle.isError ? '生命周期操作未完成' : '操作已受理'}
            subtitle={lifecycle.isError ? `${errorMessage(lifecycle.error, '模型状态仍以后端当前结果为准。')}${actionErrorDetail?.retry_after_seconds ? ` 请在 ${actionErrorDetail.retry_after_seconds} 秒后重试。` : ''}` : acceptedMessage ?? ''}
          />
          {actionErrorDetail?.retryable && lastAction && retryReady && <Button kind="tertiary" onClick={() => lifecycle.mutate(lastAction)}>重试</Button>}
        </div>
      )}

      <div className="section overview-grid">
        <article className="panel model-panel">
          <h2 id="model-control-heading" tabIndex={-1}>模型控制</h2>
          <ModelStatusTag state={modelState} />
          <p className="model-identity">当前对外名称：{data.model.public_model_id}</p>
          <p className="muted">原始名称：{data.model.default_public_model_id}</p>
          <p className="safe-message">
            {modelState === 'unloaded' && '模型未加载，新的推理请求将被拒绝且不会排队。'}
            {modelState === 'starting' && '正在加载并预热模型。完成前不能接收推理请求。'}
            {modelState === 'ready' && '模型已完成加载和预热，可以接收推理请求。'}
            {modelState === 'stopping' && '正在取消活跃请求、清空等待队列并卸载模型。'}
            {modelState === 'unavailable' && `模型当前无法安全提供推理。${data.model.failure ? ` ${data.model.failure.message}（${data.model.failure.code}）` : ''}`}
            {unknownState && '控制台无法识别后端状态，请刷新或检查服务。'}
          </p>
          <p className="muted">最近状态变化：{formatTime(data.model.transition_started_at)}</p>
          <div className="actions">
            {canStart && <Button renderIcon={Play} disabled={lifecycle.isPending || transition || unknownState} onClick={() => { setAcceptedMessage(null); lifecycle.reset(); lifecycle.mutate('start'); }}>{lifecycle.isPending ? '正在提交…' : '启动模型'}</Button>}
            {canStop && <Button ref={stopTrigger} kind="danger" renderIcon={StopFilledAlt} disabled={lifecycle.isPending || transition || unknownState} onClick={() => { setAcceptedMessage(null); lifecycle.reset(); setStopOpen(true); }}>停止模型</Button>}
            {transition && <Tag type="blue">正在等待权威操作结果</Tag>}
          </div>
          <form className="model-name-form" onSubmit={(event) => { event.preventDefault(); if (validName && nameDraft !== data.model.public_model_id && !nameChange.isPending) nameChange.mutate({ next: nameDraft, expected: data.model.public_model_id }); }}>
            <TextInput id="public-model-name" labelText="自定义对外模型名" helperText="保存后，新请求只能使用新名称；旧名称立即失效。允许 1–128 位英文字母、数字、点、下划线、冒号、斜杠或短横线。" value={nameDraft} maxLength={128} disabled={nameChange.isPending} onChange={(event) => { setNameTouched(true); setNameDraft(event.target.value); setNameMessage(null); nameChange.reset(); }} invalid={nameTouched && !validName} invalidText="模型名格式无效。" />
            <div className="actions">
              <Button type="submit" disabled={!validName || nameDraft === data.model.public_model_id || nameChange.isPending || snapshot.isError}>{nameChange.isPending ? '正在保存…' : '保存名称'}</Button>
              <Button type="button" kind="ghost" disabled={nameChange.isPending || nameDraft === data.model.default_public_model_id} onClick={() => setNameDraft(data.model.default_public_model_id)}>填入原始名称</Button>
            </div>
            {nameMessage && <InlineNotification lowContrast hideCloseButton kind="success" role="status" title="模型名已更新" subtitle={nameMessage} />}
            {nameChange.isError && <InlineNotification lowContrast hideCloseButton kind="error" role="alert" title="模型名未更新" subtitle={errorMessage(nameChange.error, '请刷新后重试。')} />}
          </form>
        </article>

        <article className="panel execution-panel">
          <div className="panel-heading-row"><h2>当前执行</h2><span className="queue-count">{data.queue.depth} / {data.queue.capacity}</span></div>
          <p className="muted">单请求串行执行，其他请求按 FIFO 等待。</p>
          <div className="queue-meter" aria-label={`等待队列已使用 ${data.queue.depth}，容量 ${data.queue.capacity}`}><span style={{ width: `${Math.min(100, (data.queue.depth / data.queue.capacity) * 100)}%` }} /></div>
          {data.queue.active ? (
            <dl className="definition-grid">
              <div className="definition-block"><dt>请求标识</dt><dd className="code" title={data.queue.active.id}>{shortId(data.queue.active.id)}</dd></div>
              <div className="definition-block"><dt>状态</dt><dd><RequestStatusTag state={data.queue.active.status} /></dd></div>
              <div className="definition-block"><dt>端点</dt><dd>{data.queue.active.endpoint}</dd><dt>流式 / 推理</dt><dd>{yesNo(data.queue.active.stream)} / {yesNo(data.queue.active.reasoning_enabled)}</dd></div>
            </dl>
          ) : <EmptyState>当前没有活跃请求。</EmptyState>}
        </article>
      </div>

      <section className="section"><div className="section-heading"><div><h2>资源摘要</h2><p>这些值仅代表标注的应用、DMR 进程与项目存储范围。</p></div><Button href="/metrics" kind="ghost" renderIcon={ArrowRight}>查看趋势</Button></div><ResourceGrid resources={data.resources} compact /></section>

      <div className="overview-tail-grid">
        <section className="section tail-panel" aria-labelledby="waiting-queue-heading">
          <div className="section-heading"><div><h2 id="waiting-queue-heading" tabIndex={-1}>FIFO 等待队列</h2><p>显示前 5 项，顺位由后端管理。</p></div><Button href="/requests" kind="ghost" renderIcon={ArrowRight}>查看全部</Button></div>
          <WaitingQueue waiting={data.queue.waiting} limit={5} />
        </section>

        <section className="section tail-panel attention-panel"><div className="section-heading"><div><h2>运行关注项</h2><p>告警与数据保护状态。</p></div></div><div className="attention-list"><a href="/alerts" className="attention-item"><span>当前告警</span><strong>{data.active_alert_count === 0 ? '无告警' : `${data.active_alert_count} 个`}</strong><ArrowRight size={18} aria-hidden="true" /></a><a href="/data" className="attention-item"><span>最近备份</span><strong>{operations.isError ? '读取失败' : operations.data ? `${operations.data.backup.outcome === 'succeeded' ? '成功' : operations.data.backup.outcome === 'failed' ? '失败' : '待确认'} · ${formatTime(operations.data.backup.last_run_at)}` : '读取中'}</strong><ArrowRight size={18} aria-hidden="true" /></a></div></section>
      </div>

      <Modal
        danger
        open={stopOpen}
        modalHeading="停止模型？"
        primaryButtonText={lifecycle.isPending ? '正在提交…' : '停止模型'}
        secondaryButtonText="返回"
        primaryButtonDisabled={lifecycle.isPending}
        preventCloseOnClickOutside={lifecycle.isPending}
        onRequestClose={() => { if (!lifecycle.isPending) setStopOpen(false); }}
        onRequestSubmit={() => { if (!lifecycle.isPending) lifecycle.mutate('stop'); }}
      >
        {(realtime.state === 'stale' || snapshot.isError) && <InlineNotification lowContrast hideCloseButton kind="warning" title="当前数据可能已过期" subtitle="后端会在执行时重新校验。" />}
        <p>这将取消当前活跃请求、清空当前等待队列并卸载模型。受影响的请求不会自动重放。实际结果以后端执行时的状态为准。</p>
        <p>当前观察到：活跃请求 {data.queue.active ? 1 : 0} 个，等待请求 {data.queue.depth} 个。采样时间：{formatTime(data.generated_at)}。</p>
        {lifecycle.isError && <div ref={modalError} tabIndex={-1}><InlineNotification hideCloseButton kind="error" role="alert" title="停止操作未完成" subtitle={errorMessage(lifecycle.error, '模型状态仍以后端当前结果为准。')} /></div>}
      </Modal>
    </section>
  );
}
