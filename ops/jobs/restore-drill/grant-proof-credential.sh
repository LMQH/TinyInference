#!/bin/sh
set -eu
umask 077
: "${ADMIN_DATABASE_URL_FILE:?}"
: "${RESTORE_PASSWORD_FILE:?}"
[ -f "$ADMIN_DATABASE_URL_FILE" ] && [ ! -L "$ADMIN_DATABASE_URL_FILE" ] || exit 1
[ -f "$RESTORE_PASSWORD_FILE" ] && [ ! -L "$RESTORE_PASSWORD_FILE" ] || exit 1
ADMIN_DATABASE_URL=$(cat "$ADMIN_DATABASE_URL_FILE")
RESTORE_PASSWORD=$(cat "$RESTORE_PASSWORD_FILE")
[ -n "$ADMIN_DATABASE_URL" ] || exit 1
case "$RESTORE_PASSWORD" in ''|*[!A-Za-z0-9_.~-]*) echo 'invalid mini_restore password secret' >&2; exit 1;; esac
GRANT_SQL=/tmp/restore-proof-grant.sql
cleanup() {
  rm -f "$GRANT_SQL"
  unset ADMIN_DATABASE_URL RESTORE_PASSWORD state GRANT_SQL
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

cat >"$GRANT_SQL" <<'SQL'
BEGIN;
DO $grant$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM pg_stat_activity
    WHERE usename='mini_restore'
      AND pid<>pg_backend_pid()
  ) THEN
    RAISE EXCEPTION 'mini_restore has an active production session';
  END IF;
  IF (SELECT max(version) FROM public.schema_migrations) <> 7 THEN
    RAISE EXCEPTION 'schema version 7 is required';
  END IF;
  EXECUTE format('GRANT CONNECT ON DATABASE %I TO mini_restore', current_database());
END
$grant$;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM mini_restore;
GRANT EXECUTE ON FUNCTION public.record_restore_proof(uuid,text,timestamptz,timestamptz,bigint,bigint,bigint,bigint,text) TO mini_restore;
SQL
printf "ALTER ROLE mini_restore PASSWORD '%s';\n" "$RESTORE_PASSWORD" >>"$GRANT_SQL"
unset RESTORE_PASSWORD
printf '%s\n' 'COMMIT;' >>"$GRANT_SQL"
psql --dbname="$ADMIN_DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 --file="$GRANT_SQL" >/dev/null
rm -f "$GRANT_SQL"

state=$(psql --dbname="$ADMIN_DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "
  SELECT r.rolpassword IS NOT NULL
     AND has_database_privilege('mini_restore', current_database(), 'CONNECT')
     AND has_function_privilege(
       'mini_restore',
       'public.record_restore_proof(uuid,text,timestamp with time zone,timestamp with time zone,bigint,bigint,bigint,bigint,text)',
       'EXECUTE'
     )
     AND NOT EXISTS (
       SELECT 1
       FROM pg_proc AS p
       JOIN pg_namespace AS n ON n.oid=p.pronamespace
       WHERE n.nspname='public'
         AND p.oid<>'public.record_restore_proof(uuid,text,timestamp with time zone,timestamp with time zone,bigint,bigint,bigint,bigint,text)'::regprocedure
         AND has_function_privilege('mini_restore', p.oid, 'EXECUTE')
     )
     AND NOT has_function_privilege(
       'mini_restore',
       'public.assert_restore_schema()',
       'EXECUTE'
     )
  FROM pg_authid AS r
  WHERE r.rolname='mini_restore'")
[ "$state" = t ] || { echo 'proof credential grant was not confirmed' >&2; exit 1; }
unset state RESTORE_PASSWORD ADMIN_DATABASE_URL
echo 'production restore proof credential granted'
