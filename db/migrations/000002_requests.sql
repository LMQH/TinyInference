BEGIN;

CREATE TABLE inference_requests (
    id uuid PRIMARY KEY,
    endpoint text NOT NULL CHECK (endpoint IN ('chat.completions','completions')),
    public_model_id text NOT NULL,
    stream boolean NOT NULL,
    reasoning_enabled boolean NOT NULL,
    tool_calls_returned boolean NOT NULL DEFAULT false,
    status text NOT NULL CHECK (status IN ('waiting','active','succeeded','failed','cancelled','queue_timeout','interrupted','rejected')),
    terminal_code text,
    http_status smallint CHECK (http_status BETWEEN 100 AND 599),
    fence_epoch bigint NOT NULL CHECK (fence_epoch > 0),
    arrival_seq bigint CHECK (arrival_seq > 0),
    created_at timestamptz NOT NULL,
    enqueued_at timestamptz,
    started_at timestamptz,
    first_token_at timestamptz,
    completed_at timestamptz,
    input_tokens bigint CHECK (input_tokens >= 0),
    output_tokens bigint CHECK (output_tokens >= 0),
    reasoning_tokens bigint CHECK (reasoning_tokens >= 0),
    queue_wait_ms bigint CHECK (queue_wait_ms >= 0),
    ttft_ms bigint CHECK (ttft_ms >= 0),
    duration_ms bigint CHECK (duration_ms >= 0),
    generation_ms bigint CHECK (generation_ms >= 0),
    created_day date GENERATED ALWAYS AS ((created_at AT TIME ZONE 'UTC')::date) STORED,
    CHECK ((status='rejected' AND arrival_seq IS NULL AND enqueued_at IS NULL AND started_at IS NULL AND first_token_at IS NULL AND completed_at IS NOT NULL)
        OR (status='waiting' AND arrival_seq IS NOT NULL AND enqueued_at IS NOT NULL AND started_at IS NULL AND first_token_at IS NULL AND completed_at IS NULL)
        OR (status='active' AND arrival_seq IS NOT NULL AND enqueued_at IS NOT NULL AND started_at IS NOT NULL AND completed_at IS NULL)
        OR (status='queue_timeout' AND arrival_seq IS NOT NULL AND enqueued_at IS NOT NULL AND started_at IS NULL AND first_token_at IS NULL AND completed_at IS NOT NULL)
        OR (status IN ('succeeded','failed','cancelled','interrupted') AND arrival_seq IS NOT NULL AND enqueued_at IS NOT NULL AND completed_at IS NOT NULL)),
    CHECK (enqueued_at IS NULL OR created_at <= enqueued_at),
    CHECK (started_at IS NULL OR (enqueued_at IS NOT NULL AND enqueued_at <= started_at)),
    CHECK (first_token_at IS NULL OR (started_at IS NOT NULL AND started_at <= first_token_at)),
    CHECK (completed_at IS NULL OR created_at <= completed_at),
    CHECK (completed_at IS NULL OR started_at IS NULL OR started_at <= completed_at),
    CHECK (completed_at IS NULL OR first_token_at IS NULL OR first_token_at <= completed_at)
);
CREATE INDEX inference_requests_queue_idx ON inference_requests(status,enqueued_at,arrival_seq);
CREATE INDEX inference_requests_history_idx ON inference_requests(created_at DESC,id DESC);
CREATE INDEX inference_requests_completed_idx ON inference_requests(completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX inference_requests_day_idx ON inference_requests(created_day,endpoint,status);

CREATE TABLE request_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id uuid NOT NULL REFERENCES inference_requests(id) ON DELETE CASCADE,
    from_status text CHECK (from_status IS NULL OR from_status IN ('waiting','active','succeeded','failed','cancelled','queue_timeout','interrupted','rejected')),
    to_status text NOT NULL CHECK (to_status IN ('waiting','active','succeeded','failed','cancelled','queue_timeout','interrupted','rejected')),
    reason_code text,
    fence_epoch bigint NOT NULL CHECK (fence_epoch > 0),
    occurred_at timestamptz NOT NULL
);
CREATE INDEX request_events_request_idx ON request_events(request_id,id);
CREATE INDEX request_events_time_idx ON request_events(occurred_at);

