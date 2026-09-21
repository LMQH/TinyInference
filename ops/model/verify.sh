#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd -P)
MODEL="$ROOT/models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf"
EXPECTED_SIZE=1561318368
EXPECTED_SHA=ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd
CONFIG="$ROOT/config/runtime.mac.conf"
REF=$(sed -n 's/^MODEL_OCI_REFERENCE=//p' "$CONFIG")
EXPECTED_OCI_DIGEST=$(sed -n 's/^MODEL_OCI_DIGEST=//p' "$CONFIG")
[ "$REF" = local/minicpm5-2b:q4_k_m-ec2d58016400 ] || { echo 'unexpected model reference' >&2; exit 1; }
[ -f "$MODEL" ] && [ ! -L "$MODEL" ] || exit 1
[ "$(wc -c < "$MODEL" | tr -d ' ')" = "$EXPECTED_SIZE" ] || exit 1
[ "$(shasum -a 256 "$MODEL" | cut -d ' ' -f 1)" = "$EXPECTED_SHA" ] || exit 1
python3 - "$ROOT/config/compatibility-manifest.json" "$ROOT/var/artifacts/tokenizer/tokenizer.gguf" "$EXPECTED_OCI_DIGEST" "$REF" <<'PY'
import hashlib, json, pathlib, re, sys
m=json.load(open(sys.argv[1], encoding='utf-8'))
d=m['model']['oci_digest']
configured_digest=sys.argv[3]
expected_ref=sys.argv[4]
if not isinstance(d,str) or not re.fullmatch(r'sha256:[0-9a-f]{64}',d):
    raise SystemExit('model OCI digest remains unresolved')
if configured_digest != d:
    raise SystemExit('model OCI digest differs between runtime config and manifest')
if m['model']['oci_reference'] != expected_ref:
    raise SystemExit('manifest model reference mismatch')
if m['model']['source_sha256']!='ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd':
    raise SystemExit('manifest source checksum mismatch')
artifact=m['model'].get('tokenizer_artifact')
path=pathlib.Path(sys.argv[2])
if not isinstance(artifact,dict) or path.is_symlink() or not path.is_file():
    raise SystemExit('tokenizer metadata artifact remains unresolved')
h=hashlib.sha256()
with path.open('rb') as f:
    for chunk in iter(lambda:f.read(1024*1024),b''): h.update(chunk)
if artifact != {'path':'var/artifacts/tokenizer/tokenizer.gguf','size_bytes':path.stat().st_size,'sha256':h.hexdigest(),'source_sha256':m['model']['source_sha256']}:
    raise SystemExit('tokenizer metadata artifact identity mismatch')
PY
DMR="$ROOT/var/artifacts/compatible-runtime/bin/dmr"
STORE_INDEX="$ROOT/var/artifacts/compatible-runtime/models/models.json"
[ -x "$DMR" ] && [ -f "$STORE_INDEX" ] && [ ! -L "$STORE_INDEX" ] || {
  echo 'compatible DMR is unavailable; run make runtime-build and make runtime-start' >&2
  exit 1
}
actual_digest=$(jq -er --arg tag "docker.io/$REF" \
  '.models | select(length == 1) | .[0] | select(.tags == [$tag]) | .id | select(test("^sha256:[0-9a-f]{64}$"))' \
  "$STORE_INDEX") || {
  echo 'approved model is unavailable from the compatible DMR store or has invalid identity' >&2
  exit 1
}
[ "$actual_digest" = "$EXPECTED_OCI_DIGEST" ] || {
  echo 'local model ID does not match approved OCI digest' >&2
  exit 1
}
unset actual_digest EXPECTED_OCI_DIGEST
echo 'model source, local OCI identity, manifest, and tokenizer metadata artifact verified'
