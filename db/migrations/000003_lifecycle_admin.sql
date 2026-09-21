BEGIN;

CREATE TABLE model_operations (
 id uuid PRIMARY KEY,
 operation text NOT NULL CHECK(operation IN ('start','stop','reconcile_unload')),
 status text NOT NULL CHECK(status IN ('running','succeeded','failed','indeterminate','interrupted')),
 requested_at timestamptz NOT NULL, started_at timestamptz NOT NULL, completed_at timestamptz,
 fence_epoch bigint NOT NULL CHECK(fence_epoch>0), result_code text,
 observed_state text CHECK(observed_state IN ('loaded','unloaded','unknown')),
 CHECK(requested_at<=started_at), CHECK(completed_at IS NULL OR started_at<=completed_at),
 CHECK((status='running' AND completed_at IS NULL AND result_code IS NULL) OR (status<>'running' AND completed_at IS NOT NULL))
);
CREATE INDEX model_operations_time_idx ON model_operations(requested_at DESC);
CREATE UNIQUE INDEX model_operations_one_running_idx ON model_operations((status)) WHERE status='running';

CREATE TABLE model_operation_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 operation_id uuid NOT NULL REFERENCES model_operations(id) ON DELETE CASCADE,
 from_status text CHECK(from_status IS NULL OR from_status IN ('running','succeeded','failed','indeterminate','interrupted')),
 to_status text NOT NULL CHECK(to_status IN ('running','succeeded','failed','indeterminate','interrupted')),
 reason_code text, observed_state text CHECK(observed_state IS NULL OR observed_state IN ('loaded','unloaded','unknown')),
 occurred_at timestamptz NOT NULL, fence_epoch bigint NOT NULL CHECK(fence_epoch>0)
);
CREATE INDEX model_operation_events_operation_idx ON model_operation_events(operation_id,id);

CREATE TABLE admin_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 snapshot_version bigint NOT NULL UNIQUE,
 event_type text NOT NULL CHECK(event_type IN ('snapshot_changed','model_changed','queue_changed','metrics_updated','alert_changed','operation_changed','resync_required')),
 data jsonb NOT NULL DEFAULT '{}'::jsonb,
 occurred_at timestamptz NOT NULL,
 CHECK(jsonb_typeof(data)='object'),
 CHECK(data - ARRAY['changed','request_id','operation_id','code','count'] = '{}'::jsonb)
);
CREATE INDEX admin_events_time_idx ON admin_events(occurred_at);

CREATE TABLE alerts (
 id uuid PRIMARY KEY, severity text NOT NULL CHECK(severity IN ('info','warning','critical')),
 code text NOT NULL, message text NOT NULL, state text NOT NULL CHECK(state IN ('active','resolved')),
 occurrences bigint NOT NULL CHECK(occurrences>0), first_seen_at timestamptz NOT NULL,last_seen_at timestamptz NOT NULL,resolved_at timestamptz,
 CHECK(first_seen_at<=last_seen_at), CHECK((state='active' AND resolved_at IS NULL) OR (state='resolved' AND resolved_at IS NOT NULL))
);
CREATE UNIQUE INDEX alerts_active_code_idx ON alerts(code) WHERE state='active';
CREATE INDEX alerts_history_idx ON alerts(last_seen_at DESC,id DESC);

CREATE TABLE safe_log_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, occurred_at timestamptz NOT NULL,
 level text NOT NULL CHECK(level IN ('info','warning','error')), event_code text NOT NULL,
 request_id uuid REFERENCES inference_requests(id) ON DELETE SET NULL,
 operation_id uuid REFERENCES model_operations(id) ON DELETE SET NULL,
 message text NOT NULL
);
CREATE INDEX safe_log_events_history_idx ON safe_log_events(occurred_at DESC,id DESC);
CREATE INDEX safe_log_events_request_idx ON safe_log_events(request_id);
CREATE INDEX safe_log_events_operation_idx ON safe_log_events(operation_id);

