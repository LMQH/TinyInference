import type {
  AdminEvent,
  AdminSnapshot,
  AlertPage,
  AlertRecord,
  CancelResult,
  LogPage,
  LogRecord,
  MetricBucket,
  MetricsResponse,
  ModelActionAccepted,
  ModelNameSaved,
  OperationsResponse,
  RequestPage,
  RequestRecord,
  ResourceSnapshot,
  WaitingRequest,
} from '../generated/admin';
import { ContractError } from './http';

type JsonObject = Record<string, unknown>;

function object(value: unknown, name: string, keys: readonly string[]): JsonObject {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new ContractError(`${name} 必须是对象。`);
  }
  const record = value as JsonObject;
  const actual = Object.keys(record);
  if (actual.length !== keys.length || actual.some((key) => !keys.includes(key))) {
    throw new ContractError(`${name} 字段与已发布契约不一致。`);
  }
  return record;
}

function string(value: unknown, name: string): string {
  if (typeof value !== 'string') throw new ContractError(`${name} 必须是字符串。`);
  return value;
}

function nullableString(value: unknown, name: string): string | null {
  return value === null ? null : string(value, name);
}

function number(value: unknown, name: string): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) throw new ContractError(`${name} 必须是有限数字。`);
  return value;
}

function integer(value: unknown, name: string): number {
  const result = number(value, name);
  if (!Number.isSafeInteger(result)) throw new ContractError(`${name} 必须是安全整数。`);
  return result;
}

function nullableNumber(value: unknown, name: string): number | null {
  return value === null ? null : number(value, name);
}

function nullableInteger(value: unknown, name: string): number | null {
  return value === null ? null : integer(value, name);
}

function boolean(value: unknown, name: string): boolean {
  if (typeof value !== 'boolean') throw new ContractError(`${name} 必须是布尔值。`);
  return value;
}

function array(value: unknown, name: string): unknown[] {
  if (!Array.isArray(value)) throw new ContractError(`${name} 必须是数组。`);
  return value;
}

function decodeFailure(value: unknown) {
  if (value === null) return null;
  const item = object(value, 'model.failure', ['code', 'message', 'retryable']);
  return { code: string(item.code, 'failure.code'), message: string(item.message, 'failure.message'), retryable: boolean(item.retryable, 'failure.retryable') };
}

function decodeActive(value: unknown) {
  if (value === null) return null;
  const item = object(value, 'queue.active', ['id', 'status', 'endpoint', 'stream', 'reasoning_enabled', 'enqueued_at', 'started_at']);
  return {
    id: string(item.id, 'active.id'),
    status: string(item.status, 'active.status') as 'active',
    endpoint: string(item.endpoint, 'active.endpoint') as 'chat.completions',
    stream: boolean(item.stream, 'active.stream'),
    reasoning_enabled: boolean(item.reasoning_enabled, 'active.reasoning_enabled'),
    enqueued_at: string(item.enqueued_at, 'active.enqueued_at'),
    started_at: string(item.started_at, 'active.started_at'),
  };
}

function decodeWaiting(value: unknown): WaitingRequest {
  const item = object(value, 'queue.waiting[]', ['id', 'status', 'endpoint', 'stream', 'reasoning_enabled', 'position', 'enqueued_at', 'deadline_at', 'can_cancel']);
  return {
    id: string(item.id, 'waiting.id'),
    status: string(item.status, 'waiting.status') as 'waiting',
    endpoint: string(item.endpoint, 'waiting.endpoint') as WaitingRequest['endpoint'],
    stream: boolean(item.stream, 'waiting.stream'),
    reasoning_enabled: boolean(item.reasoning_enabled, 'waiting.reasoning_enabled'),
    position: integer(item.position, 'waiting.position'),
    enqueued_at: string(item.enqueued_at, 'waiting.enqueued_at'),
    deadline_at: string(item.deadline_at, 'waiting.deadline_at'),
    can_cancel: boolean(item.can_cancel, 'waiting.can_cancel'),
  };
}

