BEGIN;

CREATE TABLE public_model_identity (
 singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
 public_model_id text NOT NULL CHECK (public_model_id ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$')
);
INSERT INTO public_model_identity(singleton,public_model_id)
VALUES (true,'openbmb/MiniCPM5-2B-Q4_K_M');

CREATE VIEW api_public_model_identity WITH (security_barrier=true) AS
SELECT public_model_id FROM public_model_identity WHERE singleton;

CREATE FUNCTION assert_current_public_model_id(p_name text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 PERFORM 1 FROM public.public_model_identity WHERE singleton AND public_model_id=p_name FOR SHARE;
 IF NOT FOUND THEN RAISE EXCEPTION USING ERRCODE='P0003',MESSAGE='public_model_id_changed'; END IF;
END $$;

CREATE FUNCTION set_public_model_id(p_holder uuid,p_epoch bigint,p_expected text,p_new text)
RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE v_current text; v_version bigint;
BEGIN
 PERFORM public.assert_backend_authority(p_holder,p_epoch);
 IF p_new IS NULL OR p_new !~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$' THEN
  RAISE EXCEPTION USING ERRCODE='22023',MESSAGE='invalid_public_model_id';
 END IF;
 SELECT public_model_id INTO STRICT v_current FROM public.public_model_identity WHERE singleton FOR UPDATE;
 IF v_current IS DISTINCT FROM p_expected THEN
  RAISE EXCEPTION USING ERRCODE='P0002',MESSAGE='stale_public_model_id';
 END IF;
 IF v_current=p_new THEN
  SELECT snapshot_version INTO STRICT v_version FROM public.admin_snapshot_state WHERE singleton;
  RETURN v_version;
 END IF;
 UPDATE public.public_model_identity SET public_model_id=p_new WHERE singleton;
 RETURN public.publish_admin_event(p_holder,p_epoch,'model_changed',jsonb_build_object('changed',jsonb_build_array('model')));
END $$;

REVOKE ALL ON public_model_identity,api_public_model_identity FROM PUBLIC;
REVOKE ALL ON FUNCTION set_public_model_id(uuid,bigint,text,text) FROM PUBLIC;
REVOKE ALL ON FUNCTION assert_current_public_model_id(text) FROM PUBLIC;
GRANT SELECT ON api_public_model_identity TO mini_api;
GRANT EXECUTE ON FUNCTION set_public_model_id(uuid,bigint,text,text) TO mini_api;
GRANT EXECUTE ON FUNCTION assert_current_public_model_id(text) TO mini_api;
GRANT SELECT ON public_model_identity TO mini_backup;
INSERT INTO schema_migrations(version) VALUES (7);
COMMIT;
