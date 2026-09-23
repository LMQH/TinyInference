#!/usr/bin/env python3
import os, sys
needles=[bytes.fromhex(x) for x in os.environ.get('CANARY_HEXES','').split(',') if x]
if not needles:
    raise SystemExit(2)
count=0
carry=b''
max_needle=max(map(len,needles))
while True:
    chunk=sys.stdin.buffer.read(1024*1024)
    if not chunk: break
    data=carry+chunk
    for needle in needles:
        count += data.count(needle)
    carry=data[-(max_needle-1):] if max_needle>1 else b''
print(count)
raise SystemExit(1 if count else 0)
