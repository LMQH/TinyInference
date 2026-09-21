import { Button, InlineNotification, Select, SelectItem, SkeletonPlaceholder, Tag } from '@carbon/react';
import { ArrowLeft, ArrowRight } from '@carbon/icons-react';
import { useSearchParams } from 'react-router-dom';
import { useAlerts, useLogs } from '../../api/client/hooks';
import { errorMessage } from '../../api/client/http';
import type { AlertSeverity, LogLevel } from '../../api/generated/admin';
import { absoluteTime, formatNumber, shortId } from '../../components/format';

const SEVERITY: Record<AlertSeverity, { label: string; type: 'blue' | 'warm-gray' | 'red' }> = {
  info: { label: '信息', type: 'blue' }, warning: { label: '警告', type: 'warm-gray' }, critical: { label: '严重', type: 'red' },
};
const LEVELS: readonly LogLevel[] = ['info', 'warning', 'error'];

export default function AlertsRoute() {
  const [search, setSearch] = useSearchParams();
  const levelValue = search.get('level');
  const level: LogLevel = LEVELS.includes(levelValue as LogLevel) ? levelValue as LogLevel : 'warning';
  const alertCursor = search.get('alert_cursor');
  const logCursor = search.get('log_cursor');
  const alerts = useAlerts(alertCursor);
  const logs = useLogs(level, logCursor);
  const updateSearch = (values: Record<string, string | null>) => {
    const next = new URLSearchParams(search);
    Object.entries(values).forEach(([key, value]) => value === null ? next.delete(key) : next.set(key, value));
    setSearch(next);
  };

  return (
    <section className="page">
      <header className="page-header"><h1>告警与日志</h1><p>仅展示后端净化的固定字段。告警不会发送至邮件、即时通讯、桌面通知或 webhook。</p></header>
      <section className="section" aria-labelledby="alerts-heading">
        <div className="section-heading"><div><h2 id="alerts-heading">告警</h2><p>按生产者顺序显示，不提供客户端筛选。</p></div></div>
        {alerts.isPending && <SkeletonPlaceholder style={{ width: '100%', height: '12rem' }} />}
        {alerts.isError && !alerts.data && <InlineNotification hideCloseButton kind="error" title="无法获取告警" subtitle={errorMessage(alerts.error, '告警读取失败，不能据此判断当前无告警。')} />}
        {alerts.data?.items.length === 0 && <p className="empty-state">当前无告警。</p>}
        {alerts.data && alerts.data.items.length > 0 && <div className="table-scroll" role="region" aria-labelledby="alerts-heading" tabIndex={0}><p className="table-description">表格可横向滚动。颜色之外同时提供严重度文本。</p><table className="data-table"><caption>控制台内告警</caption><thead><tr><th scope="col">严重度</th><th scope="col">状态</th><th scope="col">代码</th><th scope="col">安全摘要</th><th scope="col">首次发生</th><th scope="col">最近发生</th><th scope="col">次数</th></tr></thead><tbody>{alerts.data.items.map((alert) => { const severity = SEVERITY[alert.severity] ?? { label: '无法识别', type: 'warm-gray' as const }; return <tr key={alert.id}><td><Tag type={severity.type}>{severity.label}</Tag></td><td>{alert.state === 'active' ? '当前' : alert.state === 'resolved' ? '已解决' : '无法识别'}</td><td className="code">{alert.code}</td><td>{alert.message}</td><td>{absoluteTime(alert.first_seen_at)}</td><td>{absoluteTime(alert.last_seen_at)}</td><td>{formatNumber(alert.occurrences)}</td></tr>; })}</tbody></table></div>}
        <div className="pagination-row">{alertCursor && <Button kind="secondary" renderIcon={ArrowLeft} onClick={() => updateSearch({ alert_cursor: null })}>告警第一页</Button>}{alerts.data?.next_cursor && <Button renderIcon={ArrowRight} onClick={() => updateSearch({ alert_cursor: alerts.data.next_cursor })}>更多告警</Button>}</div>
      </section>

      <section className="section" aria-labelledby="logs-heading">
        <div className="section-heading"><div><h2 id="logs-heading">结构化日志</h2><p>只呈现固定安全字段；不渲染任意 JSON、HTML、Markdown 或堆栈。</p></div><Select id="log-level" labelText="日志级别" value={level} onChange={(event) => updateSearch({ level: event.target.value, log_cursor: null })}><SelectItem value="info" text="信息" /><SelectItem value="warning" text="警告" /><SelectItem value="error" text="错误" /></Select></div>
        {logs.isPending && <SkeletonPlaceholder style={{ width: '100%', height: '18rem' }} />}
        {logs.isError && !logs.data && <InlineNotification hideCloseButton kind="error" title="无法获取结构化日志" subtitle={errorMessage(logs.error, '当前日志区域暂不可用。')} />}
        {logs.data?.items.length === 0 && <p className="empty-state">当前日志级别下没有日志。</p>}
        {logs.data && logs.data.items.length > 0 && <div className="table-scroll" role="region" aria-labelledby="logs-heading" tabIndex={0}><p className="table-description">表格可横向滚动。消息以纯文本呈现。</p><table className="data-table"><caption>内容安全的结构化日志</caption><thead><tr><th scope="col">时间</th><th scope="col">级别</th><th scope="col">事件代码</th><th scope="col">安全消息</th><th scope="col">请求标识</th><th scope="col">操作标识</th></tr></thead><tbody>{logs.data.items.map((log) => <tr key={log.id}><td>{absoluteTime(log.occurred_at)}</td><td>{log.level === 'info' ? '信息' : log.level === 'warning' ? '警告' : log.level === 'error' ? '错误' : '无法识别'}</td><td className="code">{log.event_code}</td><td>{log.message}</td><td className="code" title={log.request_id ?? ''}>{log.request_id ? shortId(log.request_id) : '不适用'}</td><td className="code" title={log.operation_id ?? ''}>{log.operation_id ? shortId(log.operation_id) : '不适用'}</td></tr>)}</tbody></table></div>}
        <div className="pagination-row">{logCursor && <Button kind="secondary" renderIcon={ArrowLeft} onClick={() => updateSearch({ log_cursor: null })}>日志第一页</Button>}{logs.data?.next_cursor && <Button renderIcon={ArrowRight} onClick={() => updateSearch({ log_cursor: logs.data.next_cursor })}>更多日志</Button>}</div>
      </section>
    </section>
  );
}
