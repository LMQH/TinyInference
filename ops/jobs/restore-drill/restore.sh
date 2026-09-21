#!/bin/sh
set -eu
umask 077
: "${RESTORE_DATABASE_URL_FILE:?}"
: "${RESTORE_ASSERT_DATABASE_URL_FILE:?}"
: "${RESTORE_ARCHIVE:?set an approved backup basename}"
: "${BACKUP_ROOT:?}"
: "${EVIDENCE_ROOT:?}"
: "${EXPECTED_SCHEMA_VERSION:?}"
case "$RESTORE_ARCHIVE" in mini-inference-????????T??????Z-"$EXPECTED_SCHEMA_VERSION".dump) ;; *) echo 'invalid restore archive basename' >&2; exit 1;; esac
[ -f "$RESTORE_DATABASE_URL_FILE" ] && [ ! -L "$RESTORE_DATABASE_URL_FILE" ] || exit 1
[ -f "$RESTORE_ASSERT_DATABASE_URL_FILE" ] && [ ! -L "$RESTORE_ASSERT_DATABASE_URL_FILE" ] || exit 1
archive="$BACKUP_ROOT/$RESTORE_ARCHIVE"
checksum="$archive.sha256"
[ -f "$archive" ] && [ ! -L "$archive" ] && [ -f "$checksum" ] && [ ! -L "$checksum" ] || exit 1
[ -d "$EVIDENCE_ROOT" ] && [ ! -L "$EVIDENCE_ROOT" ] || exit 1
(cd "$BACKUP_ROOT" && sha256sum -c "${checksum##*/}" >/dev/null)
pg_restore --list "$archive" >/dev/null
RESTORE_DATABASE_URL=$(cat "$RESTORE_DATABASE_URL_FILE")
RESTORE_ASSERT_DATABASE_URL=$(cat "$RESTORE_ASSERT_DATABASE_URL_FILE")
[ -n "$RESTORE_DATABASE_URL" ] && [ -n "$RESTORE_ASSERT_DATABASE_URL" ] || exit 1
cleanup() {
  unset RESTORE_DATABASE_URL RESTORE_ASSERT_DATABASE_URL result
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
started=$(date -u +%Y-%m-%dT%H:%M:%SZ)
pg_restore --exit-on-error --no-owner --no-privileges --dbname="$RESTORE_DATABASE_URL" "$archive" >/dev/null
result=$(psql --dbname="$RESTORE_ASSERT_DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 --field-separator='|' -c 'SELECT request_count,input_tokens,output_tokens,reasoning_tokens FROM assert_restore_schema()')
case "$result" in *'|'*'|'*'|'*) ;; *) echo 'restore assertion returned an invalid shape' >&2; exit 1;; esac
IFS='|' read -r count input output reasoning <<EOF
$result
EOF
for value in "$count" "$input" "$output" "$reasoning"; do case "$value" in ''|*[!0-9]*) exit 1;; esac; done
completed=$(date -u +%Y-%m-%dT%H:%M:%SZ)
id=$(cat /proc/sys/kernel/random/uuid)
tmp="$EVIDENCE_ROOT/restore-proof.env.partial"
final="$EVIDENCE_ROOT/restore-proof.env"
printf 'id=%s\nstatus=succeeded\nstarted=%s\ncompleted=%s\nrequest_count=%s\ninput_tokens=%s\noutput_tokens=%s\nreasoning_tokens=%s\n' \
  "$id" "$started" "$completed" "$count" "$input" "$output" "$reasoning" > "$tmp"
chmod 600 "$tmp"
mv "$tmp" "$final"
unset RESTORE_DATABASE_URL RESTORE_ASSERT_DATABASE_URL result
echo 'isolated restore and administrator-only content-safe schema/token assertions succeeded'
