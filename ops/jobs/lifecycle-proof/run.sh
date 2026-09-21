#!/bin/sh
set -eu
: "${DATABASE_URL_FILE:?}"
: "${CONTROLLER_BASE_URL:?}"
[ "$CONTROLLER_BASE_URL" = http://controller:9090 ] || exit 1
[ -f "$DATABASE_URL_FILE" ] && [ ! -L "$DATABASE_URL_FILE" ] || exit 1
DATABASE_URL=$(cat "$DATABASE_URL_FILE")
[ -n "$DATABASE_URL" ] || exit 1
trap 'unset DATABASE_URL authority AUTHORITY_EPOCH AUTHORITY_HOLDER' EXIT HUP INT TERM
authority=$(psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 --field-separator='|' -c "SELECT epoch,holder_id FROM public.controller_authority_epoch WHERE heartbeat_at >= transaction_timestamp()-interval '6 seconds'")
case "$authority" in *'|'*) ;; *) echo 'no fresh controller authority' >&2; exit 1;; esac
AUTHORITY_EPOCH=${authority%%|*}
AUTHORITY_HOLDER=${authority#*|}
export AUTHORITY_EPOCH AUTHORITY_HOLDER
/usr/local/bin/lifecycle-proof positive
/usr/local/bin/lifecycle-proof negative
unset DATABASE_URL authority AUTHORITY_EPOCH AUTHORITY_HOLDER
echo 'fixed lifecycle positive and ingress-negative proof completed'
