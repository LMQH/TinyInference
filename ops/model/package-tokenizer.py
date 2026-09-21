#!/usr/bin/env python3
import hashlib,json,os,pathlib,struct,sys,tempfile
ROOT=pathlib.Path(__file__).resolve().parents[2]
SOURCE=ROOT/'models/openbmb/MiniCPM5-2B-GGUF/MiniCPM5-2B-Q4_K_M.gguf'
OUT=ROOT/'var/artifacts/tokenizer/tokenizer.gguf'
MANIFEST=ROOT/'config/compatibility-manifest.json'
EXPECTED_SIZE=1561318368
EXPECTED_SHA='ec2d5801640099e97d8d7e8003ad4d81f336e757811f03a26173dddf386602fd'
FIXED={0:1,1:1,2:2,3:2,4:4,5:4,6:4,7:1,10:8,11:8,12:8}

def digest(path):
    h=hashlib.sha256()
    with path.open('rb') as f:
        for chunk in iter(lambda:f.read(1024*1024),b''): h.update(chunk)
    return h.hexdigest()

def read_exact(f,n):
    data=f.read(n)
    if len(data)!=n: raise SystemExit('truncated GGUF metadata')
    return data

def u32(f): return struct.unpack('<I',read_exact(f,4))[0]
def u64(f): return struct.unpack('<Q',read_exact(f,8))[0]
def skip_string(f):
    n=u64(f)
    if n>64*1024*1024: raise SystemExit('oversize GGUF metadata string')
    read_exact(f,n)
def skip_value(f,kind,depth=0):
    if depth>8: raise SystemExit('GGUF metadata nesting exceeds bound')
    if kind in FIXED: read_exact(f,FIXED[kind]); return
    if kind==8: skip_string(f); return
    if kind==9:
        element=u32(f); count=u64(f)
        if count>100_000_000: raise SystemExit('GGUF metadata array exceeds bound')
        for _ in range(count): skip_value(f,element,depth+1)
        return
    raise SystemExit('unsupported GGUF metadata type')

before_stat=SOURCE.stat()
if not SOURCE.is_file() or SOURCE.is_symlink() or before_stat.st_size!=EXPECTED_SIZE or digest(SOURCE)!=EXPECTED_SHA:
    raise SystemExit('immutable GGUF source identity mismatch')
with SOURCE.open('rb') as f:
    if read_exact(f,4)!=b'GGUF': raise SystemExit('invalid GGUF magic')
    version=u32(f)
    if version not in (2,3): raise SystemExit('unsupported GGUF version')
    u64(f) # tensor count; tensor payload is intentionally excluded
    metadata_count=u64(f)
    if metadata_count>1_000_000: raise SystemExit('GGUF metadata entry count exceeds bound')
    for _ in range(metadata_count):
        skip_string(f); skip_value(f,u32(f))
    prefix_size=f.tell()
    f.seek(0); prefix=read_exact(f,prefix_size)
after_stat=SOURCE.stat()
if after_stat.st_size!=before_stat.st_size or after_stat.st_mode!=before_stat.st_mode or digest(SOURCE)!=EXPECTED_SHA:
    raise SystemExit('immutable GGUF source changed during tokenizer extraction')
OUT.parent.mkdir(parents=True,exist_ok=True)
fd,tmp=tempfile.mkstemp(prefix='tokenizer.gguf.',dir=OUT.parent)
try:
    with os.fdopen(fd,'wb') as f: f.write(prefix); f.flush(); os.fsync(f.fileno())
    os.chmod(tmp,0o444); os.replace(tmp,OUT)
finally:
    if os.path.exists(tmp): os.unlink(tmp)
artifact_sha=hashlib.sha256(prefix).hexdigest()
m=json.loads(MANIFEST.read_text(encoding='utf-8'))
m['model']['tokenizer_artifact']={'path':'var/artifacts/tokenizer/tokenizer.gguf','size_bytes':prefix_size,'sha256':artifact_sha,'source_sha256':EXPECTED_SHA}
m['unresolved_fields']=[x for x in m['unresolved_fields'] if x!='model.tokenizer_artifact']
fd,tmp=tempfile.mkstemp(prefix='compatibility-manifest.',dir=MANIFEST.parent,text=True)
with os.fdopen(fd,'w',encoding='utf-8') as f: json.dump(m,f,indent=2); f.write('\n'); f.flush(); os.fsync(f.fileno())
os.replace(tmp,MANIFEST)
print('tokenizer metadata artifact recorded; immutable source identity preserved')
