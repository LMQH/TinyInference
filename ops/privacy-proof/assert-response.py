#!/usr/bin/env python3
"""Confirm a synthetic canary reached its intended output without printing content."""

import hashlib
import json
import os
import sys

kind = sys.argv[1]
marker = os.environ["EXPECTED_MARKER"]
response = json.load(sys.stdin)
choice = response["choices"][0]
message = choice["message"]
if kind == "normal":
    output = message.get("content") or ""
elif kind == "reasoning":
    output = message.get("reasoning_content") or ""
elif kind == "tool":
    if choice.get("finish_reason") != "tool_calls":
        raise SystemExit("privacy tool call was not generated")
    output = "".join(call["function"]["arguments"] for call in message.get("tool_calls") or [])
else:
    raise SystemExit("invalid privacy canary kind")
if marker not in output:
    raise SystemExit(f"privacy {kind} output canary was not observed")
print(f"privacy {kind} output canary observed; sha256={hashlib.sha256(marker.encode()).hexdigest()}")