CREATE FUNCTION admit_request(
 p_holder_id uuid,p_epoch bigint,p_id uuid,p_endpoint text,p_public_model_id text,p_stream boolean,
 p_reasoning boolean,p_status text,p_arrival_seq bigint,p_created_at timestamptz,p_enqueued_at timestamptz,p_started_at timestamptz)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 PERFORM public.assert_backend_authority(p_holder_id,p_epoch);
 IF p_status NOT IN ('waiting','active') THEN RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='invalid_initial_status'; END IF;
 INSERT INTO public.inference_requests(id,endpoint,public_model_id,stream,reasoning_enabled,status,fence_epoch,arrival_seq,created_at,enqueued_at,started_at)
 VALUES(p_id,p_endpoint,p_public_model_id,p_stream,p_reasoning,p_status,p_epoch,p_arrival_seq,p_created_at,p_enqueued_at,p_started_at);
 INSERT INTO public.request_events(request_id,from_status,to_status,fence_epoch,occurred_at)
 VALUES(p_id,NULL,p_status,p_epoch,transaction_timestamp());
END $$;

CREATE FUNCTION transition_request(
 p_holder_id uuid,p_epoch bigint,p_id uuid,p_from text,p_to text,p_reason_code text,p_http_status smallint,
 p_started_at timestamptz,p_first_token_at timestamptz,p_completed_at timestamptz,
 p_input_tokens bigint,p_output_tokens bigint,p_reasoning_tokens bigint,p_queue_wait_ms bigint,p_ttft_ms bigint,p_duration_ms bigint,p_generation_ms bigint,p_tool_calls boolean)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 PERFORM public.assert_backend_authority(p_holder_id,p_epoch);
 IF NOT ((p_from='waiting' AND p_to IN ('active','failed','cancelled','queue_timeout','interrupted')) OR
         (p_from='active' AND p_to IN ('succeeded','failed','cancelled','interrupted'))) THEN
   RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='illegal_request_transition';
 END IF;
 UPDATE public.inference_requests SET status=p_to,terminal_code=CASE WHEN p_to IN ('active') THEN NULL ELSE p_reason_code END,
   http_status=p_http_status,started_at=COALESCE(p_started_at,started_at),first_token_at=COALESCE(p_first_token_at,first_token_at),
   completed_at=p_completed_at,input_tokens=p_input_tokens,output_tokens=p_output_tokens,reasoning_tokens=p_reasoning_tokens,
   queue_wait_ms=p_queue_wait_ms,ttft_ms=p_ttft_ms,duration_ms=p_duration_ms,generation_ms=p_generation_ms,
   tool_calls_returned=p_tool_calls
 WHERE id=p_id AND status=p_from AND fence_epoch=p_epoch;
 IF NOT FOUND THEN RAISE EXCEPTION USING ERRCODE='P0001',MESSAGE='authority_fence_lost'; END IF;
 INSERT INTO public.request_events(request_id,from_status,to_status,reason_code,fence_epoch,occurred_at)
 VALUES(p_id,p_from,p_to,p_reason_code,p_epoch,transaction_timestamp());
END $$;

REVOKE ALL ON inference_requests,request_events FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM PUBLIC;
REVOKE ALL ON FUNCTION admit_request(uuid,bigint,uuid,text,text,boolean,boolean,text,bigint,timestamptz,timestamptz,timestamptz), transition_request(uuid,bigint,uuid,text,text,text,smallint,timestamptz,timestamptz,timestamptz,bigint,bigint,bigint,bigint,bigint,bigint,bigint,boolean) FROM PUBLIC;

INSERT INTO schema_migrations(version) VALUES(2);
COMMIT;