CREATE FUNCTION publish_admin_event(p_holder_id uuid,p_epoch bigint,p_type text,p_data jsonb)
RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE v bigint;
BEGIN
 PERFORM public.assert_backend_authority(p_holder_id,p_epoch);
 IF p_type NOT IN ('snapshot_changed','model_changed','queue_changed','metrics_updated','alert_changed','operation_changed','resync_required') OR
    jsonb_typeof(p_data)<>'object' OR p_data-ARRAY['changed','request_id','operation_id','code','count']<>'{}'::jsonb THEN
   RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='invalid_admin_event';
 END IF;
 v:=nextval('public.admin_snapshot_version_seq');
 UPDATE public.admin_snapshot_state SET snapshot_version=v,updated_at=transaction_timestamp() WHERE singleton;
 INSERT INTO public.admin_events(snapshot_version,event_type,data,occurred_at) VALUES(v,p_type,p_data,transaction_timestamp());
 RETURN v;
END $$;

CREATE FUNCTION begin_model_operation(p_holder_id uuid,p_epoch bigint,p_id uuid,p_operation text,p_at timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 PERFORM public.assert_backend_authority(p_holder_id,p_epoch);
 INSERT INTO public.model_operations(id,operation,status,requested_at,started_at,fence_epoch)
 VALUES(p_id,p_operation,'running',p_at,p_at,p_epoch);
 INSERT INTO public.model_operation_events(operation_id,from_status,to_status,occurred_at,fence_epoch) VALUES(p_id,NULL,'running',transaction_timestamp(),p_epoch);
 PERFORM public.publish_admin_event(p_holder_id,p_epoch,'operation_changed',jsonb_build_object('changed',jsonb_build_array('operations'),'operation_id',p_id));
END $$;

CREATE FUNCTION finish_model_operation(p_holder_id uuid,p_epoch bigint,p_id uuid,p_status text,p_code text,p_observed text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 PERFORM public.assert_backend_authority(p_holder_id,p_epoch);
 IF p_status NOT IN ('succeeded','failed','indeterminate','interrupted') THEN RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='illegal_operation_transition'; END IF;
 UPDATE public.model_operations SET status=p_status,completed_at=transaction_timestamp(),result_code=p_code,observed_state=p_observed
 WHERE id=p_id AND status='running' AND fence_epoch=p_epoch;
 IF NOT FOUND THEN RAISE EXCEPTION USING ERRCODE='P0001',MESSAGE='authority_fence_lost'; END IF;
 INSERT INTO public.model_operation_events(operation_id,from_status,to_status,reason_code,observed_state,occurred_at,fence_epoch)
 VALUES(p_id,'running',p_status,p_code,p_observed,transaction_timestamp(),p_epoch);
 PERFORM public.publish_admin_event(p_holder_id,p_epoch,'operation_changed',jsonb_build_object('changed',jsonb_build_array('operations'),'operation_id',p_id));
END $$;

CREATE FUNCTION reconcile_prior_epoch(p_holder_id uuid,p_epoch bigint)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE r record;
BEGIN
 PERFORM public.assert_backend_authority(p_holder_id,p_epoch);
 IF EXISTS(SELECT 1 FROM public.inference_requests WHERE status IN ('waiting','active') AND fence_epoch>=p_epoch)
 OR EXISTS(SELECT 1 FROM public.model_operations WHERE status='running' AND fence_epoch>=p_epoch) THEN
   RAISE EXCEPTION USING ERRCODE='P0001',MESSAGE='reconciliation_epoch_conflict';
 END IF;
 FOR r IN SELECT id,status,fence_epoch FROM public.inference_requests WHERE status IN ('waiting','active') AND fence_epoch<p_epoch FOR UPDATE LOOP
   UPDATE public.inference_requests SET status='interrupted',terminal_code='leader_reconciliation',http_status=503,completed_at=transaction_timestamp() WHERE id=r.id;
   INSERT INTO public.request_events(request_id,from_status,to_status,reason_code,fence_epoch,occurred_at) VALUES(r.id,r.status,'interrupted','leader_reconciliation',p_epoch,transaction_timestamp());
 END LOOP;
 FOR r IN SELECT id,fence_epoch FROM public.model_operations WHERE status='running' AND fence_epoch<p_epoch FOR UPDATE LOOP
   UPDATE public.model_operations SET status='interrupted',result_code='leader_reconciliation',completed_at=transaction_timestamp(),observed_state='unknown' WHERE id=r.id;
   INSERT INTO public.model_operation_events(operation_id,from_status,to_status,reason_code,observed_state,occurred_at,fence_epoch) VALUES(r.id,'running','interrupted','leader_reconciliation','unknown',transaction_timestamp(),p_epoch);
 END LOOP;
 PERFORM public.publish_admin_event(p_holder_id,p_epoch,'snapshot_changed',jsonb_build_object('changed',jsonb_build_array('queue','model','operations')));
END $$;

CREATE FUNCTION record_safe_log(p_holder_id uuid,p_epoch bigint,p_level text,p_event_code text,p_request_id uuid,p_operation_id uuid)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE msg text;
BEGIN
 PERFORM public.assert_backend_authority(p_holder_id,p_epoch);
 IF p_level NOT IN ('info','warning','error') THEN RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='invalid_log_level'; END IF;
 msg:=CASE p_event_code
   WHEN 'authority_acquired' THEN 'Inference authority acquired.'
   WHEN 'authority_lost' THEN 'Inference authority lost.'
   WHEN 'model_unavailable' THEN 'The model is unavailable.'
   WHEN 'aggregation_lag' THEN 'Metrics aggregation is behind schedule.'
   WHEN 'retention_failed' THEN 'Metadata retention failed.'
   WHEN 'backup_stale' THEN 'No current valid database backup is available.'
   WHEN 'controller_failed' THEN 'The lifecycle controller operation failed.'
   ELSE NULL END;
 IF msg IS NULL THEN RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='invalid_log_code'; END IF;
 INSERT INTO public.safe_log_events(occurred_at,level,event_code,request_id,operation_id,message)
 VALUES(transaction_timestamp(),p_level,p_event_code,p_request_id,p_operation_id,msg);
END $$;

CREATE FUNCTION upsert_alert(p_holder_id uuid,p_epoch bigint,p_id uuid,p_severity text,p_code text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE msg text;
BEGIN
 PERFORM public.assert_backend_authority(p_holder_id,p_epoch);
 msg:=CASE p_code
   WHEN 'authority_lost' THEN 'Inference authority is unavailable.'
   WHEN 'model_unavailable' THEN 'The model is unavailable.'
   WHEN 'aggregation_lag' THEN 'Metrics aggregation is behind schedule.'
   WHEN 'retention_failed' THEN 'Metadata retention failed.'
   WHEN 'backup_stale' THEN 'No current valid database backup is available.'
   WHEN 'controller_failed' THEN 'The lifecycle controller is unavailable.'
   ELSE NULL END;
 IF msg IS NULL OR p_severity NOT IN ('info','warning','critical') THEN RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='invalid_alert'; END IF;
 INSERT INTO public.alerts(id,severity,code,message,state,occurrences,first_seen_at,last_seen_at)
 VALUES(p_id,p_severity,p_code,msg,'active',1,transaction_timestamp(),transaction_timestamp())
 ON CONFLICT(code) WHERE state='active' DO UPDATE SET last_seen_at=transaction_timestamp(),occurrences=public.alerts.occurrences+1;
 PERFORM public.publish_admin_event(p_holder_id,p_epoch,'alert_changed',jsonb_build_object('changed',jsonb_build_array('alerts'),'code',p_code));
END $$;

CREATE FUNCTION resolve_alert(p_holder_id uuid,p_epoch bigint,p_code text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 PERFORM public.assert_backend_authority(p_holder_id,p_epoch);
 UPDATE public.alerts SET state='resolved',resolved_at=transaction_timestamp(),last_seen_at=transaction_timestamp() WHERE code=p_code AND state='active';
 IF FOUND THEN PERFORM public.publish_admin_event(p_holder_id,p_epoch,'alert_changed',jsonb_build_object('changed',jsonb_build_array('alerts'),'code',p_code)); END IF;
END $$;

REVOKE ALL ON model_operations,model_operation_events,admin_events,alerts,safe_log_events FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM PUBLIC;
REVOKE ALL ON FUNCTION publish_admin_event(uuid,bigint,text,jsonb),begin_model_operation(uuid,bigint,uuid,text,timestamptz),finish_model_operation(uuid,bigint,uuid,text,text,text),reconcile_prior_epoch(uuid,bigint),record_safe_log(uuid,bigint,text,text,uuid,uuid),upsert_alert(uuid,bigint,uuid,text,text),resolve_alert(uuid,bigint,text) FROM PUBLIC;
INSERT INTO schema_migrations(version) VALUES(3);
COMMIT;