function decodeResources(value: unknown): ResourceSnapshot {
  const item = object(value, 'resources', ['status', 'sampled_at', 'cpu_percent', 'cpu_source', 'unified_memory_used_bytes', 'unified_memory_total_bytes', 'unified_memory_source', 'disk_used_bytes', 'disk_total_bytes', 'disk_source', 'metal', 'metal_source', 'reason_code']);
  return {
    status: string(item.status, 'resources.status') as ResourceSnapshot['status'],
    sampled_at: nullableString(item.sampled_at, 'resources.sampled_at'),
    cpu_percent: nullableNumber(item.cpu_percent, 'resources.cpu_percent'),
    cpu_source: string(item.cpu_source, 'resources.cpu_source') as ResourceSnapshot['cpu_source'],
    unified_memory_used_bytes: nullableInteger(item.unified_memory_used_bytes, 'resources.unified_memory_used_bytes'),
    unified_memory_total_bytes: nullableInteger(item.unified_memory_total_bytes, 'resources.unified_memory_total_bytes'),
    unified_memory_source: string(item.unified_memory_source, 'resources.unified_memory_source') as ResourceSnapshot['unified_memory_source'],
    disk_used_bytes: nullableInteger(item.disk_used_bytes, 'resources.disk_used_bytes'),
    disk_total_bytes: nullableInteger(item.disk_total_bytes, 'resources.disk_total_bytes'),
    disk_source: string(item.disk_source, 'resources.disk_source') as ResourceSnapshot['disk_source'],
    metal: string(item.metal, 'resources.metal') as ResourceSnapshot['metal'],
    metal_source: string(item.metal_source, 'resources.metal_source') as ResourceSnapshot['metal_source'],
    reason_code: nullableString(item.reason_code, 'resources.reason_code'),
  };
}

export function decodeSnapshot(value: unknown): AdminSnapshot {
  const root = object(value, 'AdminSnapshot', ['snapshot_version', 'generated_at', 'service', 'model', 'queue', 'resources', 'active_alert_count']);
  const service = object(root.service, 'service', ['state', 'ready', 'authority_epoch', 'reason_code']);
  const model = object(root.model, 'model', ['state', 'transition_started_at', 'operation_id', 'failure', 'public_model_id', 'default_public_model_id']);
  const queue = object(root.queue, 'queue', ['capacity', 'depth', 'active', 'waiting']);
  return {
    snapshot_version: integer(root.snapshot_version, 'snapshot_version'),
    generated_at: string(root.generated_at, 'generated_at'),
    service: {
      state: string(service.state, 'service.state') as AdminSnapshot['service']['state'],
      ready: boolean(service.ready, 'service.ready'),
      authority_epoch: integer(service.authority_epoch, 'service.authority_epoch'),
      reason_code: nullableString(service.reason_code, 'service.reason_code'),
    },
    model: {
      state: string(model.state, 'model.state') as AdminSnapshot['model']['state'],
      transition_started_at: string(model.transition_started_at, 'model.transition_started_at'),
      operation_id: nullableString(model.operation_id, 'model.operation_id'),
      failure: decodeFailure(model.failure),
      public_model_id: string(model.public_model_id, 'model.public_model_id'),
      default_public_model_id: string(model.default_public_model_id, 'model.default_public_model_id'),
    },
    queue: {
      capacity: integer(queue.capacity, 'queue.capacity'),
      depth: integer(queue.depth, 'queue.depth'),
      active: decodeActive(queue.active),
      waiting: array(queue.waiting, 'queue.waiting').map(decodeWaiting),
    },
    resources: decodeResources(root.resources),
    active_alert_count: integer(root.active_alert_count, 'active_alert_count'),
  };
}

