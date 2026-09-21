BEGIN;

REVOKE EXECUTE ON FUNCTION assert_restore_schema() FROM mini_restore;
GRANT EXECUTE ON FUNCTION record_restore_proof(uuid,text,timestamptz,timestamptz,bigint,bigint,bigint,bigint,text) TO mini_restore;

INSERT INTO schema_migrations(version) VALUES(6);
COMMIT;
