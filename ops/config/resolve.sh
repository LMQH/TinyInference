#!/bin/sh
set -eu
umask 077
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd -P)
cd "$ROOT"
CONF=config/runtime.mac.conf
MANIFEST=config/compatibility-manifest.json
for key in LAN_BIND_ADDRESS API_IMAGE WEB_IMAGE CONTROLLER_IMAGE JOBS_IMAGE POSTGRES_IMAGE MODEL_OCI_REFERENCE MODEL_OCI_DIGEST API_CPUS API_MEMORY WEB_CPUS WEB_MEMORY CONTROLLER_CPUS CONTROLLER_MEMORY POSTGRES_CPUS POSTGRES_MEMORY JOBS_CPUS JOBS_MEMORY; do
  value=$(sed -n "s/^$key=//p" "$CONF")
  [ -n "$value" ] || { echo "unresolved configuration: $key" >&2; exit 1; }
done
manifest_hash=$(shasum -a 256 "$MANIFEST" | cut -d ' ' -f 1)
python3 ops/config/update.py "$CONF" COMPATIBILITY_MANIFEST_SHA256 "$manifest_hash" COMPOSE_CONFIG_SHA256 0000000000000000000000000000000000000000000000000000000000000000
mkdir -p var/artifacts
raw=var/artifacts/compose.hash-input.yaml
final=var/artifacts/compose.resolved.yaml
docker compose --project-name mini-inference --env-file "$CONF" config > "$raw"
# Digest carrier fields are excluded to break the manifest/config mutual hash cycle.
sed '/RESOLVED_COMPOSE_CONFIG_SHA256:/d;/COMPOSE_CONFIG_SHA256:/d;/COMPATIBILITY_MANIFEST_SHA256:/d' "$raw" > "$raw.canonical"
compose_hash=$(shasum -a 256 "$raw.canonical" | cut -d ' ' -f 1)
python3 ops/config/update.py "$CONF" COMPOSE_CONFIG_SHA256 "$compose_hash"
python3 - "$MANIFEST" "$compose_hash" <<'PY'
import json,os,sys,tempfile
p=sys.argv[1]; value=sys.argv[2]
m=json.load(open(p,encoding='utf-8')); m['resolved_compose_config_sha256']=value
m['unresolved_fields']=[x for x in m['unresolved_fields'] if x!='resolved_compose_config_sha256']
fd,t=tempfile.mkstemp(prefix='manifest.',dir=os.path.dirname(p),text=True)
with os.fdopen(fd,'w',encoding='utf-8') as f: json.dump(m,f,indent=2); f.write('\n'); f.flush(); os.fsync(f.fileno())
os.replace(t,p)
PY
manifest_hash=$(shasum -a 256 "$MANIFEST" | cut -d ' ' -f 1)
python3 ops/config/update.py "$CONF" COMPATIBILITY_MANIFEST_SHA256 "$manifest_hash"
docker compose --project-name mini-inference --env-file "$CONF" config > "$final"
rm -f "$raw" "$raw.canonical"
echo 'resolved Compose configuration and manifest hashes recorded'