function decodeRequest(value: unknown): RequestRecord {
  const keys = ['id', 'endpoint', 'public_model_id', 'stream', 'reasoning_enabled', 'tool_calls_returned', 'status', 'terminal_code', 'http_status', 'created_at', 'enqueued_at', 'started_at', 'first_token_at', 'completed_at', 'input_tokens', 'output_tokens', 'reasoning_tokens', 'queue_wait_ms', 'ttft_ms', 'duration_ms', 'throughput_tokens_per_second'];
  const item = object(value, 'RequestRecord', keys);
  return {
    id: string(item.id, 'request.id'), endpoint: string(item.endpoint, 'request.endpoint') as RequestRecord['endpoint'], public_model_id: string(item.public_model_id, 'request.public_model_id'),
    stream: boolean(item.stream, 'request.stream'), reasoning_enabled: boolean(item.reasoning_enabled, 'request.reasoning_enabled'), tool_calls_returned: boolean(item.tool_calls_returned, 'request.tool_calls_returned'),
    status: string(item.status, 'request.status') as RequestRecord['status'], terminal_code: nullableString(item.terminal_code, 'request.terminal_code'), http_status: nullableInteger(item.http_status, 'request.http_status'),
    created_at: string(item.created_at, 'request.created_at'), enqueued_at: nullableString(item.enqueued_at, 'request.enqueued_at'), started_at: nullableString(item.started_at, 'request.started_at'), first_token_at: nullableString(item.first_token_at, 'request.first_token_at'), completed_at: nullableString(item.completed_at, 'request.completed_at'),
    input_tokens: nullableInteger(item.input_tokens, 'request.input_tokens'), output_tokens: nullableInteger(item.output_tokens, 'request.output_tokens'), reasoning_tokens: nullableInteger(item.reasoning_tokens, 'request.reasoning_tokens'), queue_wait_ms: nullableInteger(item.queue_wait_ms, 'request.queue_wait_ms'), ttft_ms: nullableInteger(item.ttft_ms, 'request.ttft_ms'), duration_ms: nullableInteger(item.duration_ms, 'request.duration_ms'), throughput_tokens_per_second: nullableNumber(item.throughput_tokens_per_second, 'request.throughput_tokens_per_second'),
  };
}

export function decodeRequestPage(value: unknown): RequestPage {
  const root = object(value, 'RequestPage', ['items', 'next_cursor']);
  return { items: array(root.items, 'requests.items').map(decodeRequest), next_cursor: nullableString(root.next_cursor, 'requests.next_cursor') };
}

function decodeMetric(value: unknown): MetricBucket {
  const item = object(value, 'MetricPoint', ['bucket_start', 'request_count', 'outcomes', 'input_tokens', 'output_tokens', 'reasoning_tokens', 'throughput_tokens_per_second', 'ttft_ms_avg', 'duration_ms_avg']);
  const outcomes = object(item.outcomes, 'MetricPoint.outcomes', ['waiting', 'active', 'succeeded', 'failed', 'cancelled', 'queue_timeout', 'interrupted', 'rejected']);
  return {
    bucket_start: string(item.bucket_start, 'metric.bucket_start'), request_count: integer(item.request_count, 'metric.request_count'),
    outcomes: { waiting: integer(outcomes.waiting, 'outcomes.waiting'), active: integer(outcomes.active, 'outcomes.active'), succeeded: integer(outcomes.succeeded, 'outcomes.succeeded'), failed: integer(outcomes.failed, 'outcomes.failed'), cancelled: integer(outcomes.cancelled, 'outcomes.cancelled'), queue_timeout: integer(outcomes.queue_timeout, 'outcomes.queue_timeout'), interrupted: integer(outcomes.interrupted, 'outcomes.interrupted'), rejected: integer(outcomes.rejected, 'outcomes.rejected') },
    input_tokens: integer(item.input_tokens, 'metric.input_tokens'), output_tokens: integer(item.output_tokens, 'metric.output_tokens'), reasoning_tokens: integer(item.reasoning_tokens, 'metric.reasoning_tokens'), throughput_tokens_per_second: nullableNumber(item.throughput_tokens_per_second, 'metric.throughput_tokens_per_second'), ttft_ms_avg: nullableNumber(item.ttft_ms_avg, 'metric.ttft_ms_avg'), duration_ms_avg: nullableNumber(item.duration_ms_avg, 'metric.duration_ms_avg'),
  };
}

