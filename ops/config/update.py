#!/usr/bin/env python3
import pathlib, re, sys
if len(sys.argv) < 4 or len(sys.argv[2:]) % 2:
    raise SystemExit("usage: update.py FILE KEY VALUE [KEY VALUE ...]")
path=pathlib.Path(sys.argv[1])
updates=dict(zip(sys.argv[2::2],sys.argv[3::2]))
lines=path.read_text(encoding='utf-8').splitlines()
seen=set(); out=[]
for line in lines:
    if '=' in line and not line.lstrip().startswith('#'):
        key=line.split('=',1)[0]
        if key in updates:
            value=updates[key]
            if '\n' in value or '\r' in value or not re.fullmatch(r'[A-Za-z0-9_./:@+-]*', value):
                raise SystemExit(f'unsafe config value for {key}')
            line=f'{key}={value}'; seen.add(key)
    out.append(line)
if seen != set(updates):
    raise SystemExit('refusing to append unknown configuration keys')
tmp=path.with_suffix(path.suffix+'.partial')
tmp.write_text('\n'.join(out)+'\n',encoding='utf-8')
tmp.replace(path)
