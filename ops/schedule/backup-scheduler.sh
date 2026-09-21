#!/bin/sh
set -eu
: "${DATABASE_URL_FILE:?}"
: "${BACKUP_ROOT:?}"
: "${BACKUP_SCHEDULE_UTC:?}"
: "${BACKUP_FRESHNESS_SECONDS:=93600}"
[ "$BACKUP_SCHEDULE_UTC" = 03:00 ] || { echo 'backup schedule must be 03:00 UTC' >&2; exit 1; }
[ "$BACKUP_FRESHNESS_SECONDS" = 93600 ] || { echo 'backup freshness must be 26 hours' >&2; exit 1; }
DATABASE_URL=$(cat "$DATABASE_URL_FILE")
[ -n "$DATABASE_URL" ] || exit 1
trap 'unset DATABASE_URL' EXIT HUP INT TERM

latest_day() {
  psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "SELECT COALESCE(to_char(max(completed_at) AT TIME ZONE 'UTC','YYYY-MM-DD'),'') FROM operational_runs WHERE kind='backup' AND status='succeeded'"
}
latest_age() {
  psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c "SELECT COALESCE(EXTRACT(EPOCH FROM (transaction_timestamp()-max(completed_at)))::bigint, -1) FROM operational_runs WHERE kind='backup' AND status='succeeded'"
}
catch_up_if_required() {
  day=$(date -u +%Y-%m-%d)
  prior_day=$(latest_day)
  age=$(latest_age)
  case "$age" in *[!0-9-]*|'') exit 1;; esac
  if [ "$prior_day" != "$day" ] || [ "$age" -lt 0 ] || [ "$age" -gt "$BACKUP_FRESHNESS_SECONDS" ]; then
    /opt/mini-inference/jobs/backup/backup.sh cycle || true
  else
    /opt/mini-inference/jobs/backup/backup.sh prune || true
  fi
}
health() {
  [ -d "$BACKUP_ROOT" ] && [ ! -L "$BACKUP_ROOT" ] || exit 1
  psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 -c 'SELECT 1' >/dev/null
  age=$(latest_age)
  case "$age" in *[!0-9-]*|'') exit 1;; esac
  [ "$age" -ge 0 ] && [ "$age" -le "$BACKUP_FRESHNESS_SECONDS" ]
}

if [ "${1:-run}" = health ]; then health; exit; fi
[ "${1:-run}" = run ] || exit 2
catch_up_if_required
last_scheduled_day=$(latest_day)
previous_epoch=$(date -u +%s)
while :; do
  sleep 60
  now_epoch=$(date -u +%s)
  delta=$((now_epoch - previous_epoch))
  previous_epoch=$now_epoch
  if [ "$delta" -lt 0 ] || [ "$delta" -gt 120 ]; then
    catch_up_if_required
  fi
  utc_day=$(date -u +%Y-%m-%d)
  utc_hm=$(date -u +%H:%M)
  if [ "$utc_hm" = "$BACKUP_SCHEDULE_UTC" ] && [ "$last_scheduled_day" != "$utc_day" ]; then
    /opt/mini-inference/jobs/backup/backup.sh cycle || true
    last_scheduled_day=$utc_day
  fi
done
