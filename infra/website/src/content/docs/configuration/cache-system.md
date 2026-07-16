---
title: "Cache System"
date: 2025-01-01T00:00:00Z
draft: false
weight: 3
---

BreachLine stays fast on large files by caching work it has already done. There are two caches, both kept per tab so open files never interfere with each other.

## Sort cache

The first time a query needs sorted data, BreachLine reads the whole file into memory and sorts it by the timestamp column. That sorted copy is cached and reused, so later queries against the same file do not pay the sort cost again.

The sort cache is rebuilt when any of these change:

- The file itself.
- Whether sorting by time is enabled.
- The sort direction (ascending or descending).
- The timestamp column in use.

## Query cache

On top of the sort cache, BreachLine caches the results of individual queries. Re-running a recent query, or paging back to it, returns instantly from the cache instead of re-filtering. The query cache is a most-recently-used cache, so it keeps your recent queries and lets older ones fall out.

You can turn the query cache off under **File → Settings** if you prefer to minimise memory use. Sorting is unaffected by that toggle.

## When caches are cleared

Caches are invalidated automatically when a change would make them stale, for example changing the sort order, the timestamp column, or a timezone or format setting. Closing a tab frees the memory its caches held.

## Memory trade-off

Caching trades memory for speed. On a machine that is tight on RAM relative to the file size, disabling the query cache reduces memory use at the cost of re-running filters. See [System Requirements](/docs/getting-started/system-requirements/) for sizing guidance.
