#!/bin/sh
set -eu
umask 077
: "${DATABASE_URL_FILE:?}"
: "${BACKUP_ROOT:?}"
: "${BACKUP_MAX_AGE_SECONDS:=604800}"
: "${BACKUP_FRESHNESS_SECONDS:=93600}"
[ "$BACKUP_MAX_AGE_SECONDS" = 604800 ] || { echo 'backup retention must be exactly seven days' >&2; exit 1; }
[ -f "$DATABASE_URL_FILE" ] && [ ! -L "$DATABASE_URL_FILE" ] || exit 1
[ -d "$BACKUP_ROOT" ] && [ ! -L "$BACKUP_ROOT" ] || exit 1
root=$(CDPATH= cd -- "$BACKUP_ROOT" && pwd -P)
[ "$root" = "$BACKUP_ROOT" ] || { echo 'backup root must be canonical' >&2; exit 1; }
DATABASE_URL=$(cat "$DATABASE_URL_FILE")
[ -n "$DATABASE_URL" ] || exit 1

safe_inventory() {
  unsafe=0
  for item in "$BACKUP_ROOT"/*; do
    [ -e "$item" ] || [ -L "$item" ] || continue
    base=${item##*/}
    case "$base" in
      mini-inference-????????T??????Z-[0-9]*.dump|mini-inference-????????T??????Z-[0-9]*.dump.sha256|*.partial) ;;
      *) echo 'unexpected entry in backup root' >&2; unsafe=1;;
    esac
    [ ! -L "$item" ] || { echo 'symbolic link rejected in backup root' >&2; unsafe=1; }
  done
  [ "$unsafe" -eq 0 ]
}

prune() {
  safe_inventory
  now=$(date -u +%s)
  for item in "$BACKUP_ROOT"/*.dump "$BACKUP_ROOT"/*.dump.sha256; do
    [ -e "$item" ] || continue
    [ -f "$item" ] && [ ! -L "$item" ] || exit 1
    mtime=$(stat -c %Y "$item")
    age=$((now - mtime))
    [ "$age" -gt "$BACKUP_MAX_AGE_SECONDS" ] && rm -f -- "$item"
  done
  current=0
  for archive in "$BACKUP_ROOT"/*.dump; do
    [ -f "$archive" ] && [ ! -L "$archive" ] || continue
    checksum="$archive.sha256"
    [ -f "$checksum" ] && [ ! -L "$checksum" ] || continue
    mtime=$(stat -c %Y "$archive")
    age=$((now - mtime))
    if [ "$age" -le "$BACKUP_MAX_AGE_SECONDS" ] && (cd "$BACKUP_ROOT" && sha256sum -c "${checksum##*/}" >/dev/null 2>&1); then
      current=$((current + 1))
    fi
  done
  RETAINED=$current
  [ "$current" -gt 0 ] || { echo 'critical: no valid backup remains inside the seven-day window' >&2; return 1; }
}

if [ "${1:-cycle}" = prune ]; then
  RETAINED=0
  prune
  echo "backup pruning complete; retained_count=$RETAINED"
  exit 0
fi
[ "${1:-cycle}" = cycle ] || { echo 'unsupported fixed backup action' >&2; exit 2; }
: "${SCHEMA_VERSION:?}"
case "$SCHEMA_VERSION" in *[!0-9]*|'') exit 1;; esac

RUN_ID=$(cat /proc/sys/kernel/random/uuid)
STARTED=$(date -u +%Y-%m-%dT%H:%M:%SZ)
STATUS=failed
ERROR_CODE=backup_failed
ARTIFACT=
RETAINED=0
LOCK_PID=
LOCK_BACKEND_PID=
LOCK_APP="mini_backup_lock_$(tr -d '-' </proc/sys/kernel/random/uuid)"
RECORD_RESULT=0
case "$LOCK_APP" in *[!A-Za-z0-9_]*) echo 'failed to create backup lock identity' >&2; exit 1;; esac

record_result() {
  completed=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 \
    --variable=id="$RUN_ID" --variable=status="$STATUS" --variable=started="$STARTED" \
    --variable=completed="$completed" --variable=artifact="$ARTIFACT" \
    --variable=retained="$RETAINED" --variable=error="$ERROR_CODE" <<'SQL' >/dev/null 2>&1 || true
SELECT record_backup_run(:'id'::uuid, :'status', :'started'::timestamptz, :'completed'::timestamptz,
  NULLIF(:'artifact',''), :'retained'::bigint, NULLIF(:'error',''));
SQL
}

