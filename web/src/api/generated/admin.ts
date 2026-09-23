/* Generated from admin.openapi.yaml@b0434ef and admin-events.schema.json@ab17369. Do not hand-edit wire shapes. */
export type ServiceState = 'starting' | 'ready' | 'degraded' | 'stopping';
export type ModelState = 'unloaded' | 'starting' | 'ready' | 'stopping' | 'unavailable';
export type RequestStatus =
  | 'waiting'
  | 'active'
  | 'succeeded'
  | 'failed'
  | 'cancelled'
  | 'queue_timeout'
  | 'interrupted'
  | 'rejected';
export type ResourceStatus = 'available' | 'partial' | 'unavailable' | 'stale';
export type ResourceSource = 'api_container' | 'dmr_process' | 'project_storage' | 'unavailable';
export type MetalState = 'enabled' | 'disabled' | 'unknown';
export type Endpoint = 'chat.completions' | 'completions';
export type OperationOutcome = 'succeeded' | 'failed' | 'unknown';
export type ModelOperation = 'start' | 'stop' | 'reconcile_unload';
export type ModelOperationStatus = 'running' | 'succeeded' | 'failed' | 'indeterminate' | 'interrupted';
export type ObservedModelState = 'loaded' | 'unloaded' | 'unknown';
export type AlertSeverity = 'info' | 'warning' | 'critical';
export type AlertState = 'active' | 'resolved';
export type LogLevel = 'info' | 'warning' | 'error';
export type EventType =
  | 'snapshot_changed'
  | 'model_changed'
  | 'queue_changed'
  | 'metrics_updated'
  | 'alert_changed'
  | 'operation_changed'
  | 'resync_required';

export interface SafeFailure {
  code: string;
  message: string;
  retryable: boolean;
}

export interface SafeError {
  message: string;
  type: string;
  code: string;
  param: string | null;
  request_id: string;
  retryable: boolean;
  retry_after_seconds: number | null;
}

export interface ErrorEnvelope { error: SafeError }

export interface ServiceSnapshot {
  state: ServiceState;
  ready: boolean;
  authority_epoch: number;
  reason_code: string | null;
}

export interface ModelSnapshot {
  state: ModelState;
  transition_started_at: string;
  operation_id: string | null;
  failure: SafeFailure | null;
  public_model_id: string;
  default_public_model_id: string;
}

export interface ModelNameSaved { public_model_id: string; snapshot_version: number }

export interface ActiveRequest {
  id: string;
  status: 'active';
  endpoint: Endpoint;
  stream: boolean;
  reasoning_enabled: boolean;
  enqueued_at: string;
  started_at: string;
}

export interface WaitingRequest {
  id: string;
  status: 'waiting';
  endpoint: Endpoint;
  stream: boolean;
  reasoning_enabled: boolean;
  position: number;
  enqueued_at: string;
  deadline_at: string;
  can_cancel: boolean;
}

export interface QueueSnapshot {
  capacity: number;
  depth: number;
  active: ActiveRequest | null;
  waiting: WaitingRequest[];
}

export interface ResourceSnapshot {
  status: ResourceStatus;
  sampled_at: string | null;
  cpu_percent: number | null;
  cpu_source: ResourceSource;
  unified_memory_used_bytes: number | null;
  unified_memory_total_bytes: number | null;
  unified_memory_source: ResourceSource;
  disk_used_bytes: number | null;
  disk_total_bytes: number | null;
  disk_source: ResourceSource;
  metal: MetalState;
  metal_source: ResourceSource;
  reason_code: string | null;
}

export interface AdminSnapshot {
  snapshot_version: number;
  generated_at: string;
  service: ServiceSnapshot;
  model: ModelSnapshot;
  queue: QueueSnapshot;
  resources: ResourceSnapshot;
  active_alert_count: number;
}

export interface RequestRecord {
  id: string;
  endpoint: Endpoint;
  public_model_id: string;
  stream: boolean;
  reasoning_enabled: boolean;
  tool_calls_returned: boolean;
  status: RequestStatus;
  terminal_code: string | null;
  http_status: number | null;
  created_at: string;
  enqueued_at: string | null;
  started_at: string | null;
  first_token_at: string | null;
  completed_at: string | null;
  input_tokens: number | null;
  output_tokens: number | null;
  reasoning_tokens: number | null;
  queue_wait_ms: number | null;
  ttft_ms: number | null;
  duration_ms: number | null;
  throughput_tokens_per_second: number | null;
}

export interface RequestPage { items: RequestRecord[]; next_cursor: string | null }

export interface MetricOutcomes {
  waiting: number;
  active: number;
  succeeded: number;
  failed: number;
  cancelled: number;
  queue_timeout: number;
  interrupted: number;
  rejected: number;
}

export interface MetricBucket {
  bucket_start: string;
  request_count: number;
  outcomes: MetricOutcomes;
  input_tokens: number;
  output_tokens: number;
  reasoning_tokens: number;
  throughput_tokens_per_second: number | null;
  ttft_ms_avg: number | null;
  duration_ms_avg: number | null;
}

export interface MetricsResponse {
  from: string;
  to: string;
  granularity: '1m' | '1h' | '1d';
  series: MetricBucket[];
}

export interface AlertRecord {
  id: string;
  severity: AlertSeverity;
  code: string;
  message: string;
  first_seen_at: string;
  last_seen_at: string;
  state: AlertState;
  occurrences: number;
}
export interface AlertPage { items: AlertRecord[]; next_cursor: string | null }

export interface LogRecord {
  id: number;
  occurred_at: string;
  level: LogLevel;
  event_code: string;
  request_id: string | null;
  operation_id: string | null;
  message: string;
}
export interface LogPage { items: LogRecord[]; next_cursor: string | null }

export interface RetentionOperation {
  cutoff_at: string | null;
  last_run_at: string | null;
  outcome: OperationOutcome;
  deleted_rows: number | null;
}
export interface BackupOperation {
  last_run_at: string | null;
  outcome: OperationOutcome;
  artifact_name: string | null;
  retained_count: number | null;
  next_expected_at: string | null;
}
export interface RestoreProofOperation {
  last_run_at: string | null;
  outcome: OperationOutcome;
  restored_request_count: number | null;
  restored_token_totals: { input: number; output: number; reasoning: number } | null;
}
export interface ModelOperationRecord {
  id: string;
  operation: ModelOperation;
  status: ModelOperationStatus;
  requested_at: string;
  started_at: string;
  completed_at: string | null;
  result_code: string | null;
  observed_state: ObservedModelState | null;
}
export interface OperationsResponse {
  retention: RetentionOperation;
  backup: BackupOperation;
  restore_proof: RestoreProofOperation;
  model_operations: ModelOperationRecord[];
}

export interface ModelActionAccepted {
  operation_id: string;
  status: 'running';
  accepted_at: string;
}
export interface CancelResult {
  request_id: string;
  status: 'cancelled';
  cancelled_at: string;
  snapshot_version: number;
}
export interface AdminEvent {
  event_id: number;
  snapshot_version: number;
  occurred_at: string;
  type: EventType;
  data: {
    changed?: Array<'service' | 'model' | 'queue' | 'resources' | 'metrics' | 'alerts' | 'operations' | 'requests' | 'logs'>;
    request_id?: string;
    operation_id?: string;
    code?: string;
    count?: number;
  };
}
