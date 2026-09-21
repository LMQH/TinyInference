#!/usr/bin/env python3
import os, sys
needles=[bytes.fromhex(x) for x in os.environ.get('CANARY_HEXES','').split(',') if x]
if not needles:
    raise SystemExit(2)
count=0
while True:
    chunk=sys.stdin.buffer.read(1024*1024)
    if not chunk: break
    for needle in needles:
        count += chunk.count(needle)
print(count)
raise SystemExit(1 if count else 0)
