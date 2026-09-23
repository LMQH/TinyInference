BEGIN;

DO $$
DECLARE bad bigint;
BEGIN
 SELECT count(*) INTO bad FROM information_schema.columns
 WHERE table_schema='public' AND (
   column_name ~ '(prompt|response|reasoning_content|tool_arguments|request_body|response_body|content_body)'
   OR data_type='jsonb' AND table_name<>'admin_events');
 IF bad<>0 THEN RAISE EXCEPTION 'prohibited content-capable columns exist'; END IF;
 IF (SELECT count(*) FROM pg_roles WHERE rolname IN ('mini_owner','mini_migrator','mini_api','mini_retention','mini_backup','mini_restore','mini_controller_epoch'))<>7 THEN
   RAISE EXCEPTION 'least privilege role set incomplete';
 END IF;
 IF EXISTS(SELECT 1 FROM pg_roles WHERE rolname LIKE 'mini_%' AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls)) THEN
   RAISE EXCEPTION 'application role has elevated attributes';
 END IF;
 IF has_table_privilege('mini_api','public.inference_requests','INSERT') OR has_table_privilege('mini_api','public.inference_requests','UPDATE') OR has_table_privilege('mini_api','public.inference_requests','DELETE') THEN
   RAISE EXCEPTION 'mini_api can mutate base requests directly';
 END IF;
 IF has_table_privilege('mini_controller_epoch','public.inference_requests','SELECT') OR NOT has_table_privilege('mini_controller_epoch','public.controller_authority_epoch','SELECT') THEN
   RAISE EXCEPTION 'controller authority role is over/under privileged';
 END IF;
 IF (SELECT array_agg(version ORDER BY version) FROM schema_migrations)<>ARRAY[1::bigint,2,3,4,5,6,7] THEN
   RAISE EXCEPTION 'migration sequence is not exact';
 END IF;
 IF has_function_privilege('mini_restore','public.assert_restore_schema()','EXECUTE') OR NOT has_function_privilege('mini_restore','public.record_restore_proof(uuid,text,timestamptz,timestamptz,bigint,bigint,bigint,bigint,text)','EXECUTE') THEN
   RAISE EXCEPTION 'restore role procedure privilege is over/under privileged';
 END IF;
END $$;

DO $$
DECLARE boundary uuid:=gen_random_uuid(); expired uuid:=gen_random_uuid(); run uuid:=gen_random_uuid(); cutoff timestamptz:=transaction_timestamp()-interval '30 days';
BEGIN
 INSERT INTO inference_requests(id,endpoint,public_model_id,stream,reasoning_enabled,tool_calls_returned,status,terminal_code,http_status,fence_epoch,arrival_seq,created_at,enqueued_at,completed_at)
 VALUES(boundary,'completions','openbmb/MiniCPM5-2B-Q4_K_M',false,true,false,'cancelled','test',409,1,1,cutoff,cutoff,cutoff),
       (expired,'completions','openbmb/MiniCPM5-2B-Q4_K_M',false,true,false,'cancelled','test',409,1,2,cutoff-interval '1 microsecond',cutoff-interval '1 microsecond',cutoff-interval '1 microsecond');
 PERFORM * FROM purge_expired_metadata(run);
 IF NOT EXISTS(SELECT 1 FROM inference_requests WHERE id=boundary) OR EXISTS(SELECT 1 FROM inference_requests WHERE id=expired) THEN
   RAISE EXCEPTION 'retention strict boundary violated';
 END IF;
END $$;

ROLLBACK;
