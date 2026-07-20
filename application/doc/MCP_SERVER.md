# MCP Server

BreachLine can host an [MCP](https://modelcontextprotocol.io) server so an AI client can drive the running window: open files and directories, run queries, read results, and manage workspaces and annotations. Actions taken by the AI happen in the live UI, so a person can watch and keep working in the same session by hand.

The server is off by default. It is a free feature, but the workspace and annotation tools it exposes still require a license (the same rule as the rest of the app).

## Enabling it

Open **Settings -> MCP** and tick **Enable MCP server**, then press Save. The listen address defaults to `127.0.0.1:8765`; keep it on a loopback address so the server is never reachable off this machine. Saving generates a bearer token, shown in the same panel, that clients must present.

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

`AppBridge` (defined in `app/mcpserver/bridge.go`, implemented by `app/mcp_bridge.go`) exposes only read operations: list tabs, get schema, get a page of query results, get a histogram, in-file search, validate a timestamp column, list workspace files, and get annotations (per-row, whole-file, or a presence check). `mcpserver` never imports the `app` package, so there is no import cycle; `main.go` wires the adapter in. One read, `get_console_log`, is the exception: the log buffer lives in the window, so it is dispatched like an action rather than answered on the backend.

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
- `search_in_file` `{tabId, term, isRegex?, query?, page?}` - free-text or regex search over a tab's current query results, returning matching cells with context snippets. Use this for substring/regex matching; use `get_rows`/`apply_query` for the structured query language.
- `validate_timestamp_column` `{tabId, columnName}` - check whether a column parses as timestamps, without changing anything. Call before `set_timestamp_column`.
- `list_workspace_files` - files tracked by the open workspace, each with `fileHash` and `options` (license required). Pass those two back to `remove_file_from_workspace` / `update_file_description` to target an entry.
- `get_annotations` `{tabId, rowIndices}` - annotations on given rows (license required).
- `get_file_annotations` `{tabId}` - every annotation on a tab's file, each with its display index under the current query (`-1` if not visible). License required.
- `has_annotations` `{tabId}` - whether a tab's file has any annotations at all (a cheap presence check). License required.
- `get_console_log` `{limit?, level?}` - recent entries from the app console (backend and UI events), for diagnosing why an earlier action failed. `level` is the minimum of `info|warn|error`. Answered by the window (it holds the log buffer), so it counts as an action path internally.

Action tools (performed by the visible window):

- `open_file` `{path, jpath?, noHeaderRow?, ingestTimezone?}` - open a single file in a new tab.
- `open_directory` `{path, filePattern, jpath?, includeSourceColumn?}` - open a directory of files as one merged, time-sorted tab. `filePattern` is a recursive glob such as `**/*.json.gz`. If the result has `truncated: true`, more matching files existed than the configured limit allowed, so only `filesLoaded` of them were opened and the dataset is incomplete; the caller should warn the user to raise the directory file limit in Settings (0 = unlimited) or narrow `filePattern`.
- `apply_query` `{tabId, query}` - apply a query to a tab (updates the search bar, grid, and histogram).
- `set_active_tab` `{tabId}`, `close_tab` `{tabId}` - tab navigation.
- `set_timestamp_column` `{tabId, columnName}` - change which column a tab uses as its timestamp, re-sorting and refreshing the view. Switches the window to that tab. Validate with `validate_timestamp_column` first.
- `open_workspace` `{path}`, `close_workspace`, `add_file_to_workspace` `{path, ...}` - open/close a workspace and add a file (license required).
- `create_local_workspace` `{path}` - create a new local `.breachline` workspace at `path` and open it (license required).
- `create_remote_workspace` `{name}` - create a synced remote workspace and open it (license required; the user must be signed in to sync).
- `remove_file_from_workspace` `{fileHash, options}` - remove a file from the open workspace, identified by the `fileHash` + `options` from `list_workspace_files` (license required).
- `update_file_description` `{fileHash, options, description}` - set a workspace file's description, identified the same way (license required).
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
