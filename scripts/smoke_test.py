#!/usr/bin/env python3
"""Exercise discovery on the actual stdio binary; keep stdin open for its reply."""
import json
import queue
import subprocess
import sys
import threading

request = {
    "jsonrpc": "2.0", "id": 1, "method": "server/discover",
    "params": {"_meta": {
        "io.modelcontextprotocol/protocolVersion": "2026-07-28",
        "io.modelcontextprotocol/clientInfo": {"name": "smoke", "version": "1"},
        "io.modelcontextprotocol/clientCapabilities": {},
    }},
}
process = subprocess.Popen([sys.argv[1] if len(sys.argv) > 1 else "./bin/pcloud-mcp"],
                           stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE, text=True)
lines = queue.Queue()
threading.Thread(target=lambda: lines.put(process.stdout.readline()), daemon=True).start()
try:
    process.stdin.write(json.dumps(request) + "\n")
    process.stdin.flush()
    try:
        line = lines.get(timeout=5)
    except queue.Empty:
        raise SystemExit("Discovery timed out")
    response = json.loads(line)
    if response.get("error") or not response.get("result"):
        raise SystemExit("Discovery did not succeed")
    print(json.dumps(response, indent=2))
finally:
    process.stdin.close()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait()
        raise SystemExit("Server shutdown timed out")