export function decodeMetrics(value: unknown): MetricsResponse {
  const root = object(value, 'MetricsResponse', ['from', 'to', 'granularity', 'series']);
  return { from: string(root.from, 'metrics.from'), to: string(root.to, 'metrics.to'), granularity: string(root.granularity, 'metrics.granularity') as MetricsResponse['granularity'], series: array(root.series, 'metrics.series').map(decodeMetric) };
}

function decodeAlert(value: unknown): AlertRecord {
  const item = object(value, 'Alert', ['id', 'severity', 'code', 'message', 'first_seen_at', 'last_seen_at', 'state', 'occurrences']);
  return { id: string(item.id, 'alert.id'), severity: string(item.severity, 'alert.severity') as AlertRecord['severity'], code: string(item.code, 'alert.code'), message: string(item.message, 'alert.message'), first_seen_at: string(item.first_seen_at, 'alert.first_seen_at'), last_seen_at: string(item.last_seen_at, 'alert.last_seen_at'), state: string(item.state, 'alert.state') as AlertRecord['state'], occurrences: integer(item.occurrences, 'alert.occurrences') };
}

export function decodeAlerts(value: unknown): AlertPage {
  const root = object(value, 'AlertPage', ['items', 'next_cursor']);
  return { items: array(root.items, 'alerts.items').map(decodeAlert), next_cursor: nullableString(root.next_cursor, 'alerts.next_cursor') };
}

function decodeLog(value: unknown): LogRecord {
  const item = object(value, 'SafeLog', ['id', 'occurred_at', 'level', 'event_code', 'request_id', 'operation_id', 'message']);
  return { id: integer(item.id, 'log.id'), occurred_at: string(item.occurred_at, 'log.occurred_at'), level: string(item.level, 'log.level') as LogRecord['level'], event_code: string(item.event_code, 'log.event_code'), request_id: nullableString(item.request_id, 'log.request_id'), operation_id: nullableString(item.operation_id, 'log.operation_id'), message: string(item.message, 'log.message') };
}

export function decodeLogs(value: unknown): LogPage {
  const root = object(value, 'LogPage', ['items', 'next_cursor']);
  return { items: array(root.items, 'logs.items').map(decodeLog), next_cursor: nullableString(root.next_cursor, 'logs.next_cursor') };
}

export function decodeModelAction(value: unknown): ModelActionAccepted {
  const root = object(value, 'ModelActionAccepted', ['operation_id', 'status', 'accepted_at']);
  return { operation_id: string(root.operation_id, 'operation_id'), status: string(root.status, 'status') as 'running', accepted_at: string(root.accepted_at, 'accepted_at') };
}

export function decodeModelNameSaved(value: unknown): ModelNameSaved {
  const root = object(value, 'ModelNameSaved', ['public_model_id', 'snapshot_version']);
  return { public_model_id: string(root.public_model_id, 'public_model_id'), snapshot_version: integer(root.snapshot_version, 'snapshot_version') };
}

export function decodeCancel(value: unknown): CancelResult {
  const root = object(value, 'CancelResult', ['request_id', 'status', 'cancelled_at', 'snapshot_version']);
  return { request_id: string(root.request_id, 'request_id'), status: string(root.status, 'status') as 'cancelled', cancelled_at: string(root.cancelled_at, 'cancelled_at'), snapshot_version: integer(root.snapshot_version, 'snapshot_version') };
}

