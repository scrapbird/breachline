---
title: "Tabs & Multiple Files"
date: 2025-01-01T00:00:00Z
draft: false
weight: 4
---

BreachLine opens each file in its own tab, so you can pivot between several data sources during a single investigation without losing your place.

## How tabs work

- Every file you open gets a tab along the top of the window.
- Each tab keeps its own state: its query, its timestamp column, its sort order, its scroll position, and its caches.
- Switching tabs is instant. BreachLine does not reload or re-sort the data, so you can move between a firewall log and an auth log while keeping both queries intact.

## Opening more files

- **File → Open** or **File → Open Directory** creates a new tab for the file or directory you pick.
- Opening a file from the [workspace dashboard](/docs/workspaces/overview/) opens it in a new tab too.

## Closing tabs

Close a tab with its close button. If you close the active tab, BreachLine switches to another open tab automatically. Closing a tab frees the memory its caches were using.

## A note on memory

Because each open file is held in memory with its own caches, the number of large files you keep open at once is bounded by available RAM. See [System Requirements](/docs/getting-started/system-requirements/).
