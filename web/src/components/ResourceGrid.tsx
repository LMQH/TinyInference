import { InlineNotification, Tag } from '@carbon/react';
import type { ResourceSnapshot, ResourceSource } from '../api/generated/admin';
import { formatBytes, formatNumber, formatTime } from './format';

const SOURCE_LABEL: Record<ResourceSource, string> = {
  api_container: '来源：API 容器',
  dmr_process: '来源：DMR 模型进程',
  project_storage: '来源：项目/模型/运行时存储',
  unavailable: '来源：无法获取',
};

function sourceLabel(source: string, metal = false): string {
  if (metal && source === 'dmr_process') return '来源：DMR 推理引擎';
  return SOURCE_LABEL[source as ResourceSource] ?? '来源：无法识别';
}

export interface ResourceGridProps { resources: ResourceSnapshot; compact?: boolean }

export function ResourceGrid({ resources, compact = false }: ResourceGridProps) {
  const sample = formatTime(resources.sampled_at);
  const metal = resources.metal === 'enabled' ? '已启用' : resources.metal === 'disabled' ? '未启用' : '无法获取';
  const memory = resources.unified_memory_used_bytes === null
    ? '无法获取'
    : `${formatBytes(resources.unified_memory_used_bytes)} / ${formatBytes(resources.unified_memory_total_bytes)}`;
  const disk = resources.disk_used_bytes === null
    ? '无法获取'
    : `${formatBytes(resources.disk_used_bytes)} / ${formatBytes(resources.disk_total_bytes)}`;

  return (
    <div className="inline-stack">
      {(resources.status === 'stale' || resources.status === 'partial' || resources.status === 'unavailable') && (
        <InlineNotification
          lowContrast
          hideCloseButton
          kind={resources.status === 'stale' ? 'warning' : 'info'}
          title={resources.status === 'stale' ? '资源样本已陈旧' : resources.status === 'partial' ? '部分资源暂不可用' : '资源指标暂不可用'}
          subtitle={resources.reason_code ? `安全诊断代码：${resources.reason_code}` : '可用字段仍按各自来源展示，缺失值不会显示为 0。'}
        />
      )}
      <div className="resource-grid" aria-label="有来源的资源状态">
        <article className="resource">
          <h3>API 容器 CPU</h3>
          <p className="resource-value">{formatNumber(resources.cpu_percent, '%')}</p>
          <div className="resource-meta"><span>{sourceLabel(resources.cpu_source)}</span>{!compact && <span>采样：{sample}</span>}</div>
        </article>
        <article className="resource">
          <h3>DMR 模型进程统一内存</h3>
          <p className="resource-value">{memory}</p>
          <div className="resource-meta"><span>{sourceLabel(resources.unified_memory_source)}</span>{!compact && <span>采样：{sample}</span>}</div>
        </article>
        <article className="resource">
          <h3>项目/模型/运行时存储</h3>
          <p className="resource-value">{disk}</p>
          <div className="resource-meta"><span>{sourceLabel(resources.disk_source)}</span>{!compact && <span>采样：{sample}</span>}</div>
        </article>
        <article className="resource">
          <h3>DMR 推理引擎 Metal</h3>
          <p className="resource-value"><Tag type={resources.metal === 'enabled' ? 'green' : resources.metal === 'disabled' ? 'gray' : 'cool-gray'}>{metal}</Tag></p>
          <div className="resource-meta"><span>{sourceLabel(resources.metal_source, true)}</span>{!compact && <span>采样：{sample}</span>}</div>
        </article>
      </div>
    </div>
  );
}
