#!/bin/sh
set -eu
: "${DATABASE_URL_FILE:?}"
: "${EVIDENCE_FILE:?}"
[ -f "$DATABASE_URL_FILE" ] && [ ! -L "$DATABASE_URL_FILE" ] || exit 1
[ -f "$EVIDENCE_FILE" ] && [ ! -L "$EVIDENCE_FILE" ] || exit 1
id= status= started= completed= request_count= input_tokens= output_tokens= reasoning_tokens=
while IFS='=' read -r key value; do
  case "$key" in
    id) id=$value;; status) status=$value;; started) started=$value;; completed) completed=$value;;
    request_count) request_count=$value;; input_tokens) input_tokens=$value;;
    output_tokens) output_tokens=$value;; reasoning_tokens) reasoning_tokens=$value;;
    *) echo 'unexpected restore evidence field' >&2; exit 1;;
  esac
done < "$EVIDENCE_FILE"
case "$id" in ????????-????-????-????-????????????) ;; *) exit 1;; esac
[ "$status" = succeeded ] || exit 1
for value in "$request_count" "$input_tokens" "$output_tokens" "$reasoning_tokens"; do case "$value" in ''|*[!0-9]*) exit 1;; esac; done
DATABASE_URL=$(cat "$DATABASE_URL_FILE")
[ -n "$DATABASE_URL" ] || exit 1
trap 'unset DATABASE_URL' EXIT HUP INT TERM
psql --dbname="$DATABASE_URL" -X -qAt --no-psqlrc --set=ON_ERROR_STOP=1 \
  --variable=id="$id" --variable=status="$status" --variable=started="$started" --variable=completed="$completed" \
  --variable=count="$request_count" --variable=input="$input_tokens" --variable=output="$output_tokens" --variable=reasoning="$reasoning_tokens" <<'SQL' >/dev/null
SELECT record_restore_proof(:'id'::uuid, :'status', :'started'::timestamptz, :'completed'::timestamptz,
  :'count'::bigint, :'input'::bigint, :'output'::bigint, :'reasoning'::bigint, NULL);
SQL
unset DATABASE_URL
echo 'content-safe restore proof recorded'
