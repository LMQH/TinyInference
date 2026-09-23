#!/usr/bin/env python3
import os,pathlib,re,sys,tempfile
if len(sys.argv)!=2 or not re.fullmatch(r'mini-inference-\d{8}T\d{6}Z-7\.dump',sys.argv[1]):
    raise SystemExit('an exact schema-7 backup basename is required')
root=pathlib.Path(__file__).resolve().parents[2]
out=root/'var'/'artifacts'/('candidate/restore-selection' if os.environ.get('MINI_CANDIDATE_RESTORE')=='1' else 'restore-selection')
out.parent.mkdir(parents=True,exist_ok=True)
fd,tmp=tempfile.mkstemp(prefix='restore-selection.',dir=out.parent,text=True)
with os.fdopen(fd,'w') as f: f.write(sys.argv[1]+'\n'); f.flush(); os.fsync(f.fileno())
os.chmod(tmp,0o600); os.replace(tmp,out)
