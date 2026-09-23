import { useState } from 'react';
import { Button, InlineNotification, SkeletonPlaceholder } from '@carbon/react';
import { LineChart } from '@carbon/charts-react';
import { ScaleTypes, type LineChartOptions } from '@carbon/charts';
import { useSearchParams } from 'react-router-dom';
import { METRIC_SELECTIONS, type MetricSelection } from '../../api/client/admin';
import { useMetrics, useSnapshot } from '../../api/client/hooks';
import { errorMessage } from '../../api/client/http';
import type { MetricBucket } from '../../api/generated/admin';
import { ResourceGrid } from '../../components/ResourceGrid';
import { absoluteTime, formatNumber } from '../../components/format';
import { DataRegion, EmptyState, PageHeader } from '../../components/PageLayout';

interface SeriesDefinition { label: string; value: (bucket: MetricBucket) => number | null }
interface MetricPanelProps { title: string; unit: string; buckets: MetricBucket[]; series: readonly SeriesDefinition[] }

function MetricPanel({ title, unit, buckets, series }: MetricPanelProps) {
  const [tableOpen, setTableOpen] = useState(true);
  const headingId = `metric-${title.replaceAll(/\s+/g, '-')}`;
  const data = buckets.flatMap((bucket) => series.flatMap((definition) => {
    const value = definition.value(bucket);
    return value === null ? [] : [{ group: definition.label, date: new Date(bucket.bucket_start), value }];
  }));
  const missingCount = buckets.reduce((count, bucket) => count + series.filter((definition) => definition.value(bucket) === null).length, 0);
  const options: LineChartOptions = {
    title,
    accessibility: { svgAriaLabel: `${title}趋势图，单位${unit}` },
    axes: {
      bottom: { mapsTo: 'date', scaleType: ScaleTypes.TIME },
      left: { mapsTo: 'value', scaleType: ScaleTypes.LINEAR, title: unit },
    },
    curve: 'curveLinear',
    height: '300px',
    points: { enabled: true },
    legend: { enabled: series.length > 1 },
    toolbar: { enabled: false },
    animations: !window.matchMedia('(prefers-reduced-motion: reduce)').matches,
  };

  return (
    <section className="metric-section" aria-labelledby={headingId}>
      <h3 id={headingId}>{title}</h3>
      <p className="chart-summary">单位：{unit}。共 {buckets.length} 个时间桶；{missingCount > 0 ? `${missingCount} 个值无样本，图中不补零。` : '没有缺失样本。'}</p>
      {data.length === 0 ? <EmptyState>当前时间范围没有可绘制的样本。</EmptyState> : <div className="chart-shell"><LineChart data={data} options={options} /></div>}
      <Button kind="ghost" size="sm" aria-expanded={tableOpen} onClick={() => setTableOpen((open) => !open)}>{tableOpen ? '隐藏数据表' : '查看数据表'}</Button>
      {tableOpen && (
        <DataRegion label={`${title}数据表`} description="表格可横向滚动。无样本显示为“无样本”，真实零值显示为 0。">
          <table className="data-table"><caption>{title}同源数据表</caption><thead><tr><th scope="col">时间桶</th>{series.map((definition) => <th key={definition.label} scope="col">{definition.label}（{unit}）</th>)}</tr></thead><tbody>{buckets.map((bucket) => <tr key={bucket.bucket_start}><td>{absoluteTime(bucket.bucket_start)}</td>{series.map((definition) => { const value = definition.value(bucket); return <td key={definition.label}>{value === null ? '无样本' : formatNumber(value)}</td>; })}</tr>)}</tbody></table>
        </DataRegion>
      )}
    </section>
  );
}

function readSelection(search: URLSearchParams): MetricSelection {
  const windowValue = search.get('window');
  const granularity = search.get('granularity');
  return METRIC_SELECTIONS.find((item) => item.window === windowValue && item.granularity === granularity) ?? METRIC_SELECTIONS[0]!;
}

