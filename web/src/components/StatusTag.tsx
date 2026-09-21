import { Tag } from '@carbon/react';
import { CheckmarkFilled, ErrorFilled, InProgress, SubtractAlt, WarningAlt } from '@carbon/icons-react';
import type { ReactNode } from 'react';
import type { ModelState, RequestStatus, ServiceState } from '../api/generated/admin';

interface StatusView { label: string; type: 'green' | 'blue' | 'gray' | 'red' | 'warm-gray' | 'cool-gray'; icon: ReactNode }

const MODEL_STATUS: Record<ModelState, StatusView> = {
  unloaded: { label: '已卸载', type: 'gray', icon: <SubtractAlt /> },
  starting: { label: '启动中', type: 'blue', icon: <InProgress /> },
  ready: { label: '就绪', type: 'green', icon: <CheckmarkFilled /> },
  stopping: { label: '停止中', type: 'blue', icon: <InProgress /> },
  unavailable: { label: '不可用', type: 'warm-gray', icon: <WarningAlt /> },
};

const SERVICE_STATUS: Record<ServiceState, StatusView> = {
  starting: { label: '服务启动中', type: 'blue', icon: <InProgress /> },
  ready: { label: '服务就绪', type: 'green', icon: <CheckmarkFilled /> },
  degraded: { label: '服务降级', type: 'warm-gray', icon: <WarningAlt /> },
  stopping: { label: '服务停止中', type: 'blue', icon: <InProgress /> },
};

const REQUEST_STATUS: Record<RequestStatus, StatusView> = {
  waiting: { label: '等待', type: 'blue', icon: <InProgress /> },
  active: { label: '活跃', type: 'blue', icon: <InProgress /> },
  succeeded: { label: '成功', type: 'green', icon: <CheckmarkFilled /> },
  failed: { label: '失败', type: 'red', icon: <ErrorFilled /> },
  cancelled: { label: '已取消', type: 'gray', icon: <SubtractAlt /> },
  queue_timeout: { label: '排队超时', type: 'warm-gray', icon: <WarningAlt /> },
  interrupted: { label: '已中断', type: 'warm-gray', icon: <WarningAlt /> },
  rejected: { label: '准入前拒绝', type: 'red', icon: <ErrorFilled /> },
};

function UnknownStatus({ raw }: { raw: string }) {
  return <Tag type="cool-gray" title={`未识别的后端值：${raw}`}><WarningAlt /> 无法识别</Tag>;
}

export function ModelStatusTag({ state }: { state: string }) {
  const view = MODEL_STATUS[state as ModelState];
  return view ? <Tag type={view.type}>{view.icon} {view.label}</Tag> : <UnknownStatus raw={state} />;
}

export function ServiceStatusTag({ state }: { state: string }) {
  const view = SERVICE_STATUS[state as ServiceState];
  return view ? <Tag type={view.type}>{view.icon} {view.label}</Tag> : <UnknownStatus raw={state} />;
}

export function RequestStatusTag({ state }: { state: string }) {
  const view = REQUEST_STATUS[state as RequestStatus];
  return view ? <Tag type={view.type}>{view.icon} {view.label}</Tag> : <UnknownStatus raw={state} />;
}
