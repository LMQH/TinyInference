BEGIN;
CREATE TABLE schema_migrations (
    version bigint PRIMARY KEY CHECK (version > 0),
    applied_at timestamptz NOT NULL DEFAULT transaction_timestamp()
);


CREATE SEQUENCE backend_authority_epoch_seq AS bigint MINVALUE 1 NO CYCLE;
CREATE SEQUENCE admin_snapshot_version_seq AS bigint MINVALUE 1 NO CYCLE;

CREATE TABLE backend_authority (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    epoch bigint NOT NULL CHECK (epoch > 0),
    holder_id uuid NOT NULL,
    acquired_at timestamptz NOT NULL,
    heartbeat_at timestamptz NOT NULL,
    released_at timestamptz,
    CHECK (heartbeat_at >= acquired_at),
    CHECK (released_at IS NULL OR released_at >= acquired_at)
);

CREATE TABLE admin_snapshot_state (
    singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
    snapshot_version bigint NOT NULL UNIQUE,
    updated_at timestamptz NOT NULL
);
INSERT INTO admin_snapshot_state(singleton, snapshot_version, updated_at)
VALUES (true, nextval('admin_snapshot_version_seq'), transaction_timestamp());

CREATE FUNCTION acquire_backend_authority(p_holder_id uuid)
RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE v_epoch bigint;
BEGIN
    IF p_holder_id IS NULL THEN RAISE EXCEPTION USING ERRCODE='22023', MESSAGE='invalid_holder'; END IF;
    v_epoch := nextval('public.backend_authority_epoch_seq');
    INSERT INTO public.backend_authority(singleton,epoch,holder_id,acquired_at,heartbeat_at,released_at)
    VALUES(true,v_epoch,p_holder_id,transaction_timestamp(),transaction_timestamp(),NULL)
    ON CONFLICT(singleton) DO UPDATE SET epoch=EXCLUDED.epoch,holder_id=EXCLUDED.holder_id,
        acquired_at=EXCLUDED.acquired_at,heartbeat_at=EXCLUDED.heartbeat_at,released_at=NULL;
    RETURN v_epoch;
END $$;

CREATE FUNCTION heartbeat_backend_authority(p_holder_id uuid, p_epoch bigint)
RETURNS void
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
BEGIN
    UPDATE public.backend_authority SET heartbeat_at=transaction_timestamp()
      WHERE singleton AND holder_id=p_holder_id AND epoch=p_epoch AND released_at IS NULL;
    IF NOT FOUND THEN RAISE EXCEPTION USING ERRCODE='P0001', MESSAGE='authority_fence_lost'; END IF;
END $$;

CREATE FUNCTION release_backend_authority(p_holder_id uuid, p_epoch bigint)
RETURNS void
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
BEGIN
    UPDATE public.backend_authority SET released_at=transaction_timestamp()
      WHERE singleton AND holder_id=p_holder_id AND epoch=p_epoch AND released_at IS NULL;
    IF NOT FOUND THEN RAISE EXCEPTION USING ERRCODE='P0001', MESSAGE='authority_fence_lost'; END IF;
END $$;

CREATE FUNCTION assert_backend_authority(p_holder_id uuid, p_epoch bigint)
RETURNS void
LANGUAGE plpgsql SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE ok boolean;
BEGIN
    SELECT true INTO ok FROM public.backend_authority
      WHERE singleton AND holder_id=p_holder_id AND epoch=p_epoch AND released_at IS NULL FOR UPDATE;
    IF ok IS DISTINCT FROM true THEN RAISE EXCEPTION USING ERRCODE='P0001', MESSAGE='authority_fence_lost'; END IF;
END $$;

CREATE VIEW controller_authority_epoch
WITH (security_barrier=true) AS
SELECT epoch, holder_id, heartbeat_at FROM backend_authority WHERE singleton AND released_at IS NULL;

REVOKE ALL ON schema_migrations, backend_authority, admin_snapshot_state FROM PUBLIC;
REVOKE ALL ON SEQUENCE backend_authority_epoch_seq, admin_snapshot_version_seq FROM PUBLIC;
REVOKE ALL ON FUNCTION acquire_backend_authority(uuid), heartbeat_backend_authority(uuid,bigint), release_backend_authority(uuid,bigint), assert_backend_authority(uuid,bigint) FROM PUBLIC;
REVOKE ALL ON controller_authority_epoch FROM PUBLIC;

INSERT INTO schema_migrations(version) VALUES(1);
COMMIT;
