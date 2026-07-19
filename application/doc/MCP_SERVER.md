# MCP Server

BreachLine can host an [MCP](https://modelcontextprotocol.io) server so an AI client can drive the running window: open files and directories, run queries, read results, and manage workspaces and annotations. Actions taken by the AI happen in the live UI, so a person can watch and keep working in the same session by hand.

The server is off by default. It is a free feature, but the workspace and annotation tools it exposes still require a license (the same rule as the rest of the app).

## Enabling it

Open **Settings -> AI (MCP)** and tick **Enable MCP server**, then press Save. The listen address defaults to `127.0.0.1:8765`; keep it on a loopback address so the server is never reachable off this machine. Saving generates a bearer token, shown in the same panel, that clients must present.

The panel also shows a ready-to-paste client command, for example:

```
claude mcp add --transport http breachline http://127.0.0.1:8765/ --header "Authorization: Bearer <token>"
```

## Transport

The server speaks MCP over the Streamable HTTP transport, built on the official Go SDK (`github.com/modelcontextprotocol/go-sdk/mcp`) and the standard library `net/http`. A single endpoint at `/` handles the JSON-RPC 2.0 `initialize` handshake, `tools/list`, and `tools/call`. Tool input and output schemas are generated automatically from typed Go structs, so a client sees a fully typed tool surface.

## Architecture

The whole feature is one self-contained Go package, `app/mcpserver`, plus a thin adapter and a small frontend bridge. The key design choice is that the server never owns UI state itself. It answers pure reads on the backend, and for anything that changes what the user sees, it asks the frontend to perform the action using the exact handlers a human uses. That keeps the AI's view and the user's view identical: there is one set of tabs, one active query, one grid.

### Two paths

**Reads** (no UI change) are answered directly on the backend through the `AppBridge` interface:

```
AI client --tools/call get_rows--> mcpserver --AppBridge.GetRows--> App --> rows back to AI
```

`AppBridge` (defined in `app/mcpserver/bridge.go`, implemented by `app/mcp_bridge.go`) exposes only read operations: list tabs, get schema, get a page of query results, get a histogram, list workspace files, get annotations. `mcpserver` never imports the `app` package, so there is no import cycle; `main.go` wires the adapter in.

**Actions** (change the window) are dispatched to the frontend and awaited:

```
AI client --tools/call apply_query-->
  mcpserver: register a pending request, emit Wails event "mcp:command" {id, action, params}, block on a channel
    frontend useMcpBridge: run the same handler a human uses (updates search bar, grid, histogram)
    frontend: call bound MCPResolve(id, resultJSON, errMsg)
  mcpserver: the channel unblocks, the tool returns the result to the AI
```

Because the frontend performs the open/query/annotate through its normal handlers, the backend tab it creates and the on-screen state never diverge, and there is no separate "hydrate the UI from a backend tab" path to keep in sync. A dispatched action times out after 30 seconds if the window never replies.

### Lifecycle

`main.go` constructs the server with the `AppBridge` adapter, binds a small `BridgeService` to the frontend, and registers the server as the settings `MCPController`. On startup it reads the effective settings and calls `ApplyMCPConfig(enabled, address, token)`. When the user saves the MCP settings, `SettingsService` calls `ApplyMCPConfig` again; the server stops and restarts to match, generating a token the first time it is enabled. This mirrors how `SettingsService` already manages the query cache through `SetCacheManager`.

### File map

| File | Role |
|---|---|
| `app/mcpserver/bridge.go` | `AppBridge` interface + the plain result structs (tab summary, schema, rows, histogram, etc.) |
| `app/mcpserver/server.go` | lifecycle (start/stop/restart), HTTP + auth, the request/response command bridge, and the frontend-bound `BridgeService` (`MCPResolve`, `MCPGetStatus`) |
| `app/mcpserver/tools.go` | tool definitions and handlers; reads call `AppBridge`, actions call `runCommand` (the frontend dispatch) |
| `app/mcp_bridge.go` | `AppBridge` implementation over `*app.App` (backend reads only) |
| `app/settings/*.go` | `MCPServerEnabled` / `MCPServerAddress` / `MCPServerToken` fields and the `MCPController` restart hook |
| `main.go` | constructs, wires, binds, and starts the server |
| `frontend/src/hooks/useMcpBridge.ts` | listens for `mcp:command`, performs the action with the app's own handlers, replies via `MCPResolve` |
| `frontend/src/components/McpSettingsSection.tsx` | the Settings panel (enable, address, status, token, client config) |

## Tools

Read-only tools (answered on the backend):

- `get_query_help` - a cheatsheet for the query language (call this before writing a query).
- `list_tabs` - the open tabs and their ids.
- `get_schema` `{tabId}` - a tab's columns, detected timestamp column, and row count.
- `get_rows` `{tabId, query?, offset?, limit?}` - run a query against a tab and return a page of matching rows as structured data.
- `get_histogram` `{tabId, query?, bucketSeconds?}` - time-bucketed event counts for a tab.
- `list_workspace_files` - files tracked by the open workspace (license required).
- `get_annotations` `{tabId, rowIndices}` - annotations on given rows (license required).

Action tools (performed by the visible window):

- `open_file` `{path, jpath?, noHeaderRow?, ingestTimezone?}` - open a single file in a new tab.
- `open_directory` `{path, filePattern, jpath?, includeSourceColumn?}` - open a directory of files as one merged, time-sorted tab. `filePattern` is a recursive glob such as `**/*.json.gz`.
- `apply_query` `{tabId, query}` - apply a query to a tab (updates the search bar, grid, and histogram).
- `set_active_tab` `{tabId}`, `close_tab` `{tabId}` - tab navigation.
- `open_workspace` `{path}`, `close_workspace`, `add_file_to_workspace` `{path, ...}` - workspace management (license required).
- `annotate_rows` `{tabId, rowIndices, note, color?}`, `delete_annotations` `{tabId, rowIndices}` - row annotations (license required).
- `export_workspace_timeline` `{outputPath?}` - export the annotated timeline (license required).

### Query language reminder

Queries are pipeline stages joined by `|`, and every stage starts with a keyword. To match rows use the `filter` stage: a bare expression like `errorCode=*` is not valid on its own, but `filter errorCode=*` is. Operators inside `filter`: `=`, `!=`, `~` (contains), `!~`, `field=*` (present), `field=prefix*`. Other stages: `columns`, `after`/`before` (relative like `24h` or `7d`, or a quoted absolute time), `dedup`, `limit`. There are no numeric comparison operators. Call `get_query_help` for the same summary at runtime.

## Security

- Bound to a loopback address by default, so it is not reachable from other machines.
- A bearer token is required on every request (constant-time comparison); requests without it get 401.
- Off by default, and enabling it is an explicit, visible setting.
- Workspace and annotation tools additionally require a valid license.

Treat enabling the server as granting a local AI client the ability to read files you open and act in the app. Only enable it when you intend to use it.
