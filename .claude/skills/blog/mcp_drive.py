#!/usr/bin/env python3
"""Drive a running BreachLine app over its MCP server (Streamable HTTP).

Used by the `blog` skill to open data and apply queries in the live window so
screenshots capture the real app. Stdlib only, no install required.

Config via env:
  BREACHLINE_MCP_URL    default http://127.0.0.1:8765/
  BREACHLINE_MCP_TOKEN  bearer token (must match breachline.yml mcp_server_token)

Examples:
  python3 mcp_drive.py help
  python3 mcp_drive.py list-tabs
  python3 mcp_drive.py open-dir /path/to/logs --jpath '$.Records' --pattern '**/*.json.gz' --source
  python3 mcp_drive.py open-file /path/to/file.json --jpath '$.Records'
  python3 mcp_drive.py query tab-1 "filter errorCode=* | columns eventTime, eventName, errorCode"
  python3 mcp_drive.py rows tab-1 --query "filter errorCode=*" --limit 5
"""
import argparse
import json
import logging
import os
import urllib.request

logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
log = logging.getLogger("mcp_drive")

URL = os.environ.get("BREACHLINE_MCP_URL", "http://127.0.0.1:8765/")
TOKEN = os.environ.get("BREACHLINE_MCP_TOKEN", "")

_state = {"session": None, "id": 0}


def _rpc(method, params=None, notify=False):
    """Send one JSON-RPC message and return the parsed result (or None for notifications)."""
    _state["id"] += 1
    body = {"jsonrpc": "2.0", "method": method}
    if not notify:
        body["id"] = _state["id"]
    if params is not None:
        body["params"] = params

    headers = {
        "Content-Type": "application/json",
        "Accept": "application/json, text/event-stream",
    }
    if TOKEN:
        headers["Authorization"] = "Bearer " + TOKEN
    if _state["session"]:
        headers["Mcp-Session-Id"] = _state["session"]

    req = urllib.request.Request(URL, data=json.dumps(body).encode(), headers=headers, method="POST")
    with urllib.request.urlopen(req, timeout=60) as resp:
        sid = resp.headers.get("Mcp-Session-Id")
        if sid:
            _state["session"] = sid
        raw = resp.read().decode()

    if notify:
        return None
    # Response is JSON, or SSE framed as "data: {...}".
    for line in raw.splitlines():
        line = line.strip()
        if line.startswith("data:"):
            return json.loads(line[5:].strip())
    return json.loads(raw) if raw.strip() else None


def _handshake():
    _rpc("initialize", {
        "protocolVersion": "2025-06-18",
        "capabilities": {},
        "clientInfo": {"name": "blog-skill", "version": "1"},
    })
    _rpc("notifications/initialized", notify=True)


def call_tool(name, arguments):
    """Call an MCP tool and return its structured result (raises on tool error)."""
    res = _rpc("tools/call", {"name": name, "arguments": arguments})
    if res and "error" in res:
        raise RuntimeError(res["error"].get("message", str(res["error"])))
    result = (res or {}).get("result", {})
    if result.get("isError"):
        text = ""
        for c in result.get("content", []):
            text += c.get("text", "")
        raise RuntimeError(text or "tool reported an error")
    if result.get("structuredContent") is not None:
        return result["structuredContent"]
    return result


def main():
    ap = argparse.ArgumentParser(description="Drive BreachLine over MCP")
    sub = ap.add_subparsers(dest="cmd", required=True)

    sub.add_parser("help", help="print the query-language cheatsheet")
    sub.add_parser("list-tabs", help="list open tabs")

    d = sub.add_parser("open-dir", help="open a directory of files as one tab")
    d.add_argument("path")
    d.add_argument("--jpath", default="")
    d.add_argument("--pattern", required=True, help="glob, e.g. **/*.json.gz")
    d.add_argument("--source", action="store_true", help="add __source_file__ column")

    f = sub.add_parser("open-file", help="open a single file")
    f.add_argument("path")
    f.add_argument("--jpath", default="")

    q = sub.add_parser("query", help="apply a query to a tab (visible in the window)")
    q.add_argument("tab_id")
    q.add_argument("query")

    r = sub.add_parser("rows", help="fetch rows as data (no UI change)")
    r.add_argument("tab_id")
    r.add_argument("--query", default="")
    r.add_argument("--limit", type=int, default=10)

    args = ap.parse_args()
    _handshake()

    if args.cmd == "help":
        out = call_tool("get_query_help", {})
    elif args.cmd == "list-tabs":
        out = call_tool("list_tabs", {})
    elif args.cmd == "open-dir":
        out = call_tool("open_directory", {
            "path": args.path, "jpath": args.jpath,
            "filePattern": args.pattern, "includeSourceColumn": args.source,
        })
    elif args.cmd == "open-file":
        out = call_tool("open_file", {"path": args.path, "jpath": args.jpath})
    elif args.cmd == "query":
        out = call_tool("apply_query", {"tabId": args.tab_id, "query": args.query})
    elif args.cmd == "rows":
        out = call_tool("get_rows", {"tabId": args.tab_id, "query": args.query, "limit": args.limit})
    else:
        ap.error("unknown command")
        return

    print(json.dumps(out, indent=2))


if __name__ == "__main__":
    main()