export default function MetricsRoute() {
  const [search, setSearch] = useSearchParams();
  const selection = readSelection(search);
  const metrics = useMetrics(selection);
  const snapshot = useSnapshot();
  const rangeLabel = metrics.data ? `${absoluteTime(metrics.data.from)} 至 ${absoluteTime(metrics.data.to)}` : '正在读取';

  return (
    <section className="page">
      <PageHeader eyebrow="PERFORMANCE" title="指标与趋势" description="资源值保留生产者来源；请求、token 与性能趋势按固定聚合组合展示，不补造缺失样本。" />
      <section className="section content-card"><div className="section-heading"><div><h2>当前资源</h2><p>不提供资源历史、物理主机总量或 Metal GPU 利用率。</p></div></div>{snapshot.isPending && <SkeletonPlaceholder style={{ width: '100%', height: '12rem' }} />}{snapshot.isError && !snapshot.data && <InlineNotification hideCloseButton kind="error" title="资源指标暂不可用" subtitle={errorMessage(snapshot.error, '模型控制与请求队列不受此区域显示影响。')} />}{snapshot.data && <ResourceGrid resources={snapshot.data.resources} />}</section>
      <section className="section content-card" aria-labelledby="trends-heading">
        <div className="section-heading"><div><h2 id="trends-heading">请求、token 与性能趋势</h2><p>范围：{rangeLabel}；桶粒度：{selection.granularity}。</p></div></div>
        <div className="metric-controls" aria-label="指标时间范围">
          {METRIC_SELECTIONS.map((item) => <Button key={`${item.window}-${item.granularity}`} size="sm" kind={item.window === selection.window ? 'primary' : 'tertiary'} aria-pressed={item.window === selection.window} onClick={() => setSearch({ window: item.window, granularity: item.granularity })}>{item.window} / {item.granularity}</Button>)}
        </div>
        {metrics.isPending && <SkeletonPlaceholder style={{ width: '100%', height: '24rem' }} />}
        {metrics.isError && !metrics.data && <InlineNotification hideCloseButton kind="error" title="指标暂不可用" subtitle={errorMessage(metrics.error, '无法读取所选时间范围。')} />}
        {metrics.data?.series.length === 0 && <EmptyState>所选时间范围没有指标样本。</EmptyState>}
        {metrics.data && metrics.data.series.length > 0 && <div className="inline-stack">
          <MetricPanel title="请求量与终态" unit="请求" buckets={metrics.data.series} series={[
            { label: '请求总量', value: (bucket) => bucket.request_count },
            { label: '等待', value: (bucket) => bucket.outcomes.waiting },
            { label: '活跃', value: (bucket) => bucket.outcomes.active },
            { label: '成功', value: (bucket) => bucket.outcomes.succeeded },
            { label: '失败', value: (bucket) => bucket.outcomes.failed },
            { label: '已取消', value: (bucket) => bucket.outcomes.cancelled },
            { label: '排队超时', value: (bucket) => bucket.outcomes.queue_timeout },
            { label: '已中断', value: (bucket) => bucket.outcomes.interrupted },
            { label: '准入前拒绝', value: (bucket) => bucket.outcomes.rejected },
          ]} />
          <MetricPanel title="Token" unit="token" buckets={metrics.data.series} series={[{ label: '输入', value: (bucket) => bucket.input_tokens }, { label: '输出', value: (bucket) => bucket.output_tokens }, { label: '推理', value: (bucket) => bucket.reasoning_tokens }]} />
          <MetricPanel title="吞吐" unit="token/秒" buckets={metrics.data.series} series={[{ label: '吞吐', value: (bucket) => bucket.throughput_tokens_per_second }]} />
          <MetricPanel title="首 token 时间" unit="毫秒" buckets={metrics.data.series} series={[{ label: '平均 TTFT', value: (bucket) => bucket.ttft_ms_avg }]} />
          <MetricPanel title="请求总时长" unit="毫秒" buckets={metrics.data.series} series={[{ label: '平均总时长', value: (bucket) => bucket.duration_ms_avg }]} />
        </div>}
      </section>
      <section className="section content-card note-card"><div className="section-heading"><div><h2>性能基线说明</h2><p>页面只展示后端记录的实际测量值和条件。数值高低不代表通过或失败，MVP 没有性能阈值。</p></div></div></section>
    </section>
  );
}
