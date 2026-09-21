BEGIN;

CREATE TABLE inference_metrics_hourly (
 bucket_start timestamptz NOT NULL, endpoint text NOT NULL CHECK(endpoint IN ('chat.completions','completions')),
 outcome text NOT NULL CHECK(outcome IN ('waiting','active','succeeded','failed','cancelled','queue_timeout','interrupted','rejected')),
 stream boolean NOT NULL, reasoning_enabled boolean NOT NULL,
 request_count bigint NOT NULL CHECK(request_count>=0), input_tokens bigint NOT NULL CHECK(input_tokens>=0),
 output_tokens bigint NOT NULL CHECK(output_tokens>=0),reasoning_tokens bigint NOT NULL CHECK(reasoning_tokens>=0),
 generation_ms bigint NOT NULL CHECK(generation_ms>=0),ttft_ms_sum bigint NOT NULL CHECK(ttft_ms_sum>=0),
 ttft_sample_count bigint NOT NULL CHECK(ttft_sample_count>=0),duration_ms_sum bigint NOT NULL CHECK(duration_ms_sum>=0),
 duration_sample_count bigint NOT NULL CHECK(duration_sample_count>=0),
 PRIMARY KEY(bucket_start,endpoint,outcome,stream,reasoning_enabled),
 CHECK(bucket_start=date_trunc('hour',bucket_start AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')
);
CREATE INDEX inference_metrics_hourly_time_idx ON inference_metrics_hourly(bucket_start DESC);

CREATE TABLE operational_runs (
 id uuid PRIMARY KEY, kind text NOT NULL CHECK(kind IN ('aggregation','retention','backup','restore_proof')),
 status text NOT NULL CHECK(status IN ('running','succeeded','failed')), started_at timestamptz NOT NULL, completed_at timestamptz,
 cutoff_at timestamptz, affected_rows bigint CHECK(affected_rows>=0), artifact_name text,
 request_count bigint CHECK(request_count>=0), input_tokens bigint CHECK(input_tokens>=0),output_tokens bigint CHECK(output_tokens>=0),reasoning_tokens bigint CHECK(reasoning_tokens>=0),error_code text,
 CHECK((status='running' AND completed_at IS NULL) OR (status<>'running' AND completed_at IS NOT NULL)),
 CHECK(completed_at IS NULL OR started_at<=completed_at),
 CHECK(artifact_name IS NULL OR (artifact_name<>'' AND artifact_name !~ '[/\\]'))
);
CREATE INDEX operational_runs_kind_time_idx ON operational_runs(kind,started_at DESC);

CREATE FUNCTION aggregate_closed_hour(p_bucket timestamptz,p_run_id uuid)
RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE n bigint;
BEGIN
 IF NOT pg_try_advisory_xact_lock(741291552) THEN RAISE EXCEPTION USING ERRCODE='55P03',MESSAGE='scheduler_lock_busy'; END IF;
 IF p_bucket<>date_trunc('hour',p_bucket AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' OR p_bucket>=date_trunc('hour',transaction_timestamp()) THEN
   RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='invalid_closed_bucket';
 END IF;
 INSERT INTO public.operational_runs(id,kind,status,started_at,cutoff_at) VALUES(p_run_id,'aggregation','running',transaction_timestamp(),p_bucket);
 DELETE FROM public.inference_metrics_hourly WHERE bucket_start=p_bucket;
 INSERT INTO public.inference_metrics_hourly(bucket_start,endpoint,outcome,stream,reasoning_enabled,request_count,input_tokens,output_tokens,reasoning_tokens,generation_ms,ttft_ms_sum,ttft_sample_count,duration_ms_sum,duration_sample_count)
 SELECT p_bucket,endpoint,status,stream,reasoning_enabled,count(*),coalesce(sum(input_tokens),0),coalesce(sum(output_tokens),0),coalesce(sum(reasoning_tokens),0),coalesce(sum(generation_ms),0),coalesce(sum(ttft_ms),0),count(ttft_ms),coalesce(sum(duration_ms),0),count(duration_ms)
 FROM public.inference_requests WHERE created_at>=p_bucket AND created_at<p_bucket+interval '1 hour'
 GROUP BY endpoint,status,stream,reasoning_enabled;
 GET DIAGNOSTICS n=ROW_COUNT;
 UPDATE public.operational_runs SET status='succeeded',completed_at=transaction_timestamp(),affected_rows=n WHERE id=p_run_id;
 RETURN n;
EXCEPTION WHEN OTHERS THEN
 UPDATE public.operational_runs SET status='failed',completed_at=transaction_timestamp(),error_code='aggregation_failed' WHERE id=p_run_id;
 RAISE;
END $$;

CREATE FUNCTION purge_expired_metadata(p_run_id uuid)
RETURNS TABLE(cutoff_at timestamptz,deleted_rows bigint)
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE c timestamptz:=transaction_timestamp()-interval '30 days'; n bigint:=0; x bigint; batch bigint; i integer;
BEGIN
 IF NOT pg_try_advisory_xact_lock(741291552) THEN RAISE EXCEPTION USING ERRCODE='55P03',MESSAGE='scheduler_lock_busy'; END IF;
 INSERT INTO public.operational_runs(id,kind,status,started_at,cutoff_at) VALUES(p_run_id,'retention','running',transaction_timestamp(),c);
 FOR i IN 1..100 LOOP
  batch:=0;
  WITH doomed AS (SELECT id FROM public.inference_requests WHERE created_at<c ORDER BY created_at,id LIMIT 1000 FOR UPDATE SKIP LOCKED)
  DELETE FROM public.inference_requests r USING doomed d WHERE r.id=d.id;
  GET DIAGNOSTICS x=ROW_COUNT; n:=n+x; batch:=batch+x;
  WITH doomed AS (SELECT id FROM public.admin_events WHERE occurred_at<c ORDER BY id LIMIT 1000)
  DELETE FROM public.admin_events e USING doomed d WHERE e.id=d.id;
  GET DIAGNOSTICS x=ROW_COUNT; n:=n+x; batch:=batch+x;
  WITH doomed AS (SELECT id FROM public.safe_log_events WHERE occurred_at<c ORDER BY id LIMIT 1000)
  DELETE FROM public.safe_log_events e USING doomed d WHERE e.id=d.id;
  GET DIAGNOSTICS x=ROW_COUNT; n:=n+x; batch:=batch+x;
  WITH doomed AS (SELECT id FROM public.alerts WHERE state='resolved' AND resolved_at<c ORDER BY resolved_at,id LIMIT 1000)
  DELETE FROM public.alerts e USING doomed d WHERE e.id=d.id;
  GET DIAGNOSTICS x=ROW_COUNT; n:=n+x; batch:=batch+x;
  WITH doomed AS (SELECT id FROM public.model_operations WHERE completed_at<c ORDER BY completed_at,id LIMIT 1000)
  DELETE FROM public.model_operations e USING doomed d WHERE e.id=d.id;
  GET DIAGNOSTICS x=ROW_COUNT; n:=n+x; batch:=batch+x;
  DELETE FROM public.inference_metrics_hourly WHERE bucket_start<c;
  GET DIAGNOSTICS x=ROW_COUNT; n:=n+x; batch:=batch+x;
  WITH doomed AS (SELECT id FROM public.operational_runs WHERE completed_at<c AND id<>p_run_id ORDER BY completed_at,id LIMIT 1000)
  DELETE FROM public.operational_runs e USING doomed d WHERE e.id=d.id;
  GET DIAGNOSTICS x=ROW_COUNT; n:=n+x; batch:=batch+x;
  EXIT WHEN batch=0;
 END LOOP;
 UPDATE public.operational_runs SET status='succeeded',completed_at=transaction_timestamp(),affected_rows=n WHERE id=p_run_id;
 cutoff_at:=c;deleted_rows:=n;RETURN NEXT;
END $$;

CREATE FUNCTION record_backup_run(p_id uuid,p_status text,p_started timestamptz,p_completed timestamptz,p_artifact text,p_retained bigint,p_error text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 IF p_status NOT IN ('succeeded','failed') OR p_artifact ~ '[/\\]' THEN RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='invalid_backup_evidence'; END IF;
 INSERT INTO public.operational_runs(id,kind,status,started_at,completed_at,artifact_name,affected_rows,error_code)
 VALUES(p_id,'backup',p_status,p_started,p_completed,p_artifact,p_retained,p_error);
END $$;

CREATE FUNCTION record_restore_proof(p_id uuid,p_status text,p_started timestamptz,p_completed timestamptz,p_count bigint,p_input bigint,p_output bigint,p_reasoning bigint,p_error text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 IF p_status NOT IN ('succeeded','failed') THEN RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='invalid_restore_evidence'; END IF;
 INSERT INTO public.operational_runs(id,kind,status,started_at,completed_at,request_count,input_tokens,output_tokens,reasoning_tokens,error_code)
 VALUES(p_id,'restore_proof',p_status,p_started,p_completed,p_count,p_input,p_output,p_reasoning,p_error);
END $$;

CREATE FUNCTION next_closed_aggregation_bucket()
RETURNS timestamptz LANGUAGE sql STABLE SECURITY DEFINER SET search_path=pg_catalog,public AS $$
 SELECT CASE
   WHEN (SELECT max(cutoff_at) FROM public.operational_runs WHERE kind='aggregation' AND status='succeeded') IS NOT NULL
     THEN (SELECT max(cutoff_at)+interval '1 hour' FROM public.operational_runs WHERE kind='aggregation' AND status='succeeded')
   ELSE (SELECT date_trunc('hour',min(created_at)) FROM public.inference_requests)
 END
$$;

CREATE FUNCTION assert_restore_schema()
RETURNS TABLE(request_count bigint,input_tokens bigint,output_tokens bigint,reasoning_tokens bigint)
LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
 SELECT count(*),coalesce(sum(input_tokens),0),coalesce(sum(output_tokens),0),coalesce(sum(reasoning_tokens),0) FROM public.inference_requests
$$;

REVOKE ALL ON inference_metrics_hourly,operational_runs FROM PUBLIC;
REVOKE ALL ON FUNCTION aggregate_closed_hour(timestamptz,uuid),next_closed_aggregation_bucket(),purge_expired_metadata(uuid),record_backup_run(uuid,text,timestamptz,timestamptz,text,bigint,text),record_restore_proof(uuid,text,timestamptz,timestamptz,bigint,bigint,bigint,bigint,text),assert_restore_schema() FROM PUBLIC;
INSERT INTO schema_migrations(version) VALUES(4);
COMMIT;
