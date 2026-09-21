-- name: LatestOperationalRuns
SELECT DISTINCT ON(kind) kind,status,started_at,completed_at,cutoff_at,affected_rows,
       request_count,input_tokens,output_tokens,reasoning_tokens,artifact_name,error_code
FROM api_operational_runs
ORDER BY kind,started_at DESC;

-- name: LatestModelOperations
SELECT id,operation,status,requested_at,started_at,completed_at,result_code,observed_state
FROM api_model_operations
ORDER BY requested_at DESC
LIMIT $1;
