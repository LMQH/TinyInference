#!/usr/bin/env python3
import json,os,pathlib,re,sys,tempfile
if len(sys.argv)!=4: raise SystemExit('usage: record-images.py CONF MANIFEST IID_DIR')
conf,manifest_path,iid_dir=sys.argv[1:]
images={}
for name in ('api','web','controller','jobs'):
    value=pathlib.Path(iid_dir,f'{name}.iid').read_text().strip()
    if not re.fullmatch(r'sha256:[0-9a-f]{64}',value): raise SystemExit(f'invalid {name} image ID')
    images[name]=value
config={}
for line in pathlib.Path(conf).read_text().splitlines():
    if '=' in line and not line.lstrip().startswith('#'):
        key,value=line.split('=',1); config[key]=value
lines=pathlib.Path(conf).read_text().splitlines(); mapping={f'{k.upper()}_IMAGE':v for k,v in images.items()}; seen=set(); out=[]
for line in lines:
    key=line.split('=',1)[0] if '=' in line else ''
    if key in mapping: line=f'{key}={mapping[key]}'; seen.add(key)
    out.append(line)
if seen!=set(mapping): raise SystemExit('missing image configuration key')
pathlib.Path(conf).write_text('\n'.join(out)+'\n')
m=json.load(open(manifest_path,encoding='utf-8'))
for name,value in images.items():
    m['images'][name]['reference']=value
    m['images'][name]['digest']=value
postgres=config.get('POSTGRES_IMAGE','')
match=re.fullmatch(r'(.+)@(sha256:[0-9a-f]{64})',postgres)
if not match: raise SystemExit('PostgreSQL image is not digest pinned')
m['images']['postgres']['reference']=postgres
m['images']['postgres']['digest']=match.group(2)
bases={
    'go_builder':'GO_BUILDER_IMAGE',
    'distroless_static':'DISTROLESS_STATIC_IMAGE',
    'node_builder':'NODE_BUILDER_IMAGE',
    'nginx_runtime':'NGINX_RUNTIME_IMAGE',
    'postgres_client':'POSTGRES_CLIENT_IMAGE',
}
for field,key in bases.items():
    value=config.get(key,'')
    if not re.search(r'@sha256:[0-9a-f]{64}$',value):
        raise SystemExit(f'build base is not digest pinned: {key}')
    m['build_bases'][field]=value
commit=config.get('MODEL_RUNNER_COMMIT','')
checksum=config.get('DOCKER_MODEL_PLUGIN_SHA256','')
if not re.fullmatch(r'[0-9a-f]{40}',commit) or not re.fullmatch(r'[0-9a-f]{64}',checksum):
    raise SystemExit('invalid controller plugin commit or checksum')
m['controller_plugin']['commit']=commit
m['controller_plugin']['binary_sha256']=checksum
resolved={f'images.{name}' for name in images}
resolved.update(f'build_bases.{name}' for name in bases)
resolved.update({'controller_plugin.commit','controller_plugin.binary_sha256'})
m['unresolved_fields']=[x for x in m['unresolved_fields'] if x not in resolved]
fd,tmp=tempfile.mkstemp(prefix='manifest.',dir=os.path.dirname(manifest_path),text=True)
with os.fdopen(fd,'w',encoding='utf-8') as f: json.dump(m,f,indent=2); f.write('\n'); f.flush(); os.fsync(f.fileno())
os.replace(tmp,manifest_path)
