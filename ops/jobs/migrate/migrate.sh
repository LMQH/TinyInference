#!/bin/sh
set -eu
umask 077
: "${DATABASE_URL_FILE:?}"
: "${ADMIN_DATABASE_URL_FILE:?}"
: "${MIGRATIONS_ROOT:?}"
: "${EXPECTED_SCHEMA_VERSION:?}"
[ "$EXPECTED_SCHEMA_VERSION" = 6 ] || { echo 'unexpected schema version' >&2; exit 1; }
[ -f "$DATABASE_URL_FILE" ] && [ ! -L "$DATABASE_URL_FILE" ] || exit 1
[ -f "$ADMIN_DATABASE_URL_FILE" ] && [ ! -L "$ADMIN_DATABASE_URL_FILE" ] || exit 1
DATABASE_URL=$(cat "$DATABASE_URL_FILE")
ADMIN_DATABASE_URL=$(cat "$ADMIN_DATABASE_URL_FILE")
[ -n "$DATABASE_URL" ] && [ -n "$ADMIN_DATABASE_URL" ] || exit 1

LOCK_PID=
LOCK_APP="mini_migrate_lock_$(tr -d '-' </proc/sys/kernel/random/uuid)"
case "$LOCK_APP" in *[!A-Za-z0-9_]*) echo 'failed to create migration lock identity' >&2; exit 1;; esac
cleanup() {
  PGCONNECT_TIMEOUT=2 PGAPPNAME=mini_migrate_lock_cleanup \
    psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "
      SELECT pg_terminate_backend(a.pid)
      FROM pg_stat_activity AS a
      JOIN pg_locks AS l ON l.pid=a.pid
      WHERE a.application_name='$LOCK_APP'
        AND a.pid<>pg_backend_pid()
        AND l.locktype='advisory'
        AND l.granted
        AND l.classid=0
        AND l.objid=741291550
        AND l.objsubid=1" >/dev/null 2>&1 || true
  [ -z "$LOCK_PID" ] || { kill "$LOCK_PID" 2>/dev/null || true; wait "$LOCK_PID" 2>/dev/null || true; }
  rm -f /tmp/runtime-role-passwords.sql
  unset DATABASE_URL ADMIN_DATABASE_URL LOCK_APP
}
trap cleanup EXIT HUP INT TERM
PGAPPNAME="$LOCK_APP" psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 >/dev/null 2>&1 <<'SQL' &
SELECT pg_sleep(86400)
FROM (SELECT pg_try_advisory_lock(741291550) AS held) AS lock_attempt
WHERE held;
SQL
LOCK_PID=$!
i=0
while :; do
  if ! kill -0 "$LOCK_PID" 2>/dev/null; then
    wait "$LOCK_PID" 2>/dev/null || true
    LOCK_PID=
    echo 'migration lock held' >&2
    exit 75
  fi
  owned=$(PGAPPNAME=mini_migrate_lock_probe psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c \
    "SELECT EXISTS (
       SELECT 1
       FROM pg_stat_activity AS a
       JOIN pg_locks AS l ON l.pid=a.pid
       WHERE a.application_name='$LOCK_APP'
         AND l.locktype='advisory'
         AND l.granted
         AND l.classid=0
         AND l.objid=741291550
         AND l.objsubid=1
     )")
  if [ "$owned" = t ] && kill -0 "$LOCK_PID" 2>/dev/null; then break; fi
  i=$((i+1))
  [ "$i" -le 100 ] || { echo 'migration lock handshake timed out' >&2; exit 1; }
  sleep 0.1
done

migration_path() {
  case "$1" in
    1) echo "$MIGRATIONS_ROOT/000001_authority.sql";;
    2) echo "$MIGRATIONS_ROOT/000002_requests.sql";;
    3) echo "$MIGRATIONS_ROOT/000003_lifecycle_admin.sql";;
    4) echo "$MIGRATIONS_ROOT/000004_metrics_operations.sql";;
    5) echo "$MIGRATIONS_ROOT/000005_roles_grants.sql";;
    6) echo "$MIGRATIONS_ROOT/000006_restore_least_privilege.sql";;
    *) exit 2;;
  esac
}
apply_version() {
  version=$1
  path=$(migration_path "$version")
  [ -f "$path" ] && [ ! -L "$path" ] || { echo 'migration set incomplete' >&2; exit 1; }
  case "$version" in
    1|2|3|4)
      psql --dbname="$DATABASE_URL" -X --no-psqlrc --set=ON_ERROR_STOP=1 \
        --command='SET ROLE mini_owner' --file="$path" >/dev/null
      ;;
    5|6)
      psql --dbname="$DATABASE_URL" -X --no-psqlrc --set=ON_ERROR_STOP=1 \
        --file="$path" >/dev/null
      ;;
  esac
  [ "$(psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "SELECT count(*) FROM public.schema_migrations WHERE version=$version")" = 1 ] || {
    echo 'migration did not record its version' >&2; exit 1;
  }
  echo "migration applied: version $version"
}

