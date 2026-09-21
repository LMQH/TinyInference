#!/bin/sh
set -eu
umask 077
: "${ADMIN_DATABASE_URL_FILE:?}"
[ -f "$ADMIN_DATABASE_URL_FILE" ] && [ ! -L "$ADMIN_DATABASE_URL_FILE" ] || exit 1
ADMIN_DATABASE_URL=$(cat "$ADMIN_DATABASE_URL_FILE")
[ -n "$ADMIN_DATABASE_URL" ] || exit 1
cleanup() {
  unset ADMIN_DATABASE_URL state
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

psql --dbname="$ADMIN_DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 <<'SQL' >/dev/null
BEGIN;
DO $revoke$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM pg_stat_activity
    WHERE usename='mini_restore'
      AND pid<>pg_backend_pid()
  ) THEN
    RAISE EXCEPTION 'mini_restore has an active production session';
  END IF;
  EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM PUBLIC', current_database());
  EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM mini_restore', current_database());
END
$revoke$;
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM mini_restore;
ALTER ROLE mini_restore PASSWORD NULL;
COMMIT;
SQL

state=$(psql --dbname="$ADMIN_DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "
  SELECT r.rolpassword IS NULL
     AND NOT has_database_privilege('mini_restore', current_database(), 'CONNECT')
     AND NOT EXISTS (
       SELECT 1
       FROM pg_proc AS p
       JOIN pg_namespace AS n ON n.oid=p.pronamespace
       WHERE n.nspname='public'
         AND has_function_privilege('mini_restore', p.oid, 'EXECUTE')
     )
     AND NOT EXISTS (
       SELECT 1
       FROM pg_stat_activity
       WHERE usename='mini_restore'
         AND pid<>pg_backend_pid()
     )
  FROM pg_authid AS r
  WHERE r.rolname='mini_restore'")
[ "$state" = t ] || { echo 'proof credential revocation was not confirmed' >&2; exit 1; }
unset state ADMIN_DATABASE_URL
echo 'production restore proof credential revoked'
