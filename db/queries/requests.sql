-- name: RequestHistory
SELECT id, endpoint, public_model_id, stream, reasoning_enabled, tool_calls_returned,
       status, terminal_code, http_status, created_at, enqueued_at, started_at,
       first_token_at, completed_at, input_tokens, output_tokens, reasoning_tokens,
       queue_wait_ms, ttft_ms, duration_ms, generation_ms
FROM api_request_history
WHERE ($1::timestamptz IS NULL OR (created_at,id) < ($1,$2))
ORDER BY created_at DESC,id DESC
LIMIT $3;

-- name: LiveQueue
SELECT id, endpoint, stream, reasoning_enabled, status, enqueued_at, started_at, arrival_seq
FROM api_request_history
WHERE status IN ('active','waiting')
ORDER BY CASE status WHEN 'active' THEN 0 ELSE 1 END, arrival_seq;
