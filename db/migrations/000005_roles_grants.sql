BEGIN;

DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='mini_owner') THEN CREATE ROLE mini_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='mini_migrator') THEN CREATE ROLE mini_migrator LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='mini_api') THEN CREATE ROLE mini_api LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='mini_retention') THEN CREATE ROLE mini_retention LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='mini_backup') THEN CREATE ROLE mini_backup LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='mini_restore') THEN CREATE ROLE mini_restore LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='mini_controller_epoch') THEN CREATE ROLE mini_controller_epoch LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS; END IF;
END $$;
GRANT mini_owner TO mini_migrator;

CREATE VIEW api_schema_version WITH (security_barrier=true) AS SELECT version,applied_at FROM schema_migrations;
CREATE VIEW api_authority_state WITH (security_barrier=true) AS SELECT epoch,holder_id,acquired_at,heartbeat_at,released_at FROM backend_authority;
CREATE VIEW api_snapshot_state WITH (security_barrier=true) AS SELECT snapshot_version,updated_at FROM admin_snapshot_state WHERE singleton;
CREATE VIEW api_request_history WITH (security_barrier=true) AS SELECT * FROM inference_requests;
CREATE VIEW api_request_events WITH (security_barrier=true) AS SELECT * FROM request_events;
CREATE VIEW api_model_operations WITH (security_barrier=true) AS SELECT * FROM model_operations;
CREATE VIEW api_admin_events WITH (security_barrier=true) AS SELECT * FROM admin_events;
CREATE VIEW api_alerts WITH (security_barrier=true) AS SELECT * FROM alerts;
CREATE VIEW api_safe_logs WITH (security_barrier=true) AS SELECT * FROM safe_log_events;
CREATE VIEW api_metrics_hourly WITH (security_barrier=true) AS SELECT * FROM inference_metrics_hourly;
CREATE VIEW api_operational_runs WITH (security_barrier=true) AS SELECT * FROM operational_runs;

REVOKE ALL ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM PUBLIC;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE mini_owner IN SCHEMA public REVOKE ALL ON TABLES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE mini_owner IN SCHEMA public REVOKE ALL ON SEQUENCES FROM PUBLIC;
ALTER DEFAULT PRIVILEGES FOR ROLE mini_owner IN SCHEMA public REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;

DO $$ BEGIN EXECUTE format('GRANT CONNECT ON DATABASE %I TO mini_migrator,mini_api,mini_retention,mini_backup,mini_restore,mini_controller_epoch',current_database()); END $$;
GRANT USAGE ON SCHEMA public TO mini_migrator,mini_api,mini_retention,mini_backup,mini_restore,mini_controller_epoch;

GRANT SELECT ON api_schema_version,api_authority_state,api_snapshot_state,api_request_history,api_request_events,api_model_operations,api_admin_events,api_alerts,api_safe_logs,api_metrics_hourly,api_operational_runs TO mini_api;
GRANT EXECUTE ON FUNCTION acquire_backend_authority(uuid),heartbeat_backend_authority(uuid,bigint),release_backend_authority(uuid,bigint),assert_backend_authority(uuid,bigint),admit_request(uuid,bigint,uuid,text,text,boolean,boolean,text,bigint,timestamptz,timestamptz,timestamptz),transition_request(uuid,bigint,uuid,text,text,text,smallint,timestamptz,timestamptz,timestamptz,bigint,bigint,bigint,bigint,bigint,bigint,bigint,boolean),publish_admin_event(uuid,bigint,text,jsonb),begin_model_operation(uuid,bigint,uuid,text,timestamptz),finish_model_operation(uuid,bigint,uuid,text,text,text),reconcile_prior_epoch(uuid,bigint),record_safe_log(uuid,bigint,text,text,uuid,uuid),upsert_alert(uuid,bigint,uuid,text,text),resolve_alert(uuid,bigint,text) TO mini_api;
GRANT USAGE,SELECT ON SEQUENCE backend_authority_epoch_seq TO mini_api;

GRANT EXECUTE ON FUNCTION aggregate_closed_hour(timestamptz,uuid),next_closed_aggregation_bucket(),purge_expired_metadata(uuid) TO mini_retention;
GRANT SELECT ON api_operational_runs TO mini_retention;

GRANT SELECT ON schema_migrations,backend_authority,admin_snapshot_state,inference_requests,request_events,model_operations,model_operation_events,admin_events,alerts,safe_log_events,inference_metrics_hourly,operational_runs TO mini_backup;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO mini_backup;
GRANT EXECUTE ON FUNCTION record_backup_run(uuid,text,timestamptz,timestamptz,text,bigint,text) TO mini_backup;

GRANT EXECUTE ON FUNCTION record_restore_proof(uuid,text,timestamptz,timestamptz,bigint,bigint,bigint,bigint,text),assert_restore_schema() TO mini_restore;
GRANT SELECT ON controller_authority_epoch TO mini_controller_epoch;

DO $$
DECLARE r record; object_kind text;
BEGIN
 FOR r IN SELECT c.relkind,n.nspname,c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
          WHERE n.nspname='public' AND c.relkind IN ('r','S','v') LOOP
   object_kind:=CASE r.relkind WHEN 'r' THEN 'TABLE' WHEN 'S' THEN 'SEQUENCE' ELSE 'VIEW' END;
   EXECUTE format('ALTER %s %I.%I OWNER TO mini_owner',object_kind,r.nspname,r.relname);
 END LOOP;
 FOR r IN SELECT n.nspname,p.proname,pg_get_function_identity_arguments(p.oid) AS args
          FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' LOOP
   EXECUTE format('ALTER FUNCTION %I.%I(%s) OWNER TO mini_owner',r.nspname,r.proname,r.args);
 END LOOP;
END $$;
ALTER SCHEMA public OWNER TO mini_owner;

INSERT INTO schema_migrations(version) VALUES(5);
COMMIT;
