#!/bin/sh
set -eu
umask 077
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd -P)
cd "$ROOT"
if [ "${MINI_PRIVACY_TARGET:-}" = candidate ]; then
  python3 ops/candidate/check.py
  API_KEY=888888
  API_BASE=http://127.0.0.1:18888
  COMPOSE='docker compose --project-name mini-inference-candidate --env-file var/artifacts/candidate/runtime.conf -f compose.yaml -f compose.candidate.yaml'
elif [ -z "${MINI_PRIVACY_TARGET:-}" ]; then
  [ -f var/secrets/api_key ] && [ ! -L var/secrets/api_key ] || exit 1
  LAN_BIND_ADDRESS=$(sed -n 's/^LAN_BIND_ADDRESS=//p' config/runtime.mac.conf)
  case "$LAN_BIND_ADDRESS" in ''|*[!0-9A-Fa-f:.]*) echo 'invalid private bind address configuration' >&2; exit 1;; esac
  API_KEY=$(cat var/secrets/api_key)
  [ -n "$API_KEY" ] || exit 1
  API_BASE="http://$LAN_BIND_ADDRESS:8888"
  COMPOSE='docker compose --project-name mini-inference --env-file config/runtime.mac.conf'
else
  echo 'invalid privacy target' >&2; exit 1
fi
MODEL_ID=$(curl --fail --silent --show-error -H "Authorization: Bearer $API_KEY" "$API_BASE/v1/models" | jq -er '.data | if length == 1 then .[0].id else empty end | select(test("^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$"))')
[ -n "$MODEL_ID" ] || exit 1
CANARY_A="privacy-normal-$(python3 -c 'import secrets; print(secrets.token_hex(16))')"
CANARY_B="privacy-reasoning-$(python3 -c 'import secrets; print(secrets.token_hex(16))')"
CANARY_C="privacy-tool-$(python3 -c 'import secrets; print(secrets.token_hex(16))')"
CANARY_HEXES=$(printf '%s\n%s\n%s\n' "$CANARY_A" "$CANARY_B" "$CANARY_C" | python3 -c 'import sys; print(",".join(x.rstrip("\n").encode().hex() for x in sys.stdin))')
export CANARY_HEXES
DMR="$ROOT/var/artifacts/compatible-runtime/bin/dmr"
DMR_HOST=http://127.0.0.1:12435
[ -x "$DMR" ] || { echo 'compatible DMR unavailable' >&2; exit 1; }