cleanup() {
  trap - EXIT HUP INT TERM
  pid_filter=
  case "$LOCK_BACKEND_PID" in
    '') ;;
    *[!0-9]*) ;;
    *) pid_filter="AND a.pid=$LOCK_BACKEND_PID";;
  esac
  PGCONNECT_TIMEOUT=2 PGAPPNAME=mini_backup_lock_cleanup \
    psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "
      SELECT pg_terminate_backend(a.pid)
      FROM pg_stat_activity AS a
      JOIN pg_locks AS l ON l.pid=a.pid
      WHERE a.application_name='$LOCK_APP'
        AND a.usename=current_user
        AND a.pid<>pg_backend_pid()
        AND l.locktype='advisory'
        AND l.granted
        AND l.classid=0
        AND l.objid=741291553
        AND l.objsubid=1
        $pid_filter" >/dev/null 2>&1 || true
  [ -z "$LOCK_PID" ] || { kill "$LOCK_PID" 2>/dev/null || true; wait "$LOCK_PID" 2>/dev/null || true; }
  [ "$RECORD_RESULT" -eq 0 ] || record_result
  unset DATABASE_URL LOCK_APP LOCK_BACKEND_PID pid_filter
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

PGAPPNAME="$LOCK_APP" psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 >/dev/null 2>&1 <<'SQL' &
SELECT pg_sleep(86400)
FROM (SELECT pg_try_advisory_lock(741291553) AS held) AS lock_attempt
WHERE held;
SQL
LOCK_PID=$!
i=0
while :; do
  if ! kill -0 "$LOCK_PID" 2>/dev/null; then
    wait "$LOCK_PID" 2>/dev/null || true
    LOCK_PID=
    echo 'backup cycle skipped: lock held' >&2
    exit 75
  fi
  lock_state=$(PGAPPNAME=mini_backup_lock_probe psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "
    SELECT 'LOCKED|' || a.pid
    FROM pg_stat_activity AS a
    JOIN pg_locks AS l ON l.pid=a.pid
    WHERE a.application_name='$LOCK_APP'
      AND a.usename=current_user
      AND l.locktype='advisory'
      AND l.granted
      AND l.classid=0
      AND l.objid=741291553
      AND l.objsubid=1")
  case "$lock_state" in
    LOCKED\|*)
      LOCK_BACKEND_PID=${lock_state#LOCKED|}
      case "$LOCK_BACKEND_PID" in ''|*[!0-9]*) echo 'invalid backup lock handshake' >&2; exit 1;; esac
      kill -0 "$LOCK_PID" 2>/dev/null || { echo 'backup lock helper exited during handshake' >&2; exit 1; }
      break
      ;;
    '') ;;
    *) echo 'invalid backup lock handshake' >&2; exit 1;;
  esac
  i=$((i + 1))
  [ "$i" -le 100 ] || { echo 'backup lock handshake timed out' >&2; exit 1; }
  sleep 0.1
done
unset lock_state
RECORD_RESULT=1

ts=$(date -u +%Y%m%dT%H%M%SZ)
ARTIFACT="mini-inference-$ts-$SCHEMA_VERSION.dump"
partial="$BACKUP_ROOT/$ARTIFACT.partial"
final="$BACKUP_ROOT/$ARTIFACT"
checksum_partial="$BACKUP_ROOT/$ARTIFACT.sha256.partial"
checksum="$BACKUP_ROOT/$ARTIFACT.sha256"
[ ! -e "$partial" ] && [ ! -e "$final" ] && [ ! -e "$checksum_partial" ] && [ ! -e "$checksum" ] || exit 1
pg_dump --dbname="$DATABASE_URL" --format=custom --no-owner --no-privileges --file="$partial"
chmod 600 "$partial"
pg_restore --list "$partial" >/dev/null
(cd "$BACKUP_ROOT" && sha256sum "$ARTIFACT.partial" | sed 's/\.partial$//' > "$ARTIFACT.sha256.partial")
chmod 600 "$checksum_partial"
mv "$partial" "$final"
mv "$checksum_partial" "$checksum"
prune
STATUS=succeeded
ERROR_CODE=
echo "backup complete; artifact=$ARTIFACT retained_count=$RETAINED"
