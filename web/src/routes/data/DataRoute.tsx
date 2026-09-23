import { InlineNotification, SkeletonPlaceholder, Tag } from '@carbon/react';
import { useOperations } from '../../api/client/hooks';
import { errorMessage } from '../../api/client/http';
import { absoluteTime, formatNumber, shortId } from '../../components/format';
import { DataRegion, EmptyState, PageHeader } from '../../components/PageLayout';

function Outcome({ value }: { value: string }) {
  if (value === 'succeeded') return <Tag type="green">成功</Tag>;
  if (value === 'failed') return <Tag type="red">失败</Tag>;
  if (value === 'unknown') return <Tag type="cool-gray">尚无可确认的证据</Tag>;
  return <Tag type="cool-gray">无法识别</Tag>;
}

const OPERATION_LABEL: Record<string, string> = { start: '启动', stop: '停止', reconcile_unload: '启动协调卸载' };
const STATUS_LABEL: Record<string, string> = { running: '进行中', succeeded: '成功', failed: '失败', indeterminate: '结果不确定', interrupted: '已中断' };

export default function DataRoute() {
  const operations = useOperations();
  if (operations.isPending) return <section className="page"><PageHeader eyebrow="DATA PROTECTION" title="数据与备份" /><SkeletonPlaceholder style={{ width: '100%', height: '24rem' }} /></section>;
  if (operations.isError && !operations.data) return <section className="page"><PageHeader eyebrow="DATA PROTECTION" title="数据与备份" /><InlineNotification hideCloseButton kind="error" title="无法获取数据与备份证据" subtitle={errorMessage(operations.error, '当前不能确认保留、备份或恢复状态。')} /></section>;
  if (!operations.data) return null;

  const { retention, backup, restore_proof: restore, model_operations: modelOperations } = operations.data;
  return (
    <section className="page">
      <PageHeader eyebrow="DATA PROTECTION" title="数据与备份" description="检查不含正文的元数据保留、每日逻辑备份、7 天备份保留和最近恢复证明。本页不提供恢复或下载操作。" />
      <section className="section content-card"><div className="section-heading"><div><h2>请求元数据保留 30 天</h2><p>只有早于严格截止边界的元数据会被清理。</p></div><Outcome value={retention.outcome} /></div><dl className="definition-grid"><div className="definition-block"><dt>截止时间</dt><dd>{absoluteTime(retention.cutoff_at)}</dd></div><div className="definition-block"><dt>最近执行</dt><dd>{absoluteTime(retention.last_run_at)}</dd></div><div className="definition-block"><dt>删除行数</dt><dd>{retention.deleted_rows === null ? '尚无可确认的证据' : formatNumber(retention.deleted_rows)}</dd></div></dl></section>
      <section className="section content-card"><div className="section-heading"><div><h2>每日逻辑备份，仅保留最近 7 天</h2><p>制品仅显示 basename；控制台不暴露路径、大小或下载入口。</p></div><Outcome value={backup.outcome} /></div><dl className="definition-grid"><div className="definition-block"><dt>最近执行</dt><dd>{absoluteTime(backup.last_run_at)}</dd><dt>下次预计</dt><dd>{absoluteTime(backup.next_expected_at)}</dd></div><div className="definition-block"><dt>备份制品</dt><dd className="code">{backup.artifact_name ?? '尚无可确认的证据'}</dd></div><div className="definition-block"><dt>保留数量</dt><dd>{backup.retained_count === null ? '尚无可确认的证据' : formatNumber(backup.retained_count)}</dd></div></dl></section>
      <section className="section content-card"><div className="section-heading"><div><h2>最近恢复证明</h2><p>恢复只由隔离的受控运维流程执行，不能从浏览器发起。</p></div><Outcome value={restore.outcome} /></div><dl className="definition-grid"><div className="definition-block"><dt>最近执行</dt><dd>{absoluteTime(restore.last_run_at)}</dd><dt>恢复请求数</dt><dd>{restore.restored_request_count === null ? '尚无可确认的证据' : formatNumber(restore.restored_request_count)}</dd></div><div className="definition-block"><dt>输入 token</dt><dd>{restore.restored_token_totals ? formatNumber(restore.restored_token_totals.input) : '尚无可确认的证据'}</dd><dt>输出 token</dt><dd>{restore.restored_token_totals ? formatNumber(restore.restored_token_totals.output) : '尚无可确认的证据'}</dd></div><div className="definition-block"><dt>推理 token</dt><dd>{restore.restored_token_totals ? formatNumber(restore.restored_token_totals.reasoning) : '尚无可确认的证据'}</dd></div></dl></section>
      <section className="section content-card" aria-labelledby="operations-heading"><div className="section-heading"><div><h2 id="operations-heading">模型操作记录</h2><p>用于确认生命周期动作最终结果；受理不代表完成。</p></div></div>{modelOperations.length === 0 ? <EmptyState>当前没有模型操作记录。</EmptyState> : <DataRegion labelledBy="operations-heading" description="表格可横向滚动。"><table className="data-table"><caption>模型生命周期操作结果</caption><thead><tr><th scope="col">请求时间</th><th scope="col">操作标识</th><th scope="col">操作</th><th scope="col">状态</th><th scope="col">完成时间</th><th scope="col">观察状态</th><th scope="col">结果代码</th></tr></thead><tbody>{modelOperations.map((operation) => <tr key={operation.id}><td>{absoluteTime(operation.requested_at)}</td><td className="code" title={operation.id}>{shortId(operation.id)}</td><td>{OPERATION_LABEL[operation.operation] ?? '无法识别'}</td><td>{STATUS_LABEL[operation.status] ?? '无法识别'}</td><td>{absoluteTime(operation.completed_at)}</td><td>{operation.observed_state === 'loaded' ? '已加载' : operation.observed_state === 'unloaded' ? '已卸载' : operation.observed_state === 'unknown' || operation.observed_state === null ? '未知' : '无法识别'}</td><td className="code">{operation.result_code ?? '不适用'}</td></tr>)}</tbody></table></DataRegion>}</section>
    </section>
  );
}
