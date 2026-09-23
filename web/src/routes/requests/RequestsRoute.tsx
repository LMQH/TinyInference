import { Button, InlineNotification, SkeletonPlaceholder } from '@carbon/react';
import { ArrowLeft, ArrowRight } from '@carbon/icons-react';
import { useSearchParams } from 'react-router-dom';
import { useRequests, useSnapshot } from '../../api/client/hooks';
import { errorMessage } from '../../api/client/http';
import { absoluteTime, formatDuration, formatNumber, shortId, yesNo } from '../../components/format';
import { RequestStatusTag } from '../../components/StatusTag';
import { WaitingQueue } from '../../components/WaitingQueue';
import { ContentCard, DataRegion, EmptyState, PageHeader, SectionHeader } from '../../components/PageLayout';

export default function RequestsRoute() {
  const [search, setSearch] = useSearchParams();
  const cursor = search.get('cursor');
  const snapshot = useSnapshot();
  const requests = useRequests(cursor);

  return (
    <section className="page">
      <PageHeader eyebrow="REQUEST OPERATIONS" title="请求与队列" description="观察唯一活跃请求、后端权威 FIFO 等待顺位和不含正文的最近请求元数据。" />

      <ContentCard labelledBy="active-heading">
        <SectionHeader id="active-heading" title="活跃请求" description="最多 1 个；没有活跃项不代表模型已就绪。" />
        {snapshot.isPending && <SkeletonPlaceholder style={{ width: '100%', height: '7rem' }} />}
        {snapshot.isError && !snapshot.data && <InlineNotification hideCloseButton kind="error" title="无法获取活跃请求" subtitle={errorMessage(snapshot.error, '当前没有可用的队列快照。')} />}
        {snapshot.data?.queue.active ? (
          <dl className="definition-grid">
            <div className="definition-block"><dt>请求标识</dt><dd className="code" title={snapshot.data.queue.active.id}>{shortId(snapshot.data.queue.active.id)}</dd><dt>状态</dt><dd><RequestStatusTag state={snapshot.data.queue.active.status} /></dd></div>
            <div className="definition-block"><dt>端点</dt><dd>{snapshot.data.queue.active.endpoint}</dd><dt>流式 / 推理</dt><dd>{yesNo(snapshot.data.queue.active.stream)} / {yesNo(snapshot.data.queue.active.reasoning_enabled)}</dd></div>
            <div className="definition-block"><dt>入队时间</dt><dd>{absoluteTime(snapshot.data.queue.active.enqueued_at)}</dd><dt>开始时间</dt><dd>{absoluteTime(snapshot.data.queue.active.started_at)}</dd></div>
          </dl>
        ) : snapshot.data ? <EmptyState>当前没有活跃请求。</EmptyState> : null}
      </ContentCard>

      <ContentCard labelledBy="waiting-queue-heading">
        <SectionHeader id="waiting-queue-heading" title="完整等待队列" description={<>当前等待深度 {snapshot.data?.queue.depth ?? '无法获取'} / {snapshot.data?.queue.capacity ?? 20}。</>} />
        {snapshot.isPending && <SkeletonPlaceholder style={{ width: '100%', height: '12rem' }} />}
        {snapshot.isError && !snapshot.data && <InlineNotification hideCloseButton kind="error" title="无法获取队列状态" subtitle={errorMessage(snapshot.error, '当前没有可安全展示的等待顺位。')} />}
        {snapshot.data && <WaitingQueue waiting={snapshot.data.queue.waiting} />}
      </ContentCard>

      <ContentCard labelledBy="history-heading">
        <SectionHeader id="history-heading" title="最近请求元数据" description="固定按创建时间与请求标识倒序，不提供客户端排序或正文搜索。" />
        {requests.isPending && <SkeletonPlaceholder style={{ width: '100%', height: '18rem' }} />}
        {requests.isError && !requests.data && <div className="inline-stack"><InlineNotification hideCloseButton kind="error" title="无法获取请求记录" subtitle={errorMessage(requests.error, '请求历史暂不可用。')} /><div><Button kind="tertiary" onClick={() => cursor ? setSearch({}, { replace: true }) : void requests.refetch()}>{cursor ? '回到第一页' : '重新读取'}</Button></div></div>}
        {requests.data?.items.length === 0 && <EmptyState>当前没有请求记录。</EmptyState>}
        {requests.data && requests.data.items.length > 0 && (
          <DataRegion labelledBy="history-heading" description="表格可横向滚动。顺序由后端固定，不可排序。">
            <table className="data-table">
              <caption>不含正文的最近请求元数据</caption>
              <thead><tr><th scope="col">创建时间</th><th scope="col">请求标识</th><th scope="col">公开模型</th><th scope="col">端点</th><th scope="col">流式</th><th scope="col">推理</th><th scope="col">返回工具调用</th><th scope="col">状态</th><th scope="col">HTTP</th><th scope="col">入队时间</th><th scope="col">开始时间</th><th scope="col">首 token 时间</th><th scope="col">完成时间</th><th scope="col">输入 token</th><th scope="col">输出 token</th><th scope="col">推理 token</th><th scope="col">排队</th><th scope="col">首 token</th><th scope="col">总时长</th><th scope="col">吞吐</th><th scope="col">终态代码</th></tr></thead>
              <tbody>{requests.data.items.map((request) => (
                <tr key={request.id}>
                  <td>{absoluteTime(request.created_at)}</td><td className="code" title={request.id}>{shortId(request.id)}</td><td className="code">{request.public_model_id}</td><td>{request.endpoint}</td><td>{yesNo(request.stream)}</td><td>{yesNo(request.reasoning_enabled)}</td><td>{yesNo(request.tool_calls_returned)}</td><td><RequestStatusTag state={request.status} /></td><td>{formatNumber(request.http_status)}</td><td>{absoluteTime(request.enqueued_at)}</td><td>{absoluteTime(request.started_at)}</td><td>{absoluteTime(request.first_token_at)}</td><td>{absoluteTime(request.completed_at)}</td><td>{formatNumber(request.input_tokens)}</td><td>{formatNumber(request.output_tokens)}</td><td>{formatNumber(request.reasoning_tokens)}</td><td>{formatDuration(request.queue_wait_ms)}</td><td>{formatDuration(request.ttft_ms)}</td><td>{formatDuration(request.duration_ms)}</td><td>{formatNumber(request.throughput_tokens_per_second, ' token/秒')}</td><td className="code">{request.terminal_code ?? '不适用'}</td>
                </tr>
              ))}</tbody>
            </table>
          </DataRegion>
        )}
        <div className="pagination-row">
          {cursor && <Button kind="secondary" renderIcon={ArrowLeft} onClick={() => setSearch({}, { replace: false })}>返回第一页</Button>}
          {requests.data?.next_cursor && <Button renderIcon={ArrowRight} onClick={() => setSearch({ cursor: requests.data.next_cursor ?? '' }, { replace: false })}>下一页</Button>}
        </div>
      </ContentCard>
    </section>
  );
}
