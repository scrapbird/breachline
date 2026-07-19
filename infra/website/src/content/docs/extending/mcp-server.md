---
title: "MCP Server"
date: 2025-01-01T00:00:00Z
draft: false
weight: 3
---

BreachLine can host an [MCP](https://modelcontextprotocol.io) server so an AI client can drive the running app: open files and directories, run queries, read results, and manage workspaces and annotations. The actions the AI takes happen in the visible window, through the same handlers you use by hand, so you can watch every step and keep working in the same session.

This turns "hand a log file to a model and trust the summary" into something you can see. When the AI filters the log events, the result appears in your grid. You are not reading a claim about the data, you are watching the query run against it.

The server is off by default. It is a free feature, but the workspace and annotation tools it exposes still require a license, the same rule as the rest of the app.

## Enabling it

Open **File → Settings** and go to the **MCP** tab, tick **Enable MCP server**, then press Save.

The listen address defaults to `127.0.0.1:8765`. Keep it on a loopback address (`127.0.0.1`) so the server is never reachable from other machines. Saving generates a bearer token, shown in the same panel, that clients must present on every request.

## Connecting a client

The settings panel shows a ready to paste command for connecting a client. For example, to add BreachLine to Claude:

```
claude mcp add --transport http breachline http://127.0.0.1:8765/ --header "Authorization: Bearer <token>"
```

Copy the token from the panel into the command. Once connected, the client can drive this window.

## What the AI can do

The server exposes two kinds of tools.

Reads answer questions about what is loaded without changing the window:

- List the open tabs and their ids.
- Get a tab's columns, detected timestamp column, and row count.
- Run a query against a tab and return a page of matching rows.
- Get a time bucketed histogram of events for a tab.
- List the files tracked by the open workspace (license required).
- Read annotations on given rows (license required).
- Fetch a cheatsheet for the query language.

Actions are performed by the visible window, so they update the search bar, grid, and histogram as you watch:

- Open a single file, or a directory of files merged into one time sorted tab.
- Apply a query to a tab.
- Switch tabs and close tabs.
- Open, close, and add files to a workspace (license required).
- Annotate rows and delete annotations (license required).
- Export the annotated timeline (license required).

Because the AI writes the same query language you do, it is worth pointing it at the [query language reference](/docs/searching/query-language/) or letting it call the built in cheatsheet first.

## Watching the work

Because the server drives the real UI rather than a separate copy of the data, the AI's view and yours never diverge. There is one set of tabs, one active query, one grid. That has a few practical effects during an investigation:

- Every step is visible while it happens. If the AI goes down a wrong path, you see it in the grid and can correct it on the spot.
- When it lands on something, the grid is already filtered to it. You take over by hand, annotate the rows, and keep pivoting without any export and reimport.
- Verifying a finding is scrolling back up, not redoing the work, because the query already ran on your screen.

## Security

Enabling the server grants a local AI client the ability to read the files you open and act in the app. Treat it as a deliberate choice.

- It binds to a loopback address by default, so it is not reachable from other machines.
- Every request must carry the bearer token, checked in constant time. Requests without it are rejected.
- It is off until you explicitly enable it.

Only enable the server when you intend to use it. For more on how BreachLine handles your data, see [Security & Data Handling](/docs/reference/security-data-handling/).