scan_pipe() {
  label=$1
  shift
  pipe_dir=$(mktemp -d "$ROOT/var/artifacts/mini-inference-privacy.XXXXXX")
  fifo="$pipe_dir/stream"
  mkfifo "$fifo"
  "$@" >"$fifo" 2>/dev/null &
  producer=$!
  if count=$(python3 "$ROOT/ops/privacy-proof/count-matches.py" <"$fifo"); then
    detector=0
  else
    detector=$?
  fi
  if wait "$producer"; then producer_status=0; else producer_status=$?; fi
  rm -f "$fifo"
  rmdir "$pipe_dir"
  [ "$producer_status" -eq 0 ] && [ "$detector" -eq 0 ] && [ "$count" = 0 ] || {
    echo "privacy scan failed or found prohibited content: $label" >&2
    exit 1
  }
  echo "privacy scan: $label matches=0"
}
scan_repo() {
  count=$(python3 - "$ROOT" <<'PY'
import os,sys
needles=[bytes.fromhex(x) for x in os.environ['CANARY_HEXES'].split(',')]
root=sys.argv[1]; hits=0
excluded={'.git','node_modules'}
max_needle=max(map(len,needles))
for base,dirs,files in os.walk(root):
    dirs[:]=[d for d in dirs if d not in excluded and not (base==root and d=='models')]
    for name in files:
        path=os.path.join(base,name)
        if os.path.islink(path) or not os.path.isfile(path): continue
        try:
            with open(path,'rb') as f:
                carry=b''
                while True:
                    chunk=f.read(1024*1024)
                    if not chunk: break
                    data=carry+chunk
                    hits += sum(data.count(n) for n in needles)
                    carry=data[-(max_needle-1):] if max_needle>1 else b''
        except OSError:
            raise SystemExit('unreadable repository scan target')
print(hits)
raise SystemExit(1 if hits else 0)
PY
) || { echo 'privacy repository scan failed or found prohibited content' >&2; exit 1; }
  [ "$count" = 0 ]
  echo 'privacy scan: repository matches=0'
}
request() {
  body=$1
  marker=$2
  kind=$3
  printf '%s' "$body" | curl --fail-with-body --silent --show-error --no-buffer \
    -H "Authorization: Bearer $API_KEY" -H 'Content-Type: application/json' \
    --data-binary @- "$API_BASE/v1/chat/completions" | EXPECTED_MARKER="$marker" python3 "$ROOT/ops/privacy-proof/assert-response.py" "$kind"
}
body_a=$(printf '{"model":"%s","messages":[{"role":"user","content":"Repeat exactly: %s"}],"reasoning":false,"max_tokens":128}' "$MODEL_ID" "$CANARY_A")
body_b=$(printf '{"model":"%s","messages":[{"role":"user","content":"Think step by step about the string %s. In the reasoning section first copy the full string exactly, then count its characters. Final answer may be only the count."}],"reasoning":true,"max_tokens":256}' "$MODEL_ID" "$CANARY_B")
body_c=$(printf '{"model":"%s","messages":[{"role":"user","content":"Call record_marker with marker exactly %s. Do not answer in text."}],"reasoning":false,"tools":[{"type":"function","function":{"name":"record_marker","description":"Record a marker without side effects.","parameters":{"type":"object","properties":{"marker":{"type":"string"}},"required":["marker"]}}}],"tool_choice":{"type":"function","function":{"name":"record_marker"}},"max_tokens":128}' "$MODEL_ID" "$CANARY_C")
request "$body_a" "$CANARY_A" normal & p1=$!
request "$body_b" "$CANARY_B" reasoning & p2=$!
request "$body_c" "$CANARY_C" tool & p3=$!
sleep 1
[ "${MINI_PRIVACY_TARGET:-}" != candidate ] || python3 ops/candidate/check.py
scan_pipe during_application_logs $COMPOSE logs --no-color api web controller postgres backup-scheduler
scan_repo
scan_pipe during_dmr_history env MODEL_RUNNER_HOST="$DMR_HOST" "$DMR" requests
scan_pipe during_dmr_logs cat "$ROOT/var/artifacts/compatible-runtime/state/dmr.log"
wait "$p1"; wait "$p2"; wait "$p3"
[ "${MINI_PRIVACY_TARGET:-}" != candidate ] || python3 ops/candidate/check.py
scan_pipe immediately_after_application_logs $COMPOSE logs --no-color api web controller postgres backup-scheduler
scan_pipe immediately_after_dmr_history env MODEL_RUNNER_HOST="$DMR_HOST" "$DMR" requests
scan_pipe immediately_after_dmr_logs cat "$ROOT/var/artifacts/compatible-runtime/state/dmr.log"
scan_repo
printf '%s\n' 'Restart the project-compatible DMR through the approved local operator control, then press Enter. Do not change versions or configuration.' >&2
IFS= read -r _
[ "${MINI_PRIVACY_TARGET:-}" != candidate ] || python3 ops/candidate/check.py
scan_pipe post_restart_application_logs $COMPOSE logs --no-color api web controller postgres backup-scheduler
scan_pipe post_restart_dmr_history env MODEL_RUNNER_HOST="$DMR_HOST" "$DMR" requests
scan_pipe post_restart_dmr_logs cat "$ROOT/var/artifacts/compatible-runtime/state/dmr.log"
scan_repo
unset API_KEY MODEL_ID CANARY_A CANARY_B CANARY_C CANARY_HEXES body_a body_b body_c
echo 'privacy proof completed at all three timepoints; emitted evidence contains counts only'