exists=$(psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "SELECT to_regclass('public.schema_migrations') IS NOT NULL")
[ "$exists" = t ] || apply_version 1
versions=$(psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c 'SELECT version FROM public.schema_migrations ORDER BY version')
expected=1
for version in $versions; do
  [ "$version" = "$expected" ] || { echo 'migration history is not a contiguous prefix' >&2; exit 1; }
  expected=$((expected+1))
done
[ "$expected" -le 7 ] || { echo 'database migration is newer than this release' >&2; exit 1; }
while [ "$expected" -le 6 ]; do apply_version "$expected"; expected=$((expected+1)); done

migrator_needs_finalization() {
  psql --dbname="$ADMIN_DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "
    SELECT r.rolsuper
        OR r.rolcreatedb
        OR r.rolcreaterole
        OR r.rolreplication
        OR r.rolbypassrls
        OR has_database_privilege('mini_migrator', current_database(), 'CREATE')
        OR has_database_privilege('mini_migrator', current_database(), 'CONNECT WITH GRANT OPTION')
        OR EXISTS (
          SELECT 1
          FROM pg_database AS d,
               LATERAL aclexplode(COALESCE(d.datacl, acldefault('d', d.datdba))) AS acl
          WHERE d.datname=current_database()
            AND acl.grantee=0
            AND acl.privilege_type='CONNECT'
        )
        OR has_database_privilege('mini_owner', current_database(), 'CREATE')
        OR has_database_privilege('mini_restore', current_database(), 'CONNECT')
        OR EXISTS (
          SELECT 1
          FROM pg_authid AS restore_role
          WHERE restore_role.rolname='mini_restore'
            AND restore_role.rolpassword IS NOT NULL
        )
        OR EXISTS (
          SELECT 1
          FROM pg_proc AS p
          JOIN pg_namespace AS n ON n.oid=p.pronamespace
          WHERE n.nspname='public'
            AND has_function_privilege('mini_restore', p.oid, 'EXECUTE')
        )
        OR EXISTS (
          SELECT 1
          FROM pg_auth_members AS m
          JOIN pg_roles AS granted_role ON granted_role.oid=m.roleid
          WHERE m.member=r.oid
            AND granted_role.rolname='mini_owner'
            AND m.admin_option
        )
    FROM pg_roles AS r
    WHERE r.rolname='mini_migrator'"
}

needs_finalization=$(migrator_needs_finalization)
case "$needs_finalization" in
  t)
    roles=/tmp/runtime-role-passwords.sql
    : > "$roles"
    chmod 600 "$roles"
    printf '%s\n' 'BEGIN;' >> "$roles"
    for role in mini_api mini_retention mini_backup mini_controller_epoch
    do
      file="/run/secrets/${role}_password"
      [ -f "$file" ] && [ ! -L "$file" ] || exit 1
      value=$(cat "$file")
      case "$value" in ''|*[!A-Za-z0-9_.~-]*) echo "invalid secret format for $role" >&2; exit 1;; esac
      printf "ALTER ROLE %s PASSWORD '%s';\n" "$role" "$value" >> "$roles"
    done
    printf '%s\n' \
      'DO $finalize$' \
      'BEGIN' \
      "  EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM PUBLIC', current_database());" \
      "  EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM mini_restore', current_database());" \
      "  EXECUTE format('REVOKE GRANT OPTION FOR CONNECT ON DATABASE %I FROM mini_migrator CASCADE', current_database());" \
      "  EXECUTE format('REVOKE CREATE ON DATABASE %I FROM mini_migrator, mini_owner', current_database());" \
      'END' \
      '$finalize$;' >> "$roles"
    printf '%s\n' 'REVOKE ALL ON ALL FUNCTIONS IN SCHEMA public FROM mini_restore;' >> "$roles"
    printf '%s\n' 'ALTER ROLE mini_restore PASSWORD NULL;' >> "$roles"
    printf '%s\n' 'REVOKE ADMIN OPTION FOR mini_owner FROM mini_migrator CASCADE;' >> "$roles"
    printf '%s\n' 'ALTER ROLE mini_migrator NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;' >> "$roles"
    printf '%s\n' 'ALTER ROLE mini_migrator NOSUPERUSER;' >> "$roles"
    printf '%s\n' 'COMMIT;' >> "$roles"
    psql --dbname="$ADMIN_DATABASE_URL" -X --no-psqlrc --set=ON_ERROR_STOP=1 --file="$roles" >/dev/null
    rm -f "$roles"
    unset value
    [ "$(migrator_needs_finalization)" = f ] || { echo 'migrator demotion was not confirmed' >&2; exit 1; }
    ;;
  f) ;;
  *) echo 'unable to determine migrator capability state' >&2; exit 1;;
esac
unset needs_finalization
echo 'schema version 6 confirmed'