export function decodeOperations(value: unknown): OperationsResponse {
  const root = object(value, 'OperationsResponse', ['retention', 'backup', 'restore_proof', 'model_operations']);
  const retention = object(root.retention, 'retention', ['cutoff_at', 'last_run_at', 'outcome', 'deleted_rows']);
  const backup = object(root.backup, 'backup', ['last_run_at', 'outcome', 'artifact_name', 'retained_count', 'next_expected_at']);
  const restore = object(root.restore_proof, 'restore_proof', ['last_run_at', 'outcome', 'restored_request_count', 'restored_token_totals']);
  const tokenTotals = restore.restored_token_totals === null ? null : object(restore.restored_token_totals, 'restored_token_totals', ['input', 'output', 'reasoning']);
  const artifactName = nullableString(backup.artifact_name, 'backup.artifact_name');
  if (artifactName?.includes('/') || artifactName?.includes('\\')) throw new ContractError('backup.artifact_name 必须是 basename。');
  return {
    retention: {
      cutoff_at: nullableString(retention.cutoff_at, 'retention.cutoff_at'),
      last_run_at: nullableString(retention.last_run_at, 'retention.last_run_at'),
      outcome: string(retention.outcome, 'retention.outcome') as OperationsResponse['retention']['outcome'],
      deleted_rows: nullableInteger(retention.deleted_rows, 'retention.deleted_rows'),
    },
    backup: {
      last_run_at: nullableString(backup.last_run_at, 'backup.last_run_at'),
      outcome: string(backup.outcome, 'backup.outcome') as OperationsResponse['backup']['outcome'],
      artifact_name: artifactName,
      retained_count: nullableInteger(backup.retained_count, 'backup.retained_count'),
      next_expected_at: nullableString(backup.next_expected_at, 'backup.next_expected_at'),
    },
    restore_proof: {
      last_run_at: nullableString(restore.last_run_at, 'restore.last_run_at'),
      outcome: string(restore.outcome, 'restore.outcome') as OperationsResponse['restore_proof']['outcome'],
      restored_request_count: nullableInteger(restore.restored_request_count, 'restore.restored_request_count'),
      restored_token_totals: tokenTotals === null ? null : {
        input: integer(tokenTotals.input, 'restored_token_totals.input'),
        output: integer(tokenTotals.output, 'restored_token_totals.output'),
        reasoning: integer(tokenTotals.reasoning, 'restored_token_totals.reasoning'),
      },
    },
    model_operations: array(root.model_operations, 'model_operations').map((value) => {
      const operation = object(value, 'ModelOperation', ['id', 'operation', 'status', 'requested_at', 'started_at', 'completed_at', 'result_code', 'observed_state']);
      return {
        id: string(operation.id, 'model_operation.id'),
        operation: string(operation.operation, 'model_operation.operation') as OperationsResponse['model_operations'][number]['operation'],
        status: string(operation.status, 'model_operation.status') as OperationsResponse['model_operations'][number]['status'],
        requested_at: string(operation.requested_at, 'model_operation.requested_at'),
        started_at: string(operation.started_at, 'model_operation.started_at'),
        completed_at: nullableString(operation.completed_at, 'model_operation.completed_at'),
        result_code: nullableString(operation.result_code, 'model_operation.result_code'),
        observed_state: nullableString(operation.observed_state, 'model_operation.observed_state') as OperationsResponse['model_operations'][number]['observed_state'],
      };
    }),
  };
}

export function decodeAdminEvent(value: unknown): AdminEvent {
  const root = object(value, 'AdminEvent', ['event_id', 'snapshot_version', 'occurred_at', 'type', 'data']);
  const dataValue = root.data;
  if (typeof dataValue !== 'object' || dataValue === null || Array.isArray(dataValue)) throw new ContractError('AdminEvent.data 必须是对象。');
  const data = dataValue as JsonObject;
  const allowed = ['changed', 'request_id', 'operation_id', 'code', 'count'];
  if (Object.keys(data).some((key) => !allowed.includes(key))) throw new ContractError('AdminEvent.data 包含未发布字段。');
  return {
    event_id: integer(root.event_id, 'event_id'), snapshot_version: integer(root.snapshot_version, 'snapshot_version'), occurred_at: string(root.occurred_at, 'occurred_at'), type: string(root.type, 'type') as AdminEvent['type'],
    data: {
      ...(data.changed === undefined ? {} : { changed: array(data.changed, 'data.changed').map((item) => string(item, 'data.changed[]')) as AdminEvent['data']['changed'] }),
      ...(data.request_id === undefined ? {} : { request_id: string(data.request_id, 'data.request_id') }),
      ...(data.operation_id === undefined ? {} : { operation_id: string(data.operation_id, 'data.operation_id') }),
      ...(data.code === undefined ? {} : { code: string(data.code, 'data.code') }),
      ...(data.count === undefined ? {} : { count: integer(data.count, 'data.count') }),
    },
  };
}
